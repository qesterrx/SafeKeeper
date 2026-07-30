package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/qesterrx/SafeKeeper/cmd/server/internal/config"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/interceptor"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/jwta"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/service"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/storage"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/transport"
	"github.com/qesterrx/SafeKeeper/internal/logger"
	pbauth "github.com/qesterrx/SafeKeeper/proto/auth"
	pbdata "github.com/qesterrx/SafeKeeper/proto/data"
	"google.golang.org/grpc"
)

func main() {

	//Загружаем конфигурацию
	config, err := config.ParseParams()
	if err != nil {
		log.Fatal(err)
	}

	//Инициализация логгера
	err = logger.InitLogger(nil, config.LogLevel)
	if err != nil {
		log.Fatal(err)
	}

	//Подключение postgresql
	storage, err := storage.NewStoragePGSQL(config.DatabaseDSN)
	if err != nil {
		log.Fatal(err.Error())
	}

	//Создаем сервис AUTH
	authService, err := service.NewAuthService(storage)
	if err != nil {
		log.Fatal(err.Error())
	}

	//Создаем сервис DATA
	dataService, err := service.NewDataService(storage)
	if err != nil {
		log.Fatal(err.Error())
	}

	//JWT
	jwta.InitJWTConfig(config.JWTSecret, 1*time.Hour, "SafeKeeperServer")
	jwtInterceptor := interceptor.NewJWTServerInterceptor(authService)

	// Нужно определить порт для сервера
	listen, err := net.Listen("tcp", config.ServerHost.String())
	if err != nil {
		logger.Log.Error("ошибка при инициализации listener %v", err)
		log.Fatal(err.Error())
	}

	// Создаем gRPC сервер
	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptor.UnaryLogInterceptor,
			jwtInterceptor.Unary(),
		),
		grpc.ChainStreamInterceptor(
			interceptor.StreamLogInterceptor,
			jwtInterceptor.Stream(),
		),
	)

	// Регистрируем сервис
	pbauth.RegisterAuthServiceServer(s, transport.NewAuthServer(authService))
	pbdata.RegisterSafeDataServiceServer(s, transport.NewSafeDataServer(dataService))

	logger.Log.Info("Cервер gRPC начал работу по адресу %s", config.ServerHost.String())

	// Канал для отслеживания экстренной остановки сервера
	doneSrv := make(chan struct{})

	// Запускаем сервер в горутине
	go func() {
		logger.Log.Debug("main Serve START")
		defer logger.Log.Debug("main Serve STOP")
		if err := s.Serve(listen); err != nil {
			logger.Log.Error("Ошибка при работе сервера %v", err)
			log.Fatal(err.Error())
		}

		close(doneSrv)
	}()

	// Канал для сигналов завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// Ожидаем сигнал завершения
	select {
	case <-quit:
		logger.Log.Info("Получен сигнал завершения, останавливаем сервер...")
	case <-doneSrv:
		logger.Log.Error("Ошибка при работе сервера, остановка...")
	}

	// Создаем контекст с таймаутом для graceful shutdown GRPC Сервера
	ctxSrvGS, cancelSrvGS := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelSrvGS()

	// Канал для отслеживания завершения
	doneSrvGS := make(chan struct{})

	// Останавливаем сервер в горутине
	go func() {
		s.GracefulStop()
		close(doneSrvGS)
	}()

	// Ожидаем завершения или таймаута
	select {
	case <-doneSrvGS:
		logger.Log.Info("Сервер GRPC успешно остановлен")
	case <-ctxSrvGS.Done():
		logger.Log.Error("Таймаут graceful shutdown, принудительное завершение GRPC сервера")
		s.Stop()
	}

	// Создаем контекст с таймаутом для graceful shutdown Базы данных
	ctxDbGS, cancelDbGS := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDbGS()

	// Канал для отслеживания завершения
	doneDbGS := make(chan struct{})

	// Закрываем соединение с БД
	go func() {
		if err := storage.Close(); err != nil {
			logger.Log.Error("ошибка при закрытии соединения с БД: %v", err)
		}
		close(doneDbGS)

	}()

	// Ожидаем завершения или таймаута
	select {
	case <-doneDbGS:
		logger.Log.Info("Соединение с БД закрыто")
	case <-ctxDbGS.Done():
		logger.Log.Error("Таймаут graceful shutdown, принудительная остановка")
	}

	logger.Log.Info("Приложение завершено")
}

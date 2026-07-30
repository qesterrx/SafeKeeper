package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/qesterrx/SafeKeeper/cmd/client/internal/config"
	"github.com/qesterrx/SafeKeeper/cmd/client/internal/tui"
	"github.com/qesterrx/SafeKeeper/internal/logger"
)

func main() {

	//Загружаем конфигурацию
	config, err := config.ParseParams()
	if err != nil {
		log.Fatal(err)
	}

	//Инициализация логгера
	if config.LogFile != "" {
		file, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		err = logger.InitLogger(file, config.LogLevel)
		if err != nil {
			log.Fatal(err)
		}
	}

	//Основной контекст
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//Основное приложение
	app, err := tui.NewTUIApp(ctx, config)
	if err != nil {
		log.Fatal(err.Error())
	}

	//Запуск
	go func() {
		logger.Log.Debug("main StartTUIApp START")
		defer logger.Log.Debug("main StartTUIApp START")
		app.StartTUIApp()
		cancel()
	}()

	// Канал для сигналов ОС
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// Ожидаем завершения или таймаута
	select {
	case <-ctx.Done():
		logger.Log.Error("Приложение экстренно остановлено")
	case <-sigChan:
		logger.Log.Info("Получен сигнал остановки")
		app.StopTUIApp()
		cancel()
	}

	logger.Log.Info("Приложение завершено")

}

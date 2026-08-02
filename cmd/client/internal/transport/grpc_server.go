// Package transport предоставляет реализацию транспортного уровня для взаимодействия с gRPC-сервером
package transport

import (
	"context"
	"fmt"
	"time"

	pb "github.com/qesterrx/SafeKeeper/proto/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/protobuf/proto"
)

// GRPCServer представляет клиентскую часть для взаимодействия с сервером аутентификации
type GRPCServer struct{
}

// Register выполняет регистрацию нового пользователя на сервере
//
// Параметры:
//   - login: логин пользователя
//   - pswd: пароль пользователя
//   - addr: адрес сервера в формате "host:port"
//
// Возвращает:
//   - string: AES-токен для шифрования данных пользователя
//   - error: ошибка
func (srv *GRPCServer) Register(login, pswd, addr string) (string, error) {

	cred, err := getCredentials()
	if err != nil {
		return "", fmt.Errorf("ошибка соединения с сервером: %w", err)
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(*cred))
	if err != nil {
		return "", fmt.Errorf("Ошибка соединения с сервером: %w", err)
	}
	defer conn.Close()

	client := pb.NewAuthServiceClient(conn)

	// 3. Контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	regReq := &pb.RegisterRequest_builder{
		Username: proto.String(login),
		Password: proto.String(pswd),
	}

	loginResp, err := client.Register(ctx, regReq.Build())
	if err != nil {
		return "", fmt.Errorf("Ошибка регистрации: %w", err)
	}

	aesToken := loginResp.GetAesToken()

	return aesToken, nil

}

// Login выполняет аутентификацию пользователя на сервере
// Возвращает JWT-токен для последующих запросов и AES-токен для шифрования данных
//
// Параметры:
//   - login: логин пользователя
//   - pswd: пароль пользователя
//   - addr: адрес сервера в формате "host:port"
//
// Возвращает:
//   - string: JWT-токен для аутентификации запросов
//   - string: AES-токен для шифрования данных
//   - error: ошибка
func (srv *GRPCServer) Login(login, pswd, addr string) (string, string, error) {

	//Подгружаем сертификат
	cred, err := getCredentials()
	if err != nil {
		return "", "", fmt.Errorf("ошибка соединения с сервером: %w", err)
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(*cred))
	if err != nil {
		return "", "", fmt.Errorf("Ошибка соединения с сервером: %w", err)
	}
	defer conn.Close()

	client := pb.NewAuthServiceClient(conn)

	// 3. Контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	loginReq := &pb.LoginRequest_builder{
		Username: proto.String(login),
		Password: proto.String(pswd),
	}

	loginResp, err := client.Login(ctx, loginReq.Build())
	if err != nil {
		return "", "", fmt.Errorf("Ошибка авторизации: %w", err)
	}

	gwtToken := loginResp.GetJwtToken()
	aesToken := loginResp.GetAesToken()

	return gwtToken, aesToken, nil

}

// Ping проверяет доступность сервера по указанному адресу
// Проверяет состояние gRPC-соединения
func (srv *GRPCServer) Ping(addr string) error {

	//Подгружаем сертификат
	cred, err := getCredentials()
	if err != nil {
		return fmt.Errorf("ошибка соединения с сервером: %w", err)
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(*cred))

	if err != nil {
		return fmt.Errorf("Недоступен")
	}
	defer conn.Close()

	state := conn.GetState()

	if state == connectivity.Ready || state == connectivity.Idle {
		return nil
	}

	return fmt.Errorf("...")

}

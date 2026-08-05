// Package transport предоставляет реализацию транспортного уровня для взаимодействия с gRPC-сервером
package transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	pb "github.com/qesterrx/SafeKeeper/proto/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/proto"
)

// GRPCServer представляет клиентскую часть для взаимодействия с сервером аутентификации
type GRPCServer struct {
	cred *credentials.TransportCredentials
}

// NewGRPCServer создает объект для возможности авторизации/регистрации на удаленном сервере
func NewGRPCServer(certCAFile string) (*GRPCServer, error) {

	caCert, err := os.ReadFile(certCAFile)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("Не удалось добавить CA в пул")
	}

	tlsConfig := &tls.Config{
		RootCAs:            certPool,
		ServerName:         "localhost",
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS13,
	}

	cred := credentials.NewTLS(tlsConfig)

	return &GRPCServer{cred: &cred}, nil
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

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(*srv.cred))
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

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(*srv.cred))
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

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(*srv.cred))

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

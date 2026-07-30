// Package transport реализует транспортный слой (gRPC-серверы) приложения SafeKeeper
// Обеспечивает преобразование между gRPC-запросами/ответами и внутренними сервисами приложения

package transport

import (
	"context"

	pb "github.com/qesterrx/SafeKeeper/proto/auth"
)

// AuthService определяет интерфейс сервиса аутентификации, используемый транспортным слоем
// Содержит бизнес-логику регистрации и входа пользователей

type AuthService interface {
	// Register регистрирует нового пользователя
	Register(ctx context.Context, username string, password string) (string, error)

	// Login выполняет аутентификацию пользователя
	Login(ctx context.Context, username string, password string) (string, string, error)
}

// AuthServer реализует gRPC-сервер для сервиса аутентификации (auth.v1.AuthService)
type AuthServer struct {
	pb.UnimplementedAuthServiceServer
	auth AuthService
}

// NewAuthServer создаёт новый экземпляр AuthServer
// Принимает реализацию AuthService для обработки бизнес-логики
// Параметры:
//   - authService: реализация интерфейса AuthService
//
// Возвращает:
//   - *AuthServer: указатель на созданный gRPC-сервер
func NewAuthServer(authService AuthService) *AuthServer {
	return &AuthServer{auth: authService}
}

// Register обрабатывает gRPC-запрос на регистрацию нового пользователя
// Принимает имя пользователя и пароль, вызывает сервисный слой для создания пользователя
// Параметры:
//   - ctx: контекст gRPC-запроса
//   - req: запрос на регистрацию (содержит username и password)
//
// Возвращает:
//   - *pb.RegisterResponse: ответ с AES-токеном
//   - error: gRPC-статус с кодом ошибки (или nil при успехе)
func (s *AuthServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {

	var res pb.RegisterResponse

	aes, err := s.auth.Register(ctx, req.GetUsername(), req.GetPassword())
	if err != nil {
		return &res, TranslateError(err)
	}

	res.SetAesToken(aes)

	return &res, nil
}

// Login обрабатывает gRPC-запрос на вход в систему
// Принимает имя пользователя и пароль, вызывает сервисный слой для аутентификации
// Является публичным методом, доступным без JWT-аутентификации
//
// Параметры:
//   - ctx: контекст gRPC-запроса
//   - req: запрос на вход (содержит username и password)
//
// Возвращает:
//   - *pb.LoginResponse: ответ с JWT-токеном и AES-токеном
//   - error: gRPC-статус с кодом ошибки (или nil при успехе)
func (s *AuthServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {

	var res pb.LoginResponse

	jwt, aes, err := s.auth.Login(ctx, req.GetUsername(), req.GetPassword())

	if err != nil {
		return &res, TranslateError(err)
	}

	res.SetJwtToken(jwt)
	res.SetAesToken(aes)

	return &res, nil
}

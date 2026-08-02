// Package interceptor предоставляет gRPC-перехватчики (interceptors)
// Реализует сквозную функциональность: аутентификацию через JWT/AES-токены

package interceptor

import (
	"context"

	"github.com/qesterrx/SafeKeeper/cmd/server/internal/jwta"
	"github.com/qesterrx/SafeKeeper/internal/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AESChecker определяет интерфейс для проверки валидности AES-токена пользователя
// Используется в процессе аутентификации для дополнительной верификации после успешной проверки JWT
type AESChecker interface {
	// CheckAESToken проверяет, соответствует ли переданный AES-токен пользователю с указанным идентификатором
	// Возвращает true, если токен действителен, иначе false
	CheckAESToken(ctx context.Context, userId int32, aesToken string) bool
}

// JWTServerInterceptor реализует gRPC-перехватчики для аутентификации на основе JWT-токенов
// Проверяет наличие и валидность JWT-токена во входящих запросах, а также выполняет дополнительную проверку AES-токена через переданный AESChecker
// Публичные методы (регистрация, логин и др.) пропускаются без аутентификации
type JWTServerInterceptor struct {
	aesChecker    AESChecker      // Сервис для проверки AES-токенов
	publicMethods map[string]bool // Карта публичных методов, не требующих аутентификации
}

// NewJWTServerInterceptor создаёт новый экземпляр JWTServerInterceptor
// В качестве параметра принимает реализацию AESChecker для проверки AES-токенов
// Инициализирует список публичных методов, доступных без аутентификации:
//   - /auth.v1.AuthService/Register - регистрация нового пользователя
//   - /auth.v1.AuthService/Login    - вход в систему
//   - /auth.v1.AuthService/Cert     - получение сертификата
//   - /auth.v1.AuthService/Check    - проверка состояния сессии
func NewJWTServerInterceptor(aesChecker AESChecker) *JWTServerInterceptor {
	return &JWTServerInterceptor{
		aesChecker: aesChecker,
		publicMethods: map[string]bool{
			// Публичные методы, не требующие аутентификации
			"/auth.v1.AuthService/Register": true,
			"/auth.v1.AuthService/Login":    true,
			"/auth.v1.AuthService/Cert":     true,
			"/auth.v1.AuthService/Check":    true,
		},
	}
}

// Unary возвращает унарный перехватчик (UnaryServerInterceptor) для проверки JWT-токенов
//
//	Проверяет AES-токен через AESChecker
//	При успешной проверке сохраняет UserID в контексте для использования в обработчиках
//
// Возвращает ошибку с кодом Unauthenticated (код 16) в случае:
// отсутствия метаданных,
// невозможности извлечь токен,
// невалидного JWT-токена,
// невалидного AES-токена
func (j *JWTServerInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

		// Проверяем, является ли метод публичным
		if j.publicMethods[info.FullMethod] {
			logger.Log.Debug("Публичный метод %s, аутентификация не требуется", info.FullMethod)
			return handler(ctx, req)
		}

		// Извлекаем метаданные
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "метаданные отсутствуют")
		}

		// Извлекаем токен
		token, err := jwta.ExtractTokenFromMetadata(md)
		if err != nil {
			logger.Log.Debug("Ошибка извлечения токена: %v", err)
			return nil, status.Errorf(codes.Unauthenticated, "авторизация не удалась: %v", err)
		}

		// Валидируем токен
		claims, err := jwta.ValidateToken(token)
		if err != nil {
			logger.Log.Debug("Ошибка валидации токена: %v", err)
			return nil, status.Errorf(codes.Unauthenticated, "недействительный JWT токен: %v", err)
		}

		//logger.Log.Debug("Unary JWT ID=%d AES=%s", claims.Userid, claims.AESToken)

		//Проверяем ключ шифрования
		if ok := j.aesChecker.CheckAESToken(ctx, claims.Userid, claims.AESToken); !ok {
			return nil, status.Errorf(codes.Unauthenticated, "недействительный AES токен: %v", err)
		}

		// Добавляем claims в контекст
		ctx = jwta.SetUserID(ctx, claims.Userid)

		// Продолжаем выполнение
		return handler(ctx, req)
	}
}

// Stream возвращает потоковый перехватчик (StreamServerInterceptor) для проверки JWT-токенов
// Выполняет аналогичную унарному перехватчику аутентификацию, но для потоковых методов
// В отличие от унарного перехватчика, не проверяет публичные методы (все потоковые методы требуют аутентификации)
//
// При успешной проверке создаёт обёрнутый поток (wrappedServerStream) с обновлённым контекстом,
// содержащим UserID. Это позволяет обработчикам потоковых методов получать идентификатор пользователя через контекст
//
// Возвращает ошибку с кодом Unauthenticated (код 16) в случае:
// отсутствия метаданных,
// невозможности извлечь токен,
// невалидного JWT-токена,
// невалидного AES-токена
func (j *JWTServerInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

		// Проверяем токен для стримов
		ctx := ss.Context()

		// Извлекаем метаданные
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return status.Errorf(codes.Unauthenticated, "метаданные отсутствуют")
		}

		// Извлекаем токен
		token, err := jwta.ExtractTokenFromMetadata(md)
		if err != nil {
			return status.Errorf(codes.Unauthenticated, "авторизация не удалась: %v", err)
		}

		// Валидируем токен
		claims, err := jwta.ValidateToken(token)
		if err != nil {
			return status.Errorf(codes.Unauthenticated, "недействительный токен: %v", err)
		}

		//logger.Log.Debug("Stream JWT ID=%d AES=%s", claims.Userid, claims.AESToken)

		// Добавляем claims в контекст
		if ok := j.aesChecker.CheckAESToken(ctx, claims.Userid, claims.AESToken); !ok {
			return status.Errorf(codes.Unauthenticated, "недействительный AES токен: %v", err)
		}

		// Добавляем claims в контекст
		ctx = jwta.SetUserID(ctx, claims.Userid)

		// Создаем обернутый стрим с новым контекстом
		wrappedStream := &wrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		return handler(srv, wrappedStream)
	}
}

// wrappedServerStream является обёрткой над grpc.ServerStream, позволяющей подменить контекст
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context возвращает переопределённый контекст, содержащий UserID и другие значения, установленные в процессе аутентификации.
func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

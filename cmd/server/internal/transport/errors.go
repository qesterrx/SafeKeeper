// Package transport реализует транспортный слой (gRPC-серверы) приложения SafeKeeper
// Обеспечивает преобразование между gRPC-запросами/ответами и внутренними сервисами приложения

package transport

import (
	"errors"
	"strconv"

	"github.com/qesterrx/SafeKeeper/cmd/server/internal/lerrors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrInternal создаёт gRPC-ошибку с кодом Internal
// Используется для внутренних ошибок сервера, которые не следует детализировать для клиента
func ErrInternal(msg string) error {
	return status.Error(codes.Internal, msg)
}

// ErrUserAlreadyExists создаёт gRPC-ошибку с кодом AlreadyExists
// Используется при попытке регистрации пользователя с уже существующим логином
func ErrUserAlreadyExists(msg string) error {
	return status.Errorf(codes.AlreadyExists, "user already exists: %s", msg)
}

// ErrUserUnauthenticated создаёт gRPC-ошибку с кодом Unauthenticated
// Используется при неудачной попытке аутентификации (неверный логин или пароль)
func ErrUserUnauthenticated(msg string) error {
	return status.Errorf(codes.Unauthenticated, "invalid credentials for user: %s", msg)
}

// ErrObjectNotfound создаёт gRPC-ошибку с кодом NotFound
// Используется при запросе несуществующего объекта (по ID или другим критериям)
func ErrObjectNotfound(msg string) error {
	return status.Errorf(codes.NotFound, "object not found: %s", msg)
}

// ErrObjectTooOld создаёт gRPC-ошибку с кодом DataLoss
// Используется при конфликте версий в оптимистичной блокировке, когда клиент пытается обновить устаревшую версию объекта
func ErrObjectTooOld(msg string) error {
	return status.Error(codes.DataLoss, msg)
}

// TranslateError преобразует внутреннюю ошибку приложения (lerrors.LError)в стандартную gRPC-ошибку с соответствующим кодом статуса
// Выполняет сопоставление кодов ошибок:
//   - StInternalError     -> codes.Internal
//   - StUserWrongPassword -> codes.Unauthenticated
//   - StUserAlreadyExists -> codes.AlreadyExists
//   - StObjectNotFound    -> codes.NotFound
//   - StObjectTooOld      -> codes.DataLoss
//   - неизвестный код     -> codes.Internal
func TranslateError(err error) error {
	var lError *lerrors.LError
	if errors.As(err, &lError) {
		switch lError.Code {
		case lerrors.StInternalError:
			return ErrInternal(lError.Message)
		case lerrors.StUserWrongPassword:
			return ErrUserUnauthenticated(lError.Message)
		case lerrors.StUserAlreadyExists:
			return ErrUserAlreadyExists(lError.Message)
		case lerrors.StObjectNotFound:
			return ErrObjectNotfound(lError.Message)
		case lerrors.StObjectTooOld:
			return ErrObjectTooOld(lError.Message)
		default:
			return ErrInternal(strconv.Itoa(int(lError.Code)) + ":: " + lError.Message)
		}
	} else {
		return ErrInternal(err.Error())
	}
}

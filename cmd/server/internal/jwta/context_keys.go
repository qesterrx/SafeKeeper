// Package jwta предоставляет функциональность для работы с JWT (JSON Web Tokens) в системе SafeKeeper
// Реализует генерацию, валидацию и извлечение токенов из метаданных gRPC-запросов

package jwta

import "context"

type ctxKeyUserIdType string

const ctxKeyUserId ctxKeyUserIdType = "user_id"

// SetUserID - Функция для установки значения
func SetUserID(ctx context.Context, userID int32) context.Context {
	return context.WithValue(ctx, ctxKeyUserId, userID)
}

// Функция для получения значения
func GetUserID(ctx context.Context) (int32, bool) {
	userID, ok := ctx.Value(ctxKeyUserId).(int32)
	return userID, ok
}

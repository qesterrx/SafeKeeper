// Package jwta предоставляет функциональность для работы с JWT (JSON Web Tokens) в системе SafeKeeper
// Реализует генерацию, валидацию и извлечение токенов из метаданных gRPC-запросов
// Использует алгоритм подписи HS256 (HMAC-SHA256)

package jwta

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// JWTConfig содержит настройки для работы с JWT-токенами
// Используется при генерации и валидации токенов
type JWTConfig struct {
	SecretKey      string        // Секретный ключ для подписи и верификации токенов (HMAC)
	ExpirationTime time.Duration // Время жизни токена с момента создания
	Issuer         string        // Издатель токена (поле iss в claims)
}

// config содержит глобальную конфигурацию JWT для всего пакета
// Инициализируется один раз при старте сервера через InitJWTConfig
var config JWTConfig

// CustomClaims определяет пользовательские данные, сохраняемые в JWT-токене
// Расширяет стандартные RegisteredClaims библиотеки jwt-go
type CustomClaims struct {
	Userid               int32  `json:"user_id"`   // Идентификатор пользователя в системе
	AESToken             string `json:"aes_token"` // AES-токен для шифрования данных пользователя
	jwt.RegisteredClaims        // Стандартные поля JWT: exp, iat, nbf, iss и др.
}

// InitJWTConfig инициализирует глобальную конфигурацию JWT для пакета
// Должна быть вызвана до использования GenerateToken или ValidateToken
//
// Параметры:
//   - SecretKey: секретный ключ для подписи токенов (должен быть достаточно длинным и сложным)
//   - ExpirationTime: время жизни токена (например, 24 * time.Hour)
//   - Issuer: название издателя токена (используется в поле iss)
func InitJWTConfig(SecretKey string, ExpirationTime time.Duration, Issuer string) {
	config = JWTConfig{
		SecretKey:      SecretKey,
		ExpirationTime: ExpirationTime,
		Issuer:         Issuer,
	}
}

// GenerateToken создаёт новый JWT-токен с указанными пользовательскими данными
// Токен подписывается с использованием алгоритма HS256 (HMAC-SHA256)
//
// Параметры:
//   - userid: идентификатор пользователя (будет сохранён в поле Userid)
//   - aes: AES-токен пользователя для шифрования данных (будет сохранён в поле AESToken)
func GenerateToken(userid int32, aes string) (string, error) {
	claims := CustomClaims{
		Userid:   userid,
		AESToken: aes,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(config.ExpirationTime)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    config.Issuer,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.SecretKey))

}

// ValidateToken проверяет валидность JWT-токена и извлекает из него пользовательские данные
func ValidateToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{},
		func(token *jwt.Token) (interface{}, error) {
			// Проверяем алгоритм подписи
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("неподдерживаемый метод подписи: %v", token.Header["alg"])
			}
			return []byte(config.SecretKey), nil
		},
	)

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("недействительный токен")
}

// ExtractTokenFromMetadata извлекает JWT-токен из метаданных gRPC-запроса
func ExtractTokenFromMetadata(md map[string][]string) (string, error) {
	// Проверяем разные форматы заголовка Authorization
	var authHeader string

	// Стандартный формат
	if vals, ok := md["authorization"]; ok && len(vals) > 0 {
		authHeader = vals[0]
	} else if vals, ok := md["Authorization"]; ok && len(vals) > 0 {
		authHeader = vals[0]
	}

	if authHeader == "" {
		return "", errors.New("authorization заголовок отсутствует")
	}

	// Поддержка формата "Bearer <token>"
	var token string
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	} else if len(authHeader) > 6 && authHeader[:6] == "bearer " {
		token = authHeader[6:]
	} else {
		token = authHeader
	}

	if token == "" {
		return "", errors.New("токен отсутствует")
	}
	return token, nil
}

// Package service реализует бизнес-логику (сервисный слой) приложения SafeKeeper
// Содержит основные сценарии использования: регистрацию пользователей, аутентификацию и проверку токенов
// Сервисы используют репозитории (storage) для доступа к данным и обеспечивают валидацию, шифрование и генерацию токенов

package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/qesterrx/SafeKeeper/cmd/server/internal/jwta"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/lerrors"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/model"
	"github.com/qesterrx/SafeKeeper/internal/aes"
	"golang.org/x/crypto/bcrypt"
)

// AuthStorage определяет интерфейс для работы с хранилищем пользователей
//
//go:generate mockery --name=AuthStorage --output=mocks --outpkg=mocks --filename=auth_mock.go
type AuthStorage interface {
	// NewUser создаёт нового пользователя в системе
	NewUser(ctx context.Context, login string, password string, aestoken string) error

	// GetUserByLogin ищет пользователя по логину
	GetUserByLogin(ctx context.Context, login string) (*model.DBUser, error)

	// GetUserById ищет пользователя по идентификатору
	GetUserById(ctx context.Context, id int32) (*model.DBUser, error)
}

// AuthService реализует бизнес-логику аутентификации и управления пользователями
type AuthService struct {
	storage AuthStorage
}

// NewAuthService создаёт новый экземпляр AuthService
func NewAuthService(storage AuthStorage) (*AuthService, error) {
	auth := AuthService{storage: storage}
	return &auth, nil
}

// Register выполняет регистрацию нового пользователя в системе
// Важно: AES-токен генерируется на стороне сервера и не передаётся клиенту в открытом виде
// Клиент получает только зашифрованную версию, которую должен расшифровать своим паролем
func (auths *AuthService) Register(ctx context.Context, username string, password string) (string, error) {

	if password == "" || username == "" {
		return "", lerrors.NewLEUserWrongPassword(fmt.Errorf("необходимо передать логин/пароль"), "необходимо передать логин/пароль")
	}

	// Хэшируем пароль
	hashedPswd, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", lerrors.NewLEInternalError(err, "ошибка при шифровании пароля")
	}

	//Получаем токен для шифрования 32 байта
	aeskey := make([]byte, 1024)
	_, err = rand.Read(aeskey)
	if err != nil {
		return "", lerrors.NewLEInternalError(err, "ошибка получения токена шифрования")
	}

	//Шифруем его паролем
	sha := sha256.Sum256([]byte(password))
	AESToken, err := aes.EncryptGCM(aeskey, sha[:])
	if err != nil {
		return "", lerrors.NewLEInternalError(err, "ошибка шифрования токена шифрования")
	}

	AESTokenStr := base64.StdEncoding.EncodeToString(AESToken)

	err = auths.storage.NewUser(ctx, username, string(hashedPswd), AESTokenStr)
	if err != nil {
		return "", err
	}

	return AESTokenStr, nil
}

// Login выполняет аутентификацию пользователя в системе
func (auths *AuthService) Login(ctx context.Context, username string, password string) (string, string, error) {

	user, err := auths.storage.GetUserByLogin(ctx, username)
	if err != nil {
		return "", "", err
	}

	// Проверяем пароль
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", "", lerrors.NewLEUserWrongPassword(err, fmt.Sprintf("Логин/пароль некорректны [%s]", username))
	}

	// Генерируем JWT токен
	JWTToken, err := jwta.GenerateToken(int32(user.ID), user.AESToken)
	if err != nil {
		return "", "", lerrors.NewLEInternalError(err, "ошибка генерации JWT токена "+err.Error())
	}

	return JWTToken, user.AESToken, nil

}

// CheckAESToken проверяет, соответствует ли переданный AES-токен токену пользователя
// Используется в gRPC-перехватчиках для дополнительной верификации после проверки JWT на случай смены пароля в одной из сессий
func (auths *AuthService) CheckAESToken(ctx context.Context, userId int32, aesToken string) bool {

	user, err := auths.storage.GetUserById(ctx, userId)
	if err != nil {
		return false
	}

	return user.AESToken == aesToken

}

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/qesterrx/SafeKeeper/cmd/server/internal/lerrors"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/model"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/service/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthService_Register(t *testing.T) {
	// Создаем мок
	mockStorage := mocks.NewAuthStorage(t)

	// Настраиваем ожидания
	mockStorage.On("NewUser", mock.Anything, "UnExistedUser",
		mock.MatchedBy(func(password string) bool {
			// Проверяем, что пароль подходит
			if err := bcrypt.CompareHashAndPassword([]byte(password), []byte("password")); err != nil {
				return false
			}
			return true
		}),
		mock.MatchedBy(func(aest string) bool {
			//Проверяем что токен не пустой
			return aest != ""
		})).Return(nil)

	mockStorage.On("NewUser", mock.Anything, "ExistedUser", mock.Anything, mock.Anything).Return(lerrors.NewLEUserAlreadyExists(fmt.Errorf("error inside"), "error"))
	mockStorage.On("NewUser", mock.Anything, "InternalErrorUser", mock.Anything, mock.Anything).Return(lerrors.NewLEInternalError(fmt.Errorf("error inside"), "error"))

	// Создаем сервис с моком
	service, err := NewAuthService(mockStorage)
	require.NoError(t, err, "NewAuthService() error")

	// Выполняем тестируемую функцию

	//Успех
	aest, err := service.Register(context.Background(), "UnExistedUser", "password")
	require.NoError(t, err, "Register success")
	require.NotEmpty(t, aest)

	//Ошибки
	var lErr *lerrors.LError

	aest, err = service.Register(context.Background(), "login", "")
	require.ErrorAs(t, err, &lErr)
	require.Equal(t, lerrors.StUserWrongPassword, lErr.Code)
	require.Equal(t, "", aest)

	aest, err = service.Register(context.Background(), "", "")
	require.ErrorAs(t, err, &lErr)
	require.Equal(t, lerrors.StUserWrongPassword, lErr.Code)
	require.Equal(t, "", aest)

	aest, err = service.Register(context.Background(), "ExistedUser", "password")
	require.ErrorAs(t, err, &lErr)
	require.Equal(t, lerrors.StUserAlreadyExists, lErr.Code)
	require.Equal(t, "", aest)

	aest, err = service.Register(context.Background(), "InternalErrorUser", "password")
	require.ErrorAs(t, err, &lErr)
	require.Equal(t, lerrors.StInternalError, lErr.Code)
	require.Equal(t, "", aest)

	// Проверяем, что все ожидания выполнены
	mockStorage.AssertExpectations(t)
}

func TestAuthService_Login(t *testing.T) {
	// Создаем мок
	mockStorage := mocks.NewAuthStorage(t)

	user := model.DBUser{
		ID:       1,
		Login:    "ExistedUser",
		Password: "$2a$10$Y6fLdSln/2quB1DPQ2B8gulGwWtQa7IfjtJt0UzwQMnrKY9WjYM5S",
		AESToken: "aestoken",
	}

	// Настраиваем ожидания
	mockStorage.On("GetUserByLogin", mock.Anything, "ExistedUser").Return(&user, nil)
	mockStorage.On("GetUserByLogin", mock.Anything, "ExistedWrongPasswordUser").Return(&user, nil)
	mockStorage.On("GetUserByLogin", mock.Anything, "UnExistedWrongPasswordUser").Return(nil, lerrors.NewLEUserWrongPassword(fmt.Errorf("error inside"), "error"))
	mockStorage.On("GetUserByLogin", mock.Anything, "InternalErrorUser").Return(nil, lerrors.NewLEInternalError(fmt.Errorf("error inside"), "error"))

	// Создаем сервис с моком
	service, err := NewAuthService(mockStorage)
	require.NoError(t, err, "NewAuthService() error")

	// Выполняем тестируемую функцию

	//Успех
	jwtt, aest, err := service.Login(context.Background(), "ExistedUser", "password")
	require.NoError(t, err, "Login success")
	require.NotEmpty(t, jwtt)
	require.Equal(t, "aestoken", aest)

	//Ошибки
	var lErr *lerrors.LError

	jwtt, aest, err = service.Login(context.Background(), "ExistedWrongPasswordUser", "password2")
	require.ErrorAs(t, err, &lErr)
	require.Equal(t, lerrors.StUserWrongPassword, lErr.Code)
	require.Empty(t, jwtt)
	require.Empty(t, aest)

	jwtt, aest, err = service.Login(context.Background(), "UnExistedWrongPasswordUser", "")
	require.ErrorAs(t, err, &lErr)
	require.Equal(t, lerrors.StUserWrongPassword, lErr.Code)
	require.Empty(t, jwtt)
	require.Empty(t, aest)

	jwtt, aest, err = service.Login(context.Background(), "InternalErrorUser", "")
	require.ErrorAs(t, err, &lErr)
	require.Equal(t, lerrors.StInternalError, lErr.Code)
	require.Empty(t, jwtt)
	require.Empty(t, aest)

	// Проверяем, что все ожидания выполнены
	mockStorage.AssertExpectations(t)
}

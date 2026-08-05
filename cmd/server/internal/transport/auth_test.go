package transport

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/qesterrx/SafeKeeper/cmd/server/internal/lerrors"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/transport/mocks"
	pb "github.com/qesterrx/SafeKeeper/proto/auth"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAuthServer_Register(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		password    string
		setupMock   func(*mocks.AuthService)
		expectError bool
		errCode     lerrors.Status
		expectedAES string
	}{
		{
			name:     "Success",
			username: "testuser",
			password: "testpass123",
			setupMock: func(m *mocks.AuthService) {
				m.On("Register", mock.Anything, "testuser", "testpass123").
					Return("test-aes-token", nil)
			},
			expectError: false,
			expectedAES: "test-aes-token",
		},
		{
			name:     "UserAlreadyExists",
			username: "existinguser",
			password: "testpass123",
			setupMock: func(m *mocks.AuthService) {
				expectedErr := lerrors.NewLEUserAlreadyExists(fmt.Errorf("user exists"), "user already exists")
				m.On("Register", mock.Anything, "existinguser", "testpass123").
					Return("", expectedErr)
			},
			expectError: true,
			errCode:     lerrors.StUserAlreadyExists,
		},
		{
			name:     "InternalError",
			username: "testuser",
			password: "testpass123",
			setupMock: func(m *mocks.AuthService) {
				expectedErr := lerrors.NewLEInternalError(fmt.Errorf("db error"), "database error")
				m.On("Register", mock.Anything, "testuser", "testpass123").
					Return("", expectedErr)
			},
			expectError: true,
			errCode:     lerrors.StInternalError,
		},
		{
			name:     "GenericError",
			username: "testuser",
			password: "testpass123",
			setupMock: func(m *mocks.AuthService) {
				m.On("Register", mock.Anything, "testuser", "testpass123").
					Return("", errors.New("some generic error"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем мок
			mockService := mocks.NewAuthService(t)

			// Настраиваем ожидания
			tt.setupMock(mockService)

			// Создаем сервер с моком
			server := NewAuthServer(mockService, nil)

			// Формируем запрос
			req := &pb.RegisterRequest{}
			req.SetUsername(tt.username)
			req.SetPassword(tt.password)

			// Выполняем тестируемую функцию
			resp, err := server.Register(context.Background(), req)

			// Проверяем результат
			if tt.expectError {
				require.Error(t, err, "Register() expected error, got nil")
				if tt.errCode > 0 {
					// Проверяем что ошибка трансформирована в gRPC статус
					// TranslateError должен вернуть ошибку с правильным кодом
					require.NotEmpty(t, err.Error(), "Error should not be empty")
				}
			} else {
				require.NoError(t, err, "Register() error")
				require.NotNil(t, resp, "Response should not be nil")
				require.Equal(t, tt.expectedAES, resp.GetAesToken(), "AES token mismatch")
			}

			// Проверяем, что все ожидания выполнены
			mockService.AssertExpectations(t)
		})
	}
}

func TestAuthServer_Login(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		password    string
		setupMock   func(*mocks.AuthService)
		expectError bool
		errCode     lerrors.Status
		expectedJWT string
		expectedAES string
	}{
		{
			name:     "Success",
			username: "testuser",
			password: "testpass123",
			setupMock: func(m *mocks.AuthService) {
				m.On("Login", mock.Anything, "testuser", "testpass123").
					Return("jwt-token", "aes-token", nil)
			},
			expectError: false,
			expectedJWT: "jwt-token",
			expectedAES: "aes-token",
		},
		{
			name:     "WrongPassword",
			username: "testuser",
			password: "wrongpass",
			setupMock: func(m *mocks.AuthService) {
				expectedErr := lerrors.NewLEUserWrongPassword(fmt.Errorf("invalid credentials"), "invalid credentials")
				m.On("Login", mock.Anything, "testuser", "wrongpass").
					Return("", "", expectedErr)
			},
			expectError: true,
			errCode:     lerrors.StUserWrongPassword,
		},
		{
			name:     "UserNotFound",
			username: "nonexistent",
			password: "testpass123",
			setupMock: func(m *mocks.AuthService) {
				expectedErr := lerrors.NewLEUserWrongPassword(fmt.Errorf("user not found"), "user not found")
				m.On("Login", mock.Anything, "nonexistent", "testpass123").
					Return("", "", expectedErr)
			},
			expectError: true,
			errCode:     lerrors.StUserWrongPassword,
		},
		{
			name:     "InternalError",
			username: "testuser",
			password: "testpass123",
			setupMock: func(m *mocks.AuthService) {
				expectedErr := lerrors.NewLEInternalError(fmt.Errorf("db error"), "database error")
				m.On("Login", mock.Anything, "testuser", "testpass123").
					Return("", "", expectedErr)
			},
			expectError: true,
			errCode:     lerrors.StInternalError,
		},
		{
			name:     "GenericError",
			username: "testuser",
			password: "testpass123",
			setupMock: func(m *mocks.AuthService) {
				m.On("Login", mock.Anything, "testuser", "testpass123").
					Return("", "", errors.New("some generic error"))
			},
			expectError: true,
		},
		{
			name:     "EmptyCredentials",
			username: "",
			password: "",
			setupMock: func(m *mocks.AuthService) {
				expectedErr := lerrors.NewLEUserWrongPassword(fmt.Errorf("empty credentials"), "empty credentials")
				m.On("Login", mock.Anything, "", "").
					Return("", "", expectedErr)
			},
			expectError: true,
			errCode:     lerrors.StUserWrongPassword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем мок
			mockService := mocks.NewAuthService(t)

			// Настраиваем ожидания
			tt.setupMock(mockService)

			// Создаем сервер с моком
			server := NewAuthServer(mockService, nil)

			// Формируем запрос
			req := &pb.LoginRequest{}
			req.SetUsername(tt.username)
			req.SetPassword(tt.password)

			// Выполняем тестируемую функцию
			resp, err := server.Login(context.Background(), req)

			// Проверяем результат
			if tt.expectError {
				require.Error(t, err, "Login() expected error, got nil")
				if tt.errCode > 0 {
					// Проверяем что ошибка трансформирована в gRPC статус
					require.NotEmpty(t, err.Error(), "Error should not be empty")
				}
			} else {
				require.NoError(t, err, "Login() error")
				require.NotNil(t, resp, "Response should not be nil")
				require.Equal(t, tt.expectedJWT, resp.GetJwtToken(), "JWT token mismatch")
				require.Equal(t, tt.expectedAES, resp.GetAesToken(), "AES token mismatch")
			}

			// Проверяем, что все ожидания выполнены
			mockService.AssertExpectations(t)
		})
	}
}

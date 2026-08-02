package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/qesterrx/SafeKeeper/cmd/server/internal/lerrors"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/model"
)

// MockAuthStorage implements AuthStorage interface for testing
type MockAuthStorage struct {
	NewUserFunc        func(ctx context.Context, login, password, aestoken string) error
	GetUserByLoginFunc func(ctx context.Context, login string) (*model.DBUser, error)
	GetUserByIdFunc    func(ctx context.Context, id int32) (*model.DBUser, error)
}

func (m *MockAuthStorage) NewUser(ctx context.Context, login, password, aestoken string) error {
	if m.NewUserFunc != nil {
		return m.NewUserFunc(ctx, login, password, aestoken)
	}
	return nil
}

func (m *MockAuthStorage) GetUserByLogin(ctx context.Context, login string) (*model.DBUser, error) {
	if m.GetUserByLoginFunc != nil {
		return m.GetUserByLoginFunc(ctx, login)
	}
	return &model.DBUser{ID: 1, Login: login, Password: "hashedpass", AESToken: "aes-token"}, nil
}

func (m *MockAuthStorage) GetUserById(ctx context.Context, id int32) (*model.DBUser, error) {
	if m.GetUserByIdFunc != nil {
		return m.GetUserByIdFunc(ctx, id)
	}
	return &model.DBUser{ID: id, Login: "testuser", Password: "hashedpass", AESToken: "aes-token"}, nil
}

func TestAuthService_Register_Success(t *testing.T) {
	mockStorage := &MockAuthStorage{
		NewUserFunc: func(ctx context.Context, login, password, aestoken string) error {
			return nil
		},
	}
	service, err := NewAuthService(mockStorage)
	if err != nil {
		t.Fatalf("NewAuthService() error = %v", err)
	}

	aesToken, err := service.Register(context.Background(), "newuser", "password123")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if aesToken == "" {
		t.Error("Register() returned empty AES token")
	}
}

func TestAuthService_Register_DuplicateUser(t *testing.T) {
	mockStorage := &MockAuthStorage{
		NewUserFunc: func(ctx context.Context, login, password, aestoken string) error {
			return lerrors.NewLEUserAlreadyExists(fmt.Errorf("user exists"), "user exists")
		},
	}
	service, err := NewAuthService(mockStorage)
	if err != nil {
		t.Fatalf("NewAuthService() error = %v", err)
	}

	_, err = service.Register(context.Background(), "existinguser", "password123")
	if err == nil {
		t.Error("Register() expected error, got nil")
	}

	var lErr *lerrors.LError
	if !errors.As(err, &lErr) {
		t.Errorf("Register() error type = %T, want *lerrors.LError", err)
	}
	if lErr.Code != lerrors.StUserAlreadyExists {
		t.Errorf("Error code = %v, want %v", lErr.Code, lerrors.StUserAlreadyExists)
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	mockStorage := &MockAuthStorage{
		GetUserByLoginFunc: func(ctx context.Context, login string) (*model.DBUser, error) {
			hashedPassword := "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
			return &model.DBUser{
				ID:       1,
				Login:    login,
				Password: hashedPassword,
				AESToken: "aes-token",
			}, nil
		},
	}
	service, err := NewAuthService(mockStorage)
	if err != nil {
		t.Fatalf("NewAuthService() error = %v", err)
	}

	_, _, err = service.Login(context.Background(), "testuser", "wrongpassword")
	if err == nil {
		t.Error("Login() expected error, got nil")
	}

	var lErr *lerrors.LError
	if errors.As(err, &lErr) {
		if lErr.Code != lerrors.StUserWrongPassword {
			t.Errorf("Error code = %v, want %v", lErr.Code, lerrors.StUserWrongPassword)
		}
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	mockStorage := &MockAuthStorage{
		GetUserByLoginFunc: func(ctx context.Context, login string) (*model.DBUser, error) {
			return nil, lerrors.NewLEUserWrongPassword(fmt.Errorf("user not found"), "user not found")
		},
	}
	service, err := NewAuthService(mockStorage)
	if err != nil {
		t.Fatalf("NewAuthService() error = %v", err)
	}

	_, _, err = service.Login(context.Background(), "nonexistent", "password123")
	if err == nil {
		t.Error("Login() expected error, got nil")
	}
}

func TestAuthService_CheckAESToken_Success(t *testing.T) {
	mockStorage := &MockAuthStorage{
		GetUserByIdFunc: func(ctx context.Context, id int32) (*model.DBUser, error) {
			return &model.DBUser{
				ID:       id,
				Login:    "testuser",
				Password: "hashed",
				AESToken: "correct-aes-token",
			}, nil
		},
	}
	service, err := NewAuthService(mockStorage)
	if err != nil {
		t.Fatalf("NewAuthService() error = %v", err)
	}

	result := service.CheckAESToken(context.Background(), 1, "correct-aes-token")
	if !result {
		t.Error("CheckAESToken() returned false, expected true")
	}
}

func TestAuthService_CheckAESToken_Failure(t *testing.T) {
	mockStorage := &MockAuthStorage{
		GetUserByIdFunc: func(ctx context.Context, id int32) (*model.DBUser, error) {
			return &model.DBUser{
				ID:       id,
				Login:    "testuser",
				Password: "hashed",
				AESToken: "correct-aes-token",
			}, nil
		},
	}
	service, err := NewAuthService(mockStorage)
	if err != nil {
		t.Fatalf("NewAuthService() error = %v", err)
	}

	result := service.CheckAESToken(context.Background(), 1, "wrong-aes-token")
	if result {
		t.Error("CheckAESToken() returned true, expected false")
	}
}

func TestAuthService_CheckAESToken_UserNotFound(t *testing.T) {
	mockStorage := &MockAuthStorage{
		GetUserByIdFunc: func(ctx context.Context, id int32) (*model.DBUser, error) {
			return nil, errors.New("user not found")
		},
	}
	service, err := NewAuthService(mockStorage)
	if err != nil {
		t.Fatalf("NewAuthService() error = %v", err)
	}

	result := service.CheckAESToken(context.Background(), 999, "any-token")
	if result {
		t.Error("CheckAESToken() returned true for non-existent user, expected false")
	}
}

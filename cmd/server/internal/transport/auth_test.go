package transport

import (
	"context"
	"errors"
	"testing"

	"github.com/qesterrx/SafeKeeper/cmd/server/internal/lerrors"
	pb "github.com/qesterrx/SafeKeeper/proto/auth"
)

// MockAuthService implements AuthService interface for testing
type MockAuthService struct {
	RegisterFunc func(ctx context.Context, username, password string) (string, error)
	LoginFunc    func(ctx context.Context, username, password string) (string, string, error)
}

func (m *MockAuthService) Register(ctx context.Context, username, password string) (string, error) {
	if m.RegisterFunc != nil {
		return m.RegisterFunc(ctx, username, password)
	}
	return "mock-aes-token", nil
}

func (m *MockAuthService) Login(ctx context.Context, username, password string) (string, string, error) {
	if m.LoginFunc != nil {
		return m.LoginFunc(ctx, username, password)
	}
	return "mock-jwt-token", "mock-aes-token", nil
}

func TestAuthServer_Register_Success(t *testing.T) {
	mockService := &MockAuthService{
		RegisterFunc: func(ctx context.Context, username, password string) (string, error) {
			return "test-aes-token", nil
		},
	}
	server := NewAuthServer(mockService)

	req := &pb.RegisterRequest{}
	req.SetUsername("testuser")
	req.SetPassword("testpass123")

	resp, err := server.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if resp.GetAesToken() != "test-aes-token" {
		t.Errorf("AesToken = %v, want test-aes-token", resp.GetAesToken())
	}
}

func TestAuthServer_Register_Error(t *testing.T) {
	mockService := &MockAuthService{
		RegisterFunc: func(ctx context.Context, username, password string) (string, error) {
			return "", lerrors.NewLEUserAlreadyExists("user exists")
		},
	}
	server := NewAuthServer(mockService)

	req := &pb.RegisterRequest{}
	req.SetUsername("testuser")
	req.SetPassword("testpass123")

	_, err := server.Register(context.Background(), req)
	if err == nil {
		t.Error("Register() expected error, got nil")
	}

	// Should be translated to gRPC error
	if err.Error() == "" {
		t.Error("Register() error should not be empty")
	}
}

func TestAuthServer_Login_Success(t *testing.T) {
	mockService := &MockAuthService{
		LoginFunc: func(ctx context.Context, username, password string) (string, string, error) {
			return "jwt-token", "aes-token", nil
		},
	}
	server := NewAuthServer(mockService)

	req := &pb.LoginRequest{}
	req.SetUsername("testuser")
	req.SetPassword("testpass123")

	resp, err := server.Login(context.Background(), req)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if resp.GetJwtToken() != "jwt-token" {
		t.Errorf("JwtToken = %v, want jwt-token", resp.GetJwtToken())
	}
	if resp.GetAesToken() != "aes-token" {
		t.Errorf("AesToken = %v, want aes-token", resp.GetAesToken())
	}
}

func TestAuthServer_Login_InvalidCredentials(t *testing.T) {
	mockService := &MockAuthService{
		LoginFunc: func(ctx context.Context, username, password string) (string, string, error) {
			return "", "", lerrors.NewLEUserWrongPassword("invalid credentials")
		},
	}
	server := NewAuthServer(mockService)

	req := &pb.LoginRequest{}
	req.SetUsername("testuser")
	req.SetPassword("testpass123")

	_, err := server.Login(context.Background(), req)
	if err == nil {
		t.Error("Login() expected error, got nil")
	}
}

func TestAuthServer_Login_InternalError(t *testing.T) {
	mockService := &MockAuthService{
		LoginFunc: func(ctx context.Context, username, password string) (string, string, error) {
			return "", "", errors.New("database connection failed")
		},
	}
	server := NewAuthServer(mockService)

	req := &pb.LoginRequest{}
	req.SetUsername("testuser")
	req.SetPassword("testpass123")

	_, err := server.Login(context.Background(), req)
	if err == nil {
		t.Error("Login() expected error, got nil")
	}
}

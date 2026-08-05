package interceptor

import (
	"context"
	"testing"

	"github.com/qesterrx/SafeKeeper/cmd/server/internal/jwta"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type TestAESChecker struct {
	CheckFunc func(ctx context.Context, userId int32, aesToken string) bool
}

func (m *TestAESChecker) CheckAESToken(ctx context.Context, userId int32, aesToken string) bool {
	if m.CheckFunc != nil {
		return m.CheckFunc(ctx, userId, aesToken)
	}
	return true
}

func TestJWTServerInterceptor_Unary_PublicMethod(t *testing.T) {
	checker := &TestAESChecker{}
	interceptor := NewJWTServerInterceptor(checker)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/auth.v1.AuthService/Login",
	}

	_, err := interceptor.Unary()(context.Background(), nil, info, handler)
	if err != nil {
		t.Errorf("Unary() error = %v, want nil for public method", err)
	}
}

func TestJWTServerInterceptor_Unary_NoToken(t *testing.T) {
	checker := &TestAESChecker{}
	interceptor := NewJWTServerInterceptor(checker)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	ctx := context.Background() // No metadata
	info := &grpc.UnaryServerInfo{
		FullMethod: "/data.v1.DataService/GetObject",
	}

	_, err := interceptor.Unary()(ctx, nil, info, handler)
	if err == nil {
		t.Error("Unary() expected error for missing token, got nil")
	}
}

func TestJWTServerInterceptor_Unary_InvalidToken(t *testing.T) {
	checker := &TestAESChecker{}
	interceptor := NewJWTServerInterceptor(checker)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	md := metadata.Pairs("authorization", "Bearer invalid-token")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	info := &grpc.UnaryServerInfo{
		FullMethod: "/data.v1.DataService/GetObject",
	}

	_, err := interceptor.Unary()(ctx, nil, info, handler)
	if err == nil {
		t.Error("Unary() expected error for invalid token, got nil")
	}
}

func TestJWTServerInterceptor_Unary_InvalidAESToken(t *testing.T) {
	jwta.InitJWTConfig("test-secret-key-32-bytes-long!!!", 24*60*60, "test-issuer")
	token, _ := jwta.GenerateToken(123, "aes-token")

	checker := &TestAESChecker{
		CheckFunc: func(ctx context.Context, userId int32, aesToken string) bool {
			return false // Always fail
		},
	}
	interceptor := NewJWTServerInterceptor(checker)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	md := metadata.Pairs("authorization", "Bearer "+token)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	info := &grpc.UnaryServerInfo{
		FullMethod: "/data.v1.DataService/GetObject",
	}

	_, err := interceptor.Unary()(ctx, nil, info, handler)
	if err == nil {
		t.Error("Unary() expected error for invalid AES token, got nil")
	}
}

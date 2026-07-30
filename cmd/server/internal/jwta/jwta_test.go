package jwta

import (
	"testing"
	"time"
)

func TestInitJWTConfig(t *testing.T) {
	secret := "test-secret-key"
	expiration := 24 * time.Hour
	issuer := "test-issuer"

	InitJWTConfig(secret, expiration, issuer)

	if config.SecretKey != secret {
		t.Errorf("SecretKey = %v, want %v", config.SecretKey, secret)
	}
	if config.ExpirationTime != expiration {
		t.Errorf("ExpirationTime = %v, want %v", config.ExpirationTime, expiration)
	}
	if config.Issuer != issuer {
		t.Errorf("Issuer = %v, want %v", config.Issuer, issuer)
	}
}

func TestGenerateToken(t *testing.T) {
	InitJWTConfig("test-secret-key", 24*time.Hour, "test-issuer")

	userID := int32(123)
	aesToken := "test-aes-token"

	token, err := GenerateToken(userID, aesToken)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if token == "" {
		t.Error("GenerateToken() returned empty token")
	}
}

func TestValidateToken(t *testing.T) {
	secret := "test-secret-key"
	InitJWTConfig(secret, 24*time.Hour, "test-issuer")

	userID := int32(123)
	aesToken := "test-aes-token"

	tokenString, err := GenerateToken(userID, aesToken)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	claims, err := ValidateToken(tokenString)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}

	if claims.Userid != userID {
		t.Errorf("Userid = %v, want %v", claims.Userid, userID)
	}
	if claims.AESToken != aesToken {
		t.Errorf("AESToken = %v, want %v", claims.AESToken, aesToken)
	}
}

func TestValidateToken_InvalidToken(t *testing.T) {
	InitJWTConfig("test-secret-key", 24*time.Hour, "test-issuer")

	tests := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"invalid format", "invalid.token.format"},
		{"malformed", "malformed-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateToken(tt.token)
			if err == nil {
				t.Error("ValidateToken() expected error, got nil")
			}
		})
	}
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	secret := "test-secret-key"
	// Устанавливаем отрицательное время жизни для создания истекшего токена
	InitJWTConfig(secret, -1*time.Hour, "test-issuer")

	tokenString, err := GenerateToken(123, "test")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	// Восстанавливаем нормальную конфигурацию для валидации
	InitJWTConfig(secret, 24*time.Hour, "test-issuer")

	_, err = ValidateToken(tokenString)
	if err == nil {
		t.Error("ValidateToken() expected error for expired token, got nil")
	}
}

func TestExtractTokenFromMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string][]string
		expected string
		wantErr  bool
	}{
		{
			name: "valid bearer token",
			metadata: map[string][]string{
				"authorization": {"Bearer valid-token"},
			},
			expected: "valid-token",
			wantErr:  false,
		},
		{
			name: "valid bearer token with capital A",
			metadata: map[string][]string{
				"Authorization": {"Bearer valid-token"},
			},
			expected: "valid-token",
			wantErr:  false,
		},
		{
			name: "token without bearer prefix",
			metadata: map[string][]string{
				"authorization": {"valid-token"},
			},
			expected: "valid-token",
			wantErr:  false,
		},
		{
			name: "missing authorization header",
			metadata: map[string][]string{
				"other": {"value"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := ExtractTokenFromMetadata(tt.metadata)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractTokenFromMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && token != tt.expected {
				t.Errorf("ExtractTokenFromMetadata() = %v, want %v", token, tt.expected)
			}
		})
	}
}

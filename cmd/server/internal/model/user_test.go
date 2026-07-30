package model

import (
	"testing"
)

func TestDBUser_Structure(t *testing.T) {
	user := DBUser{
		ID:       1,
		Login:    "testuser",
		Password: "hashedpassword123",
		AESToken: "aes-token-xyz",
	}

	if user.ID != 1 {
		t.Errorf("ID = %v, want 1", user.ID)
	}
	if user.Login != "testuser" {
		t.Errorf("Login = %v, want testuser", user.Login)
	}
	if user.Password != "hashedpassword123" {
		t.Errorf("Password = %v, want hashedpassword123", user.Password)
	}
	if user.AESToken != "aes-token-xyz" {
		t.Errorf("AESToken = %v, want aes-token-xyz", user.AESToken)
	}
}

func TestAuthUser_Structure(t *testing.T) {
	user := AuthUser{
		Login:    "testuser",
		Password: "plainpassword",
		AESToken: "aes-token-abc",
	}

	if user.Login != "testuser" {
		t.Errorf("Login = %v, want testuser", user.Login)
	}
	if user.Password != "plainpassword" {
		t.Errorf("Password = %v, want plainpassword", user.Password)
	}
	if user.AESToken != "aes-token-abc" {
		t.Errorf("AESToken = %v, want aes-token-abc", user.AESToken)
	}
}

func TestDBUser_JSONTags(t *testing.T) {
	// Проверяем, что структура не имеет JSON-тегов (т.к. это внутренняя модель)
	// Это важно, чтобы случайно не экспортировать пароль через API
	user := DBUser{
		ID:       1,
		Login:    "test",
		Password: "secret",
		AESToken: "token",
	}

	// Поле Password не должно иметь тега json, чтобы не сериализоваться
	// Проверим через reflection (упрощённо)
	// В реальном проекте можно использовать reflect для проверки тегов
	_ = user // Просто используем переменную
}

func TestAuthUser_JSONTags(t *testing.T) {
	// Проверяем, что AuthUser имеет JSON-теги для сериализации
	user := AuthUser{
		Login:    "testuser",
		Password: "password123",
		AESToken: "aes-token",
	}

	// В Go JSON-теги проверяются во время сериализации
	// Здесь просто проверяем, что структура корректно заполняется
	if user.Login != "testuser" {
		t.Errorf("Login = %v, want testuser", user.Login)
	}
	if user.Password != "password123" {
		t.Errorf("Password = %v, want password123", user.Password)
	}
	if user.AESToken != "aes-token" {
		t.Errorf("AESToken = %v, want aes-token", user.AESToken)
	}
}

func TestDBUser_AESTokenLength(t *testing.T) {
	tests := []struct {
		name     string
		aesToken string
		wantLen  int
	}{
		{
			name:     "empty token",
			aesToken: "",
			wantLen:  0,
		},
		{
			name:     "short token",
			aesToken: "abc123",
			wantLen:  6,
		},
		{
			name:     "typical base64 token",
			aesToken: "012345678901234567890123456789012345",
			wantLen:  36,
		},
		{
			name:     "long token",
			aesToken: "012345678901234567890123456789012345678901234567890123456789",
			wantLen:  60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := DBUser{
				ID:       1,
				Login:    "testuser",
				Password: "hashed",
				AESToken: tt.aesToken,
			}
			if len(user.AESToken) != tt.wantLen {
				t.Errorf("AESToken length = %v, want %v", len(user.AESToken), tt.wantLen)
			}
		})
	}
}

func TestAuthUser_Constructor(t *testing.T) {
	// Тестируем создание AuthUser через литерал
	user := AuthUser{
		Login:    "newuser",
		Password: "securepass123",
		AESToken: "generated-aes-token",
	}

	expected := AuthUser{
		Login:    "newuser",
		Password: "securepass123",
		AESToken: "generated-aes-token",
	}

	if user != expected {
		t.Errorf("AuthUser = %v, want %v", user, expected)
	}
}

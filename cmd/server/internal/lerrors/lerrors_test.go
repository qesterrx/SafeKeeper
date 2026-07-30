package lerrors

import (
	"testing"
)

func TestLError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *LError
		expected string
	}{
		{
			name:     "internal error",
			err:      NewLEInternalError("database connection failed"),
			expected: "[1] database connection failed",
		},
		{
			name:     "wrong password",
			err:      NewLEUserWrongPassword("user@example.com"),
			expected: "[2] user@example.com",
		},
		{
			name:     "user already exists",
			err:      NewLEUserAlreadyExists("admin"),
			expected: "[3] admin",
		},
		{
			name:     "object not found",
			err:      NewLEObjectNotFound("123"),
			expected: "[4] 123",
		},
		{
			name:     "object too old",
			err:      NewLEObjectTooOld("version mismatch"),
			expected: "[5] version mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("LError.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLError_ImplementsError(t *testing.T) {
	var _ error = &LError{}
	var _ error = NewLEInternalError("test")
	var _ error = NewLEUserWrongPassword("test")
	var _ error = NewLEUserAlreadyExists("test")
	var _ error = NewLEObjectNotFound("test")
	var _ error = NewLEObjectTooOld("test")
}

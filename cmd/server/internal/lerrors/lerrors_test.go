package lerrors

import (
	"fmt"
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
			err:      NewLEInternalError(fmt.Errorf("internal error inside"), "internal error"),
			expected: fmt.Sprintf("[%d] %s \n %s", StInternalError, "internal error", "internal error inside"),
		},
		{
			name:     "wrong password",
			err:      NewLEUserWrongPassword(fmt.Errorf("wrong password inside"), "wrong password"),
			expected: fmt.Sprintf("[%d] %s \n %s", StUserWrongPassword, "wrong password", "wrong password inside"),
		},
		{
			name:     "user already exists",
			err:      NewLEUserAlreadyExists(fmt.Errorf("user already exists inside"), "user already exists"),
			expected: fmt.Sprintf("[%d] %s \n %s", StUserAlreadyExists, "user already exists", "user already exists inside"),
		},
		{
			name:     "object not found",
			err:      NewLEObjectNotFound(fmt.Errorf("object not found inside"), "object not found"),
			expected: fmt.Sprintf("[%d] %s \n %s", StObjectNotFound, "object not found", "object not found inside"),
		},
		{
			name:     "object too old",
			err:      NewLEObjectTooOld(fmt.Errorf("object too old inside"), "object too old"),
			expected: fmt.Sprintf("[%d] %s \n %s", StObjectTooOld, "object too old", "object too old inside"),
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
	var _ error = NewLEInternalError(fmt.Errorf("test"), "test")
	var _ error = NewLEUserWrongPassword(fmt.Errorf("test"), "test")
	var _ error = NewLEUserAlreadyExists(fmt.Errorf("test"), "test")
	var _ error = NewLEObjectNotFound(fmt.Errorf("test"), "test")
	var _ error = NewLEObjectTooOld(fmt.Errorf("test"), "test")
}

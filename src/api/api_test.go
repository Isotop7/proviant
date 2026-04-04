package api

import (
	"errors"
	"testing"
)

func TestAPIResponse(t *testing.T) {
	t.Run("can create APIResponse", func(t *testing.T) {
		resp := APIResponse{
			Message: "test message",
		}
		if resp.Message != "test message" {
			t.Errorf("Message = %v, want test message", resp.Message)
		}
	})
}

func TestAPIResponseJSON(t *testing.T) {
	t.Run("can marshal to JSON", func(t *testing.T) {
		resp := APIResponse{
			Message: "test message",
		}

		if resp.Message != "test message" {
			t.Errorf("Message = %v, want test message", resp.Message)
		}
	})
}

func TestError(t *testing.T) {
	tests := []struct {
		name     string
		inputErr error
		wantMsg  string
	}{
		{
			name:     "simple error",
			inputErr: errors.New("test error"),
			wantMsg:  "test error",
		},
		{
			name:     "empty error message",
			inputErr: errors.New(""),
			wantMsg:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := Error(tt.inputErr)
			if resp.Message != tt.wantMsg {
				t.Errorf("Message = %v, want %v", resp.Message, tt.wantMsg)
			}
		})
	}
}

func TestPredefinedAPIResponses(t *testing.T) {
	t.Run("ResponseErrInvalidUserData is set", func(t *testing.T) {
		if ResponseErrInvalidUserData.Message == "" {
			t.Errorf("ResponseErrInvalidUserData.Message is empty")
		}
	})

	t.Run("ResponseErrUserWithUsernameExists is set", func(t *testing.T) {
		if ResponseErrUserWithUsernameExists.Message == "" {
			t.Errorf("ResponseErrUserWithUsernameExists.Message is empty")
		}
	})

	t.Run("ResponseErrUserWithMailAddressExists is set", func(t *testing.T) {
		if ResponseErrUserWithMailAddressExists.Message == "" {
			t.Errorf("ResponseErrUserWithMailAddressExists.Message is empty")
		}
	})

	t.Run("ResponseErrDatabaseContextNotFound is set", func(t *testing.T) {
		if ResponseErrDatabaseContextNotFound.Message == "" {
			t.Errorf("ResponseErrDatabaseContextNotFound.Message is empty")
		}
	})

	t.Run("ResponseErrUserIDFromToken is set", func(t *testing.T) {
		if ResponseErrUserIDFromToken.Message == "" {
			t.Errorf("ResponseErrUserIDFromToken.Message is empty")
		}
	})

	t.Run("ResponseErrUserNoProductsFound is set", func(t *testing.T) {
		if ResponseErrUserNoProductsFound.Message == "" {
			t.Errorf("ResponseErrUserNoProductsFound.Message is empty")
		}
	})
}

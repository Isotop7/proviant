package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResponseHelpers(t *testing.T) {
	tests := []struct {
		name        string
		resp        APIResponse
		wantMessage string
		wantAction  string
	}{
		{
			name:        "InternalError",
			resp:        InternalError(),
			wantMessage: "An error occurred. Please try again or contact support if the problem persists.",
			wantAction:  ActionTryAgain,
		},
		{
			name:        "InvalidInputError",
			resp:        InvalidInputError(),
			wantMessage: "The submitted data is invalid. Please check your input and try again.",
			wantAction:  "Please verify your input and try again",
		},
		{
			name:        "CreateFailedError",
			resp:        CreateFailedError(),
			wantMessage: "Failed to save your data. Please try again.",
			wantAction:  ActionTryAgain,
		},
		{
			name:        "UpdateFailedError",
			resp:        UpdateFailedError(),
			wantMessage: "Failed to update. Please try again.",
			wantAction:  ActionTryAgain,
		},
		{
			name:        "DeleteFailedError",
			resp:        DeleteFailedError(),
			wantMessage: "Failed to delete. Please try again.",
			wantAction:  ActionTryAgain,
		},
		{
			name:        "RestoreFailedError",
			resp:        RestoreFailedError(),
			wantMessage: "Failed to restore. Please try again.",
			wantAction:  ActionTryAgain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.resp.Message != tt.wantMessage {
				t.Errorf("Message = %v, want %v", tt.resp.Message, tt.wantMessage)
			}
			if tt.resp.Action != tt.wantAction {
				t.Errorf("Action = %v, want %v", tt.resp.Action, tt.wantAction)
			}
		})
	}
}

func TestInvalidInputErrorWithDetail(t *testing.T) {
	t.Run("includes detail in message", func(t *testing.T) {
		resp := InvalidInputErrorWithDetail("field 'barcode' is required")
		want := "The submitted data is invalid: field 'barcode' is required. Please check your input and try again."
		if resp.Message != want {
			t.Errorf("Message = %v, want %v", resp.Message, want)
		}
		if resp.Action != "Please verify your input and try again" {
			t.Errorf("Action = %v, want %v", resp.Action, "Please verify your input and try again")
		}
	})

	t.Run("empty detail", func(t *testing.T) {
		resp := InvalidInputErrorWithDetail("")
		want := "The submitted data is invalid: . Please check your input and try again."
		if resp.Message != want {
			t.Errorf("Message = %v, want %v", resp.Message, want)
		}
	})
}

func TestRespondError(t *testing.T) {
	tests := []struct {
		name   string
		status int
		err    error
	}{
		{name: "bad request", status: http.StatusBadRequest, err: errors.New("bad input")},
		{name: "unauthorized", status: http.StatusUnauthorized, err: errors.New("invalid token")},
		{name: "internal error", status: http.StatusInternalServerError, err: errors.New("db down")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)

			RespondError(ctx, tt.status, tt.err)

			if w.Code != tt.status {
				t.Errorf("Status = %v, want %v", w.Code, tt.status)
			}
			var response APIResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if response.Message != tt.err.Error() {
				t.Errorf("message = %v, want %v", response.Message, tt.err.Error())
			}
		})
	}
}

func TestBulkActionResponseJSON(t *testing.T) {
	t.Run("failed ids serialized", func(t *testing.T) {
		resp := BulkActionResponse{
			APIResponse: APIResponse{Message: "some failed"},
			FailedIDs:   []uint{1, 2, 3},
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if decoded["message"] != "some failed" {
			t.Errorf("message = %v, want %v", decoded["message"], "some failed")
		}
		failedIDs, ok := decoded["failedIds"].([]any)
		if !ok || len(failedIDs) != 3 {
			t.Errorf("failedIds = %v, want 3 entries", decoded["failedIds"])
		}
	})

	t.Run("failed ids omitted when empty", func(t *testing.T) {
		resp := BulkActionResponse{APIResponse: APIResponse{Message: "ok"}}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if _, exists := decoded["failedIds"]; exists {
			t.Errorf("failedIds should be omitted, got %v", decoded["failedIds"])
		}
	})
}

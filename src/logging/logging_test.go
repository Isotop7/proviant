package logging

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// TestZerologAdapter_Struct tests the ZerologAdapter struct
func TestZerologAdapter_Struct(t *testing.T) {
	t.Run("can create ZerologAdapter", func(t *testing.T) {
		logger := zerolog.New(io.Discard)
		adapter := ZerologAdapter{
			LoggingSink: &logger,
		}

		if adapter.LoggingSink == nil {
			t.Errorf("LoggingSink = %v, want non-nil", adapter.LoggingSink)
		}
	})
}

// TestZerologAdapter_LogMode tests the LogMode method

// TestZerologAdapter_Info tests the Info method
func TestZerologAdapter_Info(t *testing.T) {
	t.Run("Info logs message correctly", func(t *testing.T) {
		// Create a test logger that writes to a buffer
		var logBuffer bytes.Buffer
		logger := zerolog.New(&logBuffer)
		adapter := ZerologAdapter{
			LoggingSink: &logger,
		}

		// Call Info method
		ctx := context.Background()
		adapter.Info(ctx, "test message %s", "with param")

		// Verify the log was written
		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "[GORM]") {
			t.Errorf("Log output = %v, want to contain [GORM]", logOutput)
		}
		if !strings.Contains(logOutput, "test message with param") {
			t.Errorf("Log output = %v, want to contain test message with param", logOutput)
		}
	})
}

// TestZerologAdapter_Warn tests the Warn method
func TestZerologAdapter_Warn(t *testing.T) {
	t.Run("Warn logs message correctly", func(t *testing.T) {
		// Create a test logger that writes to a buffer
		var logBuffer bytes.Buffer
		logger := zerolog.New(&logBuffer)
		adapter := ZerologAdapter{
			LoggingSink: &logger,
		}

		// Call Warn method
		ctx := context.Background()
		adapter.Warn(ctx, "test warning %s", "with param")

		// Verify the log was written
		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "[GORM]") {
			t.Errorf("Log output = %v, want to contain [GORM]", logOutput)
		}
		if !strings.Contains(logOutput, "test warning with param") {
			t.Errorf("Log output = %v, want to contain test warning with param", logOutput)
		}
	})
}

// TestZerologAdapter_Error tests the Error method
func TestZerologAdapter_Error(t *testing.T) {
	t.Run("Error logs message correctly", func(t *testing.T) {
		// Create a test logger that writes to a buffer
		var logBuffer bytes.Buffer
		logger := zerolog.New(&logBuffer)
		adapter := ZerologAdapter{
			LoggingSink: &logger,
		}

		// Call Error method
		ctx := context.Background()
		adapter.Error(ctx, "test error %s", "with param")

		// Verify the log was written
		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "[GORM]") {
			t.Errorf("Log output = %v, want to contain [GORM]", logOutput)
		}
		if !strings.Contains(logOutput, "test error with param") {
			t.Errorf("Log output = %v, want to contain test error with param", logOutput)
		}
	})
}

// TestZerologAdapter_Trace tests the Trace method
func TestZerologAdapter_Trace(t *testing.T) {
	t.Run("Trace logs SQL query correctly", func(t *testing.T) {
		// Create a test logger that writes to a buffer
		var logBuffer bytes.Buffer
		logger := zerolog.New(&logBuffer)
		adapter := ZerologAdapter{
			LoggingSink: &logger,
		}

		// Call Trace method
		ctx := context.Background()
		beginTime := time.Now()
		adapter.Trace(ctx, beginTime, func() (string, int64) {
			return "SELECT * FROM users", 5
		}, nil)

		// Verify the log was written
		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "[GORM] TRACE") {
			t.Errorf("Log output = %v, want to contain [GORM] TRACE", logOutput)
		}
		if !strings.Contains(logOutput, "SELECT * FROM users") {
			t.Errorf("Log output = %v, want to contain SELECT * FROM users", logOutput)
		}
		if !strings.Contains(logOutput, `"rows":5`) {
			t.Errorf("Log output = %v, want to contain rows:5", logOutput)
		}
	})
}

// TestZerologAdapter_WithDifferentLogLevels tests different log levels
func TestZerologAdapter_WithDifferentLogLevels(t *testing.T) {
	t.Run("Info level logs correctly", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := zerolog.New(&logBuffer)
		adapter := ZerologAdapter{
			LoggingSink: &logger,
		}

		ctx := context.Background()
		adapter.Info(ctx, "info message")
		adapter.Warn(ctx, "warn message")
		adapter.Error(ctx, "error message")

		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "info message") {
			t.Errorf("Log output = %v, want to contain info message", logOutput)
		}
		if !strings.Contains(logOutput, "warn message") {
			t.Errorf("Log output = %v, want to contain warn message", logOutput)
		}
		if !strings.Contains(logOutput, "error message") {
			t.Errorf("Log output = %v, want to contain error message", logOutput)
		}
	})
}

// TestZerologAdapter_WithError tests Trace method with error
func TestZerologAdapter_WithError(t *testing.T) {
	t.Run("Trace logs error correctly", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := zerolog.New(&logBuffer)
		adapter := ZerologAdapter{
			LoggingSink: &logger,
		}

		ctx := context.Background()
		beginTime := time.Now()
		testErr := errors.New("test error")
		adapter.Trace(ctx, beginTime, func() (string, int64) {
			return "SELECT * FROM users WHERE id = ?", 0
		}, testErr)

		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "[GORM] TRACE") {
			t.Errorf("Log output = %v, want to contain [GORM] TRACE", logOutput)
		}
		if !strings.Contains(logOutput, "SELECT * FROM users WHERE id = ?") {
			t.Errorf("Log output = %v, want to contain SELECT * FROM users WHERE id = ?", logOutput)
		}
	})
}

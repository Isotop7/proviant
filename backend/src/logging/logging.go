// Provides custom logging facilites
package logging

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm/logger"
)

// ZerologAdapter is a custom GORM logger that logs messages using zerolog.
type ZerologAdapter struct {
	LoggingSink *zerolog.Logger
}

// LogMode sets the log mode for the logger.
func (l ZerologAdapter) LogMode(level logger.LogLevel) logger.Interface {
	return l
}

// Info logs an info message.
func (l ZerologAdapter) Info(ctx context.Context, msg string, data ...interface{}) {
	l.LoggingSink.Info().Msgf("[GORM] %s", fmt.Sprintf(msg, data...))
}

// Warn logs a warning message.
func (l ZerologAdapter) Warn(ctx context.Context, msg string, data ...interface{}) {
	l.LoggingSink.Warn().Msgf("[GORM] %s", fmt.Sprintf(msg, data...))
}

// Error logs an error message.
func (l ZerologAdapter) Error(ctx context.Context, msg string, data ...interface{}) {
	l.LoggingSink.Error().Msgf("[GORM] %s", fmt.Sprintf(msg, data...))
}

// Trace logs a trace message.
func (l ZerologAdapter) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, rows := fc()
	l.LoggingSink.Trace().Str("sql", sql).Int64("rows", rows).Msg("[GORM] TRACE")
}

package observability

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap logger with structured logging for reservation worker
type Logger struct {
	*zap.Logger
}

// NewLogger creates a new structured logger
func NewLogger(level string) (*Logger, error) {
	config := zap.NewProductionConfig()

	// Set log level
	switch level {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "info":
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "warn":
		config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		config.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	// JSON output for structured logging
	config.Encoding = "json"
	config.EncoderConfig.TimeKey = "ts"
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.MessageKey = "msg"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{Logger: logger}, nil
}

// WithEvent adds event-specific fields to logger
func (l *Logger) WithEvent(eventType, reservationID, eventID string) *zap.Logger {
	return l.With(
		zap.String("event_type", eventType),
		zap.String("reservation_id", reservationID),
		zap.String("event_id", eventID),
	)
}

// WithTrace adds trace ID to logger
func (l *Logger) WithTrace(traceID string) *zap.Logger {
	return l.With(zap.String("trace_id", traceID))
}

// WithAttempt adds retry attempt information
func (l *Logger) WithAttempt(attempt int) *zap.Logger {
	return l.With(zap.Int("attempt", attempt))
}

// WithOutcome adds processing outcome
func (l *Logger) WithOutcome(outcome string) *zap.Logger {
	return l.With(zap.String("outcome", outcome))
}

// WithLatency adds processing latency in milliseconds
func (l *Logger) WithLatency(latencyMS int64) *zap.Logger {
	return l.With(zap.Int64("latency_ms", latencyMS))
}
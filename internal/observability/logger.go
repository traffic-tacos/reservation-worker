package observability

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap logger with context support
type Logger struct {
	*zap.Logger
}

// NewLogger creates a new structured logger
func NewLogger(level string) (*Logger, error) {
	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapLevel),
		Development: false,
		Encoding:    "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := config.Build(
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return nil, err
	}

	return &Logger{logger}, nil
}

// WithContext returns a logger with context fields
func (l *Logger) WithContext(ctx context.Context) *Logger {
	logger := l.Logger

	// Add trace ID if available
	if span := GetSpanFromContext(ctx); span != nil {
		// For now, we can't easily extract trace ID from interface{}
		// This would need proper OpenTelemetry span handling
		logger = logger.With(zap.String("trace_id", "unknown"))
	}

	// Add other context fields as needed
	if podName := getPodNameFromContext(ctx); podName != "" {
		logger = logger.With(zap.String("pod_name", podName))
	}

	return &Logger{logger}
}

// EventLog creates a structured log entry for event processing
func (l *Logger) EventLog(ctx context.Context, eventType, reservationID string, fields map[string]interface{}) *Logger {
	logger := l.WithContext(ctx).Logger.With(
		zap.String("event_type", eventType),
		zap.String("reservation_id", reservationID),
	)

	for k, v := range fields {
		logger = logger.With(zap.Any(k, v))
	}

	return &Logger{logger}
}

// ErrorLog creates a structured error log entry
func (l *Logger) ErrorLog(ctx context.Context, err error, fields map[string]interface{}) *Logger {
	logger := l.WithContext(ctx).Logger.With(zap.Error(err))

	for k, v := range fields {
		logger = logger.With(zap.Any(k, v))
	}

	return &Logger{logger}
}

// getPodNameFromContext extracts pod name from context (placeholder for future implementation)
func getPodNameFromContext(ctx context.Context) string {
	// In Kubernetes, this could be extracted from downward API or env var
	// For now, return empty string
	return ""
}

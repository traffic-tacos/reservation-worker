package retry

import (
	"context"
	"fmt"
	"time"

	"github.com/traffic-tacos/reservation-worker/internal/config"
	"go.uber.org/zap"
)

// RetryableFunc is a function that can be retried
type RetryableFunc func(ctx context.Context) error

// Retryer handles retry logic with exponential backoff
type Retryer struct {
	config *config.Config
	logger *zap.Logger
}

// NewRetryer creates a new retryer
func NewRetryer(cfg *config.Config, logger *zap.Logger) *Retryer {
	return &Retryer{
		config: cfg,
		logger: logger,
	}
}

// Do executes the function with retry logic
func (r *Retryer) Do(ctx context.Context, operation string, fn RetryableFunc) error {
	var lastErr error

	for attempt := 0; attempt < r.config.MaxRetries; attempt++ {
		// Check context before attempt
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute the function
		err := fn(ctx)
		if err == nil {
			if attempt > 0 {
				r.logger.Info("Operation succeeded after retry",
					zap.String("operation", operation),
					zap.Int("attempt", attempt+1),
				)
			}
			return nil
		}

		lastErr = err

		// Don't retry on last attempt
		if attempt == r.config.MaxRetries-1 {
			break
		}

		// Calculate backoff duration
		backoff := r.config.GetBackoffDuration(attempt)

		r.logger.Warn("Operation failed, retrying",
			zap.String("operation", operation),
			zap.Error(err),
			zap.Int("attempt", attempt+1),
			zap.Int("max_retries", r.config.MaxRetries),
			zap.Duration("backoff", backoff),
		)

		// Wait with backoff
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	return fmt.Errorf("operation %s failed after %d attempts: %w", operation, r.config.MaxRetries, lastErr)
}

// DoWithResult executes a function that returns a value with retry logic
func DoWithResult[T any](ctx context.Context, cfg *config.Config, logger *zap.Logger, operation string, fn func(ctx context.Context) (T, error)) (T, error) {
	var result T
	var lastErr error

	for attempt := 0; attempt < cfg.MaxRetries; attempt++ {
		// Check context before attempt
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		// Execute the function
		res, err := fn(ctx)
		if err == nil {
			if attempt > 0 {
				logger.Info("Operation succeeded after retry",
					zap.String("operation", operation),
					zap.Int("attempt", attempt+1),
				)
			}
			return res, nil
		}

		lastErr = err

		// Don't retry on last attempt
		if attempt == cfg.MaxRetries-1 {
			break
		}

		// Calculate backoff duration
		backoff := cfg.GetBackoffDuration(attempt)

		logger.Warn("Operation failed, retrying",
			zap.String("operation", operation),
			zap.Error(err),
			zap.Int("attempt", attempt+1),
			zap.Int("max_retries", cfg.MaxRetries),
			zap.Duration("backoff", backoff),
		)

		// Wait with backoff
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	return result, fmt.Errorf("operation %s failed after %d attempts: %w", operation, cfg.MaxRetries, lastErr)
}

// IsRetryable determines if an error should be retried
func IsRetryable(err error) bool {
	// Add logic to determine if error is retryable
	// For now, we'll retry all errors except context cancellation
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	return true
}
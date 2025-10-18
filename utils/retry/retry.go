package retry

import (
	"context"
	"math"
	"time"
)

// Config holds the configuration for retry logic
type Config struct {
	MaxRetries      int
	BaseDelay       time.Duration
	MaxDelay        time.Duration
	BackoffMultiple float64
}

// DefaultConfig returns a sensible default retry configuration
func DefaultConfig() Config {
	return Config{
		MaxRetries:      3,
		BaseDelay:       200 * time.Millisecond,
		MaxDelay:        5 * time.Second,
		BackoffMultiple: 2.0,
	}
}

// ErrorChecker defines a function that determines if an error should trigger a retry
type ErrorChecker func(err error, statusCode int, responseBody []byte) bool

// RetryableFunc defines a function that can be retried
type RetryableFunc func(attempt int) (result interface{}, statusCode int, responseBody []byte, err error)

// Logger defines a function for logging retry attempts
type Logger func(message string, args ...interface{})

// Options configures retry behavior
type Options struct {
	Config       Config
	ErrorChecker ErrorChecker
	Logger       Logger
	APIName      string
}

// calculateDelay computes the delay for the given attempt using exponential backoff
func (c Config) calculateDelay(attempt int) time.Duration {
	delay := time.Duration(float64(c.BaseDelay) * math.Pow(c.BackoffMultiple, float64(attempt)))
	if delay > c.MaxDelay {
		delay = c.MaxDelay
	}
	return delay
}

// Execute performs the retryable function with the configured retry logic
func Execute(ctx context.Context, opts Options, fn RetryableFunc) (interface{}, error) {
	var lastErr error
	var lastStatusCode int
	var lastResponseBody []byte

	for attempt := 0; attempt <= opts.Config.MaxRetries; attempt++ {
		// Add delay before retry (but not on first attempt)
		if attempt > 0 {
			delay := opts.Config.calculateDelay(attempt - 1)
			if opts.Logger != nil {
				opts.Logger("%s API retry attempt %d/%d after %v delay", opts.APIName, attempt+1, opts.Config.MaxRetries+1, delay)
			}

			// Check for context cancellation during delay
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		// Execute the function
		result, statusCode, responseBody, err := fn(attempt)
		lastErr = err
		lastStatusCode = statusCode
		lastResponseBody = responseBody

		// Check if this is a retryable error
		if opts.ErrorChecker != nil && opts.ErrorChecker(err, statusCode, responseBody) && attempt < opts.Config.MaxRetries {
			if opts.Logger != nil {
				if err != nil {
					opts.Logger("%s API network error (attempt %d/%d): %v", opts.APIName, attempt+1, opts.Config.MaxRetries+1, err)
				} else {
					opts.Logger("%s API retryable error (attempt %d/%d): status %d", opts.APIName, attempt+1, opts.Config.MaxRetries+1, statusCode)
				}
			}
			continue
		}

		// If no error or non-retryable error, return the result
		if err == nil {
			if attempt > 0 && opts.Logger != nil {
				opts.Logger("%s API request succeeded on attempt %d/%d", opts.APIName, attempt+1, opts.Config.MaxRetries+1)
			}
			return result, nil
		}

		// Non-retryable error, return immediately
		return nil, err
	}

	// All retries exhausted
	if lastErr != nil {
		return nil, lastErr
	}

	// This shouldn't happen, but return a generic error if it does
	return nil, &RetryExhaustedError{
		APIName:        opts.APIName,
		MaxAttempts:    opts.Config.MaxRetries + 1,
		LastStatusCode: lastStatusCode,
		LastResponse:   lastResponseBody,
	}
}

// RetryExhaustedError represents an error when all retry attempts have been exhausted
type RetryExhaustedError struct {
	APIName        string
	MaxAttempts    int
	LastStatusCode int
	LastResponse   []byte
}

func (e *RetryExhaustedError) Error() string {
	return "retry attempts exhausted for " + e.APIName + " API"
}

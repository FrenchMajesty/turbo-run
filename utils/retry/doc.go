// Package retry provides a flexible and configurable retry mechanism with exponential backoff
// for handling transient failures in network requests and other operations.
//
// The package supports:
//   - Configurable retry attempts with exponential backoff
//   - Custom error checking to determine if an error is retryable
//   - Context-aware execution with cancellation support
//   - Optional logging of retry attempts
//   - Maximum delay caps to prevent excessive wait times
//
// Basic Usage:
//
//	ctx := context.Background()
//	opts := retry.Options{
//	    Config: retry.DefaultConfig(),
//	    ErrorChecker: func(err error, statusCode int, responseBody []byte) bool {
//	        // Return true if the error should trigger a retry
//	        return statusCode == 429 || statusCode >= 500
//	    },
//	    APIName: "MyAPI",
//	}
//
//	result, err := retry.Execute(ctx, opts, func(attempt int) (interface{}, int, []byte, error) {
//	    // Your retryable operation here
//	    resp, err := makeAPICall()
//	    return resp, statusCode, responseBody, err
//	})
//
// Configuration:
//
// The Config struct allows fine-tuning of retry behavior:
//   - MaxRetries: Maximum number of retry attempts (default: 3)
//   - BaseDelay: Initial delay before first retry (default: 200ms)
//   - MaxDelay: Maximum delay between retries (default: 5s)
//   - BackoffMultiple: Multiplier for exponential backoff (default: 2.0)
//
// The exponential backoff calculation is: delay = BaseDelay * (BackoffMultiple ^ attempt)
// capped at MaxDelay.
//
// Error Checking:
//
// The ErrorChecker function determines whether an error should trigger a retry.
// It receives the error, HTTP status code, and response body, allowing for
// sophisticated retry logic based on various failure conditions.
//
// Context Support:
//
// The Execute function respects context cancellation and will immediately return
// if the context is cancelled during execution or between retry attempts.
package retry

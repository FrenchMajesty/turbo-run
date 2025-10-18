package parallel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParallelBuilder_BasicUsage(t *testing.T) {
	ctx := context.Background()

	// Define functions with different return types
	stringFetcher := func(ctx context.Context) (string, error) {
		return "hello", nil
	}

	intFetcher := func(ctx context.Context) (int, error) {
		return 42, nil
	}

	// Execute in parallel
	results := NewBuilder().
		Add("string", func(ctx context.Context) (any, error) { return stringFetcher(ctx) }).
		Add("int", func(ctx context.Context) (any, error) { return intFetcher(ctx) }).
		Run(ctx)

	// Get typed results
	stringResult, err := Get(results, "string", stringFetcher)
	assert.NoError(t, err)
	assert.Equal(t, "hello", stringResult)

	intResult, err := Get(results, "int", intFetcher)
	assert.NoError(t, err)
	assert.Equal(t, 42, intResult)
}

func TestParallelBuilder_WithErrors(t *testing.T) {
	ctx := context.Background()

	successFetcher := func(ctx context.Context) (string, error) {
		return "success", nil
	}

	errorFetcher := func(ctx context.Context) (string, error) {
		return "", errors.New("test error")
	}

	// Execute in parallel
	results := NewBuilder().
		Add("success", func(ctx context.Context) (any, error) { return successFetcher(ctx) }).
		Add("error", func(ctx context.Context) (any, error) { return errorFetcher(ctx) }).
		Run(ctx)

	// Get success result
	successResult, err := Get(results, "success", successFetcher)
	assert.NoError(t, err)
	assert.Equal(t, "success", successResult)

	// Get error result
	errorResult, err := Get(results, "error", errorFetcher)
	assert.Error(t, err)
	assert.Equal(t, "", errorResult)
	assert.Contains(t, err.Error(), "test error")
}

func TestParallelBuilder_Concurrency(t *testing.T) {
	ctx := context.Background()
	startTime := time.Now()

	slowFetcher1 := func(ctx context.Context) (string, error) {
		time.Sleep(100 * time.Millisecond)
		return "slow1", nil
	}

	slowFetcher2 := func(ctx context.Context) (string, error) {
		time.Sleep(100 * time.Millisecond)
		return "slow2", nil
	}

	// Execute in parallel
	results := NewBuilder().
		Add("slow1", func(ctx context.Context) (any, error) { return slowFetcher1(ctx) }).
		Add("slow2", func(ctx context.Context) (any, error) { return slowFetcher2(ctx) }).
		Run(ctx)

	duration := time.Since(startTime)

	// Should complete in roughly 100ms (parallel), not 200ms (sequential)
	assert.Less(t, duration, 150*time.Millisecond)

	// Verify results
	result1, err := Get(results, "slow1", slowFetcher1)
	assert.NoError(t, err)
	assert.Equal(t, "slow1", result1)

	result2, err := Get(results, "slow2", slowFetcher2)
	assert.NoError(t, err)
	assert.Equal(t, "slow2", result2)
}

func TestGet_KeyNotFound(t *testing.T) {
	results := Results{}
	
	fetcher := func(ctx context.Context) (string, error) {
		return "test", nil
	}

	result, err := Get(results, "nonexistent", fetcher)
	assert.Error(t, err)
	assert.Equal(t, "", result)
	assert.Contains(t, err.Error(), "no result found for key: nonexistent")
}

func TestGet_TypeAssertionFailure(t *testing.T) {
	results := Results{
		"key": Result{
			Value: 42, // int instead of string
			Error: nil,
		},
	}
	
	fetcher := func(ctx context.Context) (string, error) {
		return "test", nil
	}

	result, err := Get(results, "key", fetcher)
	assert.Error(t, err)
	assert.Equal(t, "", result)
	assert.Contains(t, err.Error(), "type assertion failed")
}

func TestParallelBuilder_EmptyBuilder(t *testing.T) {
	ctx := context.Background()
	
	results := NewBuilder().Run(ctx)
	
	assert.Equal(t, 0, len(results))
}
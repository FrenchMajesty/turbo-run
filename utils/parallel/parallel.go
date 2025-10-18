package parallel

import (
	"context"
	"fmt"
	"sync"
)

// Task represents a function to be executed in parallel
type Task func(ctx context.Context) (any, error)

// Result holds the result and error from a parallel task execution
type Result struct {
	Value any
	Error error
}

// Results holds the map of results from parallel execution
type Results map[string]Result

// Builder manages parallel task execution with type-safe retrieval
type Builder struct {
	tasks map[string]Task
}

// NewBuilder creates a new parallel builder
func NewBuilder() *Builder {
	return &Builder{
		tasks: make(map[string]Task),
	}
}

// Add adds a keyed task to be executed in parallel
func (b *Builder) Add(key string, task Task) *Builder {
	b.tasks[key] = task
	return b
}

// Run executes all tasks in parallel and returns results keyed by their original keys
func (b *Builder) Run(ctx context.Context) Results {
	if len(b.tasks) == 0 {
		return Results{}
	}

	results := make(Results)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for key, task := range b.tasks {
		wg.Add(1)
		go func(k string, t Task) {
			defer wg.Done()
			value, err := t(ctx)
			
			mu.Lock()
			results[k] = Result{Value: value, Error: err}
			mu.Unlock()
		}(key, task)
	}

	wg.Wait()
	return results
}

// Get retrieves a typed result using the function signature to infer the return type
func Get[T any](results Results, key string, fn func(ctx context.Context) (T, error)) (T, error) {
	result, exists := results[key]
	if !exists {
		var zero T
		return zero, fmt.Errorf("no result found for key: %s", key)
	}
	
	if result.Error != nil {
		var zero T
		return zero, result.Error
	}
	
	// Type assert to the inferred type from the function signature
	value, ok := result.Value.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("type assertion failed for key %s: expected %T, got %T", key, zero, result.Value)
	}
	
	return value, nil
}
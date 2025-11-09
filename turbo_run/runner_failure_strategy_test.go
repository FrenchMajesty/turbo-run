package turbo_run

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/FrenchMajesty/turbo-run/clients/groq"
	"github.com/google/uuid"
	openai "github.com/openai/openai-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestFailureStrategyPropagate tests that failures cascade to dependent children with Propagate strategy
func TestFailureStrategyPropagate(t *testing.T) {
	// Reset singleton
	instance = nil
	once = sync.Once{}

	mockGroq := NewTestableGroqClient()
	mockOpenAI := &openai.Client{}

	// Mock to return errors for all requests
	mockGroq.On("ChatCompletion", mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("simulated API error"),
	)

	turboRun := NewTurboRun(Options{
		GroqClient:              mockGroq,
		OpenAIClient:            mockOpenAI,
		failureHandlingStrategy: FailureStrategyPropagate, // Failures cascade
	})
	defer turboRun.Stop()

	// Create a chain: node1 -> node2 -> node3
	node1 := NewWorkNodeForGroq(groq.ChatCompletionRequest{
		Model: "llama3-8b-8192",
		Messages: []groq.ChatMessage{
			{Role: groq.MessageRoleUser, Content: stringPtr("test1")},
		},
	})

	node2 := NewWorkNodeForGroq(groq.ChatCompletionRequest{
		Model: "llama3-8b-8192",
		Messages: []groq.ChatMessage{
			{Role: groq.MessageRoleUser, Content: stringPtr("test2")},
		},
	})

	node3 := NewWorkNodeForGroq(groq.ChatCompletionRequest{
		Model: "llama3-8b-8192",
		Messages: []groq.ChatMessage{
			{Role: groq.MessageRoleUser, Content: stringPtr("test3")},
		},
	})

	// Push with dependencies: node1 fails -> should cancel node2 and node3
	turboRun.Push(node1)
	turboRun.PushWithDependencies(node2, []uuid.UUID{node1.ID})
	turboRun.PushWithDependencies(node3, []uuid.UUID{node2.ID})

	// Wait for node1 to fail
	result1 := turboRun.WaitFor(node1)
	assert.Error(t, result1.Error, "Node1 should fail")

	// Give time for cascade to happen
	time.Sleep(100 * time.Millisecond)

	// Verify that all nodes were removed from graph (propagation happened)
	stats := turboRun.GetStats()
	assert.Equal(t, 0, stats.GraphSize, "Graph should be empty after propagation")
	assert.Equal(t, 1, stats.LaunchedCount, "Only node1 should have been launched")
	assert.Equal(t, 3, stats.FailedCount, "All 3 nodes should be counted as failed (1 actual + 2 cancelled)")
}

// TestFailureStrategyIsolate tests that failures don't cascade to dependent children with Isolate strategy
func TestFailureStrategyIsolate(t *testing.T) {
	// Reset singleton
	instance = nil
	once = sync.Once{}

	mockGroq := NewTestableGroqClient()
	mockOpenAI := &openai.Client{}

	// First call fails, subsequent calls succeed
	mockGroq.On("ChatCompletion", mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("simulated API error"),
	).Once()

	mockGroq.On("ChatCompletion", mock.Anything, mock.Anything).Return(
		&groq.ChatCompletionResponse{}, nil,
	)

	turboRun := NewTurboRun(Options{
		GroqClient:              mockGroq,
		OpenAIClient:            mockOpenAI,
		failureHandlingStrategy: FailureStrategyIsolate, // Failures don't cascade
	})
	defer turboRun.Stop()

	// Create a chain: node1 -> node2 -> node3
	node1 := NewWorkNodeForGroq(groq.ChatCompletionRequest{
		Model: "llama3-8b-8192",
		Messages: []groq.ChatMessage{
			{Role: groq.MessageRoleUser, Content: stringPtr("test1")},
		},
	})

	node2 := NewWorkNodeForGroq(groq.ChatCompletionRequest{
		Model: "llama3-8b-8192",
		Messages: []groq.ChatMessage{
			{Role: groq.MessageRoleUser, Content: stringPtr("test2")},
		},
	})

	node3 := NewWorkNodeForGroq(groq.ChatCompletionRequest{
		Model: "llama3-8b-8192",
		Messages: []groq.ChatMessage{
			{Role: groq.MessageRoleUser, Content: stringPtr("test3")},
		},
	})

	// Push with dependencies: node1 fails -> node2 and node3 should still proceed
	turboRun.Push(node1)
	turboRun.PushWithDependencies(node2, []uuid.UUID{node1.ID})
	turboRun.PushWithDependencies(node3, []uuid.UUID{node2.ID})

	// Wait for all nodes to complete
	result1 := turboRun.WaitFor(node1)
	result2 := turboRun.WaitFor(node2)
	result3 := turboRun.WaitFor(node3)

	// Verify results
	assert.Error(t, result1.Error, "Node1 should fail")
	assert.NoError(t, result2.Error, "Node2 should succeed despite parent failure")
	assert.NoError(t, result3.Error, "Node3 should succeed despite grandparent failure")

	// Verify stats
	stats := turboRun.GetStats()
	assert.Equal(t, 0, stats.GraphSize, "Graph should be empty after all nodes complete")
	assert.Equal(t, 3, stats.LaunchedCount, "All 3 nodes should have been launched")
	assert.Equal(t, 1, stats.FailedCount, "Only node1 should be counted as failed")
	assert.Equal(t, 2, stats.CompletedCount, "Node2 and node3 should be counted as completed")
}

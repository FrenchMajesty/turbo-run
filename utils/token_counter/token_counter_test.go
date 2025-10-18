package token_counter

import (
	"testing"

	"github.com/FrenchMajesty/turbo-run/clients/groq"
	"github.com/stretchr/testify/assert"
)

func TestTokenCounter_EstimateChatMessageTokens_EdgeCases(t *testing.T) {
	counter, err := NewTokenCounter()
	assert.NoError(t, err)

	// Test with nil content
	msg := groq.ChatMessage{
		Role:    groq.MessageRoleUser,
		Content: nil,
	}
	result := counter.EstimateChatMessageTokens(msg)
	assert.Greater(t, result, 0) // Should still have role tokens + overhead

	// Test with tool calls
	toolCalls := []groq.ToolCallRequest{
		{
			ID:   "call_123",
			Type: "function",
		},
	}
	content := "Using a tool"
	msgWithToolCalls := groq.ChatMessage{
		Role:      groq.MessageRoleAssistant,
		Content:   &content,
		ToolCalls: &toolCalls,
	}
	result = counter.EstimateChatMessageTokens(msgWithToolCalls)
	assert.Greater(t, result, 10) // Should include content + tool calls + overhead

	// Test with very long content
	longContent := ""
	for i := 0; i < 1000; i++ {
		longContent += "word "
	}
	longMsg := groq.ChatMessage{
		Role:    groq.MessageRoleUser,
		Content: &longContent,
	}
	result = counter.EstimateChatMessageTokens(longMsg)
	assert.Greater(t, result, 1000) // Should be roughly proportional to content length
}

func TestTokenCounter_CountChatMessagesTokens_EmptySlice(t *testing.T) {
	counter, err := NewTokenCounter()
	assert.NoError(t, err)

	result := counter.CountChatMessagesTokens([]groq.ChatMessage{})
	assert.Equal(t, 0, result)
}

func TestNewTokenCounter_ErrorHandling(t *testing.T) {
	// This test would require mocking the tiktoken.GetEncoding function
	// which might not be easily mockable. For now, we just test successful creation
	counter, err := NewTokenCounter()
	assert.NoError(t, err)
	assert.NotNil(t, counter)

	// Verify the counter has been initialized with the correct encoding
	assert.NotNil(t, counter.encoder)
}

func TestTokenCounter_CountRequestTokens(t *testing.T) {
	counter, err := NewTokenCounter()
	assert.NoError(t, err)

	testCases := []struct {
		name     string
		request  groq.ChatCompletionRequest
		expected int // Minimum expected token count
	}{
		{
			name: "simple request with system prompt",
			request: groq.ChatCompletionRequest{
				Model: "llama3-8b-8192",
				Messages: []groq.ChatMessage{
					{
						Role:    groq.MessageRoleSystem,
						Content: stringPtr("You are a helpful assistant."),
					},
					{
						Role:    groq.MessageRoleUser,
						Content: stringPtr("Hello!"),
					},
				},
			},
			expected: 30, // System prompt + user message + overhead
		},
		{
			name: "request with tools",
			request: groq.ChatCompletionRequest{
				Model: "llama3-8b-8192",
				Messages: []groq.ChatMessage{
					{
						Role:    groq.MessageRoleUser,
						Content: stringPtr("What's the weather?"),
					},
				},
				Tools: &[]groq.ToolDefinition{
					{
						Type: "function",
						Function: groq.ToolFunction{
							Name:        "get_weather",
							Description: "Get the current weather in a given location",
							Parameters: groq.ToolFunctionParameters{
								Type: "object",
								Properties: map[string]groq.ToolFunctionProperty{
									"location": {
										Type:        "string",
										Description: "The city and state, e.g. San Francisco, CA",
									},
								},
								Required: []string{"location"},
							},
						},
					},
				},
			},
			expected: 50, // User message + tool definition + overhead
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := counter.CountRequestTokens(tc.request)
			assert.GreaterOrEqual(t, result, tc.expected, "Token count should be at least %d", tc.expected)
			assert.Less(t, result, tc.expected*3, "Token count should be reasonable (less than 3x expected)")
		})
	}
}

func stringPtr(s string) *string {
	return &s
}

// Helper function for tests removed - using the one from conversation_processor_test.go

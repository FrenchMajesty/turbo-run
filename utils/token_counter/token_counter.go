package token_counter

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/FrenchMajesty/turbo-run/clients/groq"
	"github.com/pkoukk/tiktoken-go"
)

// tokenCounterImpl provides utilities for counting tokens in conversations
type tokenCounterImpl struct {
	encoder *tiktoken.Tiktoken
}

var encodingBase = "cl100k_base"

// NewTokenCounter creates a new TokenCounter instance
func NewTokenCounter() (*tokenCounterImpl, error) {
	// Use cl100k_base encoding (used by GPT-4, GPT-3.5-turbo, and text-embedding-ada-002)
	encoder, err := tiktoken.GetEncoding(encodingBase)
	if err != nil {
		return nil, fmt.Errorf("failed to get tiktoken encoding: %w", err)
	}

	return &tokenCounterImpl{
		encoder: encoder,
	}, nil
}

// CountChatMessagesTokens estimates token count for groq.ChatMessage slice
func (tc *tokenCounterImpl) CountChatMessagesTokens(messages []groq.ChatMessage) int {
	totalTokens := 0
	for _, msg := range messages {
		totalTokens += tc.EstimateChatMessageTokens(msg)
	}
	return totalTokens
}

// EstimateChatMessageTokens estimates tokens for a groq.ChatMessage using tiktoken
func (tc *tokenCounterImpl) EstimateChatMessageTokens(msg groq.ChatMessage) int {
	totalTokens := 0

	// Count tokens for role (using role string directly)
	roleTokens := tc.encoder.Encode(string(msg.Role), nil, nil)
	totalTokens += len(roleTokens)

	// Count tokens for content
	if msg.Content != nil {
		contentTokens := tc.encoder.Encode(*msg.Content, nil, nil)
		totalTokens += len(contentTokens)
	}
	// Count tokens for tool calls
	if msg.ToolCalls != nil {
		toolCallsStr := fmt.Sprintf("%v", *msg.ToolCalls)
		toolCallsTokens := tc.encoder.Encode(toolCallsStr, nil, nil)
		totalTokens += len(toolCallsTokens)
	}

	// Add overhead for message structure (based on OpenAI's token counting methodology)
	totalTokens += 4

	return totalTokens
}

// CountTextTokens counts tokens in plain text using tiktoken
func (tc *tokenCounterImpl) CountTextTokens(text string) int {
	tokens := tc.encoder.Encode(text, nil, nil)
	return len(tokens)
}

// GetTokenCountFromUsage extracts token count from LLM response usage
func (tc *tokenCounterImpl) GetTokenCountFromUsage(usage groq.ChatCompletionUsage) (int, int, int) {
	return usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens
}

// CountRequestTokens estimates the total token count for a complete ChatCompletionRequest
// including system prompt, messages, tools, and other metadata
func (tc *tokenCounterImpl) CountRequestTokens(request groq.ChatCompletionRequest) int {
	totalTokens := 0

	bodyString, err := json.Marshal(request)
	if err != nil {
		log.Fatalf("failed to marshal request: %v", err)
	}

	totalTokens += tc.CountTextTokens(string(bodyString))

	return totalTokens
}

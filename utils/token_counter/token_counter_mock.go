package token_counter

import (
	"github.com/FrenchMajesty/turbo-run/clients/groq"
	"github.com/stretchr/testify/mock"
)

// MockTokenCounter is a mock implementation of interfaces.TokenCounterInterface for testing.
type MockTokenCounter struct {
	mock.Mock
}

var _ TokenCounterInterface = (*MockTokenCounter)(nil)

func NewMockTokenCounter() TokenCounterInterface {
	return &MockTokenCounter{}
}

func (m *MockTokenCounter) CountChatMessagesTokens(messages []groq.ChatMessage) int {
	args := m.Called(messages)
	return args.Int(0)
}

func (m *MockTokenCounter) EstimateChatMessageTokens(msg groq.ChatMessage) int {
	args := m.Called(msg)
	return args.Int(0)
}

func (m *MockTokenCounter) CountTextTokens(text string) int {
	args := m.Called(text)
	return args.Int(0)
}

func (m *MockTokenCounter) GetTokenCountFromUsage(usage groq.ChatCompletionUsage) (int, int, int) {
	args := m.Called(usage)
	return args.Int(0), args.Int(1), args.Int(2)
}

func (m *MockTokenCounter) CountRequestTokens(request groq.ChatCompletionRequest) int {
	args := m.Called(request)
	return args.Int(0)
}

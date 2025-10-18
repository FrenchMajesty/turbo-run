package groq

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockGroqClient struct {
	mock.Mock
}

// Ensure MockGroqClient implements GroqClientInterface
var _ GroqClientInterface = (*MockGroqClient)(nil)

func NewMockGroqClient() *MockGroqClient {
	return &MockGroqClient{}
}

func (m *MockGroqClient) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ChatCompletionResponse), args.Error(1)
}

func (m *MockGroqClient) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest, callback func(token string)) (*StreamingResult, error) {
	args := m.Called(ctx, req, callback)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*StreamingResult), args.Error(1)
}

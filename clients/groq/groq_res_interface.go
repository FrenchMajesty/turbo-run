package groq

type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// The response from the chat completion endpoint
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   ChatCompletionUsage    `json:"usage"`
}

type ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatError struct {
	Code             string `json:"code"`
	Message          string `json:"message"`
	Type             string `json:"type"`
	FailedGeneration string `json:"failed_generation,omitempty"`
}

type ChatCompletionResponseError struct {
	Error ChatError `json:"error"`
}

// Streaming response structures for Server-Sent Events
type ChatCompletionStreamChoice struct {
	Index        int                      `json:"index"`
	Delta        ChatCompletionDelta      `json:"delta"`
	FinishReason *string                  `json:"finish_reason"`
}

type ChatCompletionDelta struct {
	Role      *string        `json:"role,omitempty"`
	Content   *string        `json:"content,omitempty"`
	ToolCalls *[]ToolCallStream `json:"tool_calls,omitempty"`
}

type ToolCallStream struct {
	Index    int                    `json:"index"`
	ID       *string               `json:"id,omitempty"`
	Type     *string               `json:"type,omitempty"`
	Function *ToolCallFunctionStream `json:"function,omitempty"`
}

type ToolCallFunctionStream struct {
	Name      *string `json:"name,omitempty"`
	Arguments *string `json:"arguments,omitempty"`
}

// ChatCompletionStreamResponse represents a single streamed chunk
type ChatCompletionStreamResponse struct {
	ID      string                       `json:"id"`
	Object  string                       `json:"object"`
	Choices []ChatCompletionStreamChoice `json:"choices"`
	Usage   *ChatCompletionUsage         `json:"usage,omitempty"`
}

// StreamingResult wraps a chat completion response with streaming metadata
type StreamingResult struct {
	Response         *ChatCompletionResponse `json:"response"`
	TimeToFirstToken *int                    `json:"time_to_first_token_ms,omitempty"`
}

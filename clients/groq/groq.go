package groq

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FrenchMajesty/turbo-run/utils/retry"
	"github.com/google/uuid"
)

const groqBaseURL = "https://api.groq.com/openai/v1"

// ChatCompletionError wraps standard errors with raw response body for error logging
type ChatCompletionError struct {
	Message    string          `json:"message"`
	StatusCode int             `json:"status_code,omitempty"`
	RawBody    json.RawMessage `json:"raw_body,omitempty"`
}

func (e *ChatCompletionError) Error() string {
	return e.Message
}

// GetRawResponseBody returns the raw response body if available
func (e *ChatCompletionError) GetRawResponseBody() json.RawMessage {
	return e.RawBody
}

type ModelName string

var (
	ModelLlama3370bVersatile ModelName = "llama-3.3-70b-versatile"
	ModelDeepseekR1          ModelName = "deepseek-r1-distill-llama-70b"
	ModelQwen332B            ModelName = "qwen/qwen-3.3-2b-instruct"
	ModelOss20B              ModelName = "openai/gpt-oss-20b"
	ModelOss120B             ModelName = "openai/gpt-oss-120b"
)

// GroqClient is a minimal client for the Groq Chat API
type GroqClient struct {
	APIKey      string
	Env         string
	HTTPClient  *http.Client
	RetryConfig retry.Config
	RetryChan   chan int
	verboseLog  bool
	evalMode    bool
}

type GroqClientInterface interface {
	ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error)
	ChatCompletionStream(ctx context.Context, req ChatCompletionRequest, callback func(token string)) (*StreamingResult, error)
}

// Creates a new GroqClient
func NewGroqClient(apiKey string, env string) *GroqClient {
	client := &GroqClient{
		APIKey:      apiKey,
		Env:         env,
		HTTPClient:  http.DefaultClient,
		RetryConfig: retry.DefaultConfig(),
		RetryChan:   make(chan int, 10),
		verboseLog:  true,
		evalMode:    false,
	}

	return client
}

// SetVerboseLog sets the verbose log flag
func (c *GroqClient) SetVerboseLog(verboseLog bool) *GroqClient {
	c.verboseLog = verboseLog
	return c
}

// SetEvalMode sets the eval mode flag
func (c *GroqClient) SetEvalMode(evalMode bool) *GroqClient {
	c.evalMode = evalMode
	return c
}

// isRetryableError determines if an error should trigger a retry
func (c *GroqClient) isRetryableError(err error, statusCode int, responseBody []byte) bool {
	// Retry on network errors
	if err != nil {
		return true
	}

	// Retry on server errors (5xx)
	if statusCode >= 500 {
		return true
	}

	// Retry on rate limiting (429)
	if !c.evalMode && statusCode == 429 {
		return true
	}

	if statusCode == 400 {
		return true
	}

	// Check for failed_generation in response body
	if responseBody != nil {
		var errorResp ChatCompletionResponseError
		if json.Unmarshal(responseBody, &errorResp) == nil {
			if errorResp.Error.FailedGeneration != "" ||
				strings.Contains(errorResp.Error.Message, "failed_generation") {
				return true
			}
		}

		// Also check if the response body contains "failed_generation" string
		if strings.Contains(string(responseBody), "failed_generation") {
			return true
		}

		var successResp ChatCompletionResponse
		if json.Unmarshal(responseBody, &successResp) == nil {
			if len(successResp.Choices) > 0 && (successResp.Choices[0].FinishReason == "stop" || successResp.Choices[0].FinishReason == "length") {
				content := successResp.Choices[0].Message.Content
				if content == nil || *content == "" {
					return true
				}
			}
		}
	}

	return false
}

// Sends a chat completion request to Groq with retry logic
func (c *GroqClient) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	url := groqBaseURL + "/chat/completions"

	// Setup retry options
	opts := retry.Options{
		Config:       c.RetryConfig,
		ErrorChecker: c.isRetryableError,
		APIName:      "Groq",
	}
	if c.verboseLog {
		opts.Logger = log.Printf
	}

	// Define the retryable function
	retryableFn := func(attempt int) (any, int, []byte, error) {
		body, err := json.Marshal(req)
		if err != nil {
			return nil, 0, nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
		if err != nil {
			return nil, 0, nil, fmt.Errorf("failed to create HTTP request: %w", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.HTTPClient.Do(httpReq)
		if err != nil {
			c.pushToRetryChan(attempt)
			return nil, 0, nil, err
		}
		defer resp.Body.Close()

		// Read the response body once
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.pushToRetryChan(attempt)
			return nil, resp.StatusCode, nil, fmt.Errorf("failed to read response body: %w", err)
		}

		// Check if we should dump the request/response
		if os.Getenv("DUMP_LLM_REQUESTS") == "true" {
			saveResponseToFile(req.Model, req, bodyBytes, resp.StatusCode)
		}

		// If we get here and status is not OK, it's an error
		if resp.StatusCode != http.StatusOK {
			c.pushToRetryChan(attempt)
			return nil, resp.StatusCode, bodyBytes, &ChatCompletionError{
				Message:    fmt.Sprintf("groq API error %d", resp.StatusCode),
				StatusCode: resp.StatusCode,
				RawBody:    json.RawMessage(bodyBytes),
			}
		}

		// Try to parse the successful response
		var chatResp ChatCompletionResponse
		if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
			c.pushToRetryChan(attempt)
			return nil, resp.StatusCode, bodyBytes, &ChatCompletionError{
				Message:    fmt.Sprintf("failed to parse response: %v", err),
				StatusCode: resp.StatusCode,
				RawBody:    json.RawMessage(bodyBytes),
			}
		}

		return &chatResp, resp.StatusCode, bodyBytes, nil
	}

	// Execute with retry logic
	result, err := retry.Execute(ctx, opts, retryableFn)
	if err != nil {
		return nil, err
	}

	return result.(*ChatCompletionResponse), nil
}

func saveResponseToFile(model string, req ChatCompletionRequest, bodyBytes []byte, statusCode int) {
	// Create a unique filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	random := uuid.New().String()[:8]
	filename := fmt.Sprintf("groq_req_%s_%s.json", timestamp, random)

	// Create model-specific directory
	modelDir := fmt.Sprintf("llm_requests/%s", model)
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		log.Printf("Error creating directory %s: %v", modelDir, err)
		return
	}

	// Parse response body as JSON
	var responseBody any
	if err := json.Unmarshal(bodyBytes, &responseBody); err != nil {
		log.Printf("Error parsing response body as JSON: %v", err)
		return
	}

	// Create a response object to save
	responseData := map[string]any{
		"request":  req,
		"response": responseBody,
		"status":   statusCode,
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(responseData, "", "  ")
	if err != nil {
		log.Printf("Error marshaling response data: %v", err)
		return
	}

	// Write to file in model-specific directory
	filepath := filepath.Join(modelDir, filename)
	err = os.WriteFile(filepath, jsonData, 0644)
	if err != nil {
		log.Printf("Error writing to file %s: %v", filepath, err)
		return
	}
}

// ChatCompletionStream sends a streaming chat completion request to Groq
func (c *GroqClient) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest, callback func(token string)) (*StreamingResult, error) {
	url := groqBaseURL + "/chat/completions"

	// Ensure stream is enabled
	req.Stream = true

	// Setup retry options
	opts := retry.Options{
		Config:       c.RetryConfig,
		ErrorChecker: c.isRetryableError,
		APIName:      "Groq",
	}
	if c.verboseLog {
		opts.Logger = log.Printf
	}

	var requestStartTime time.Time
	var firstTokenTime *time.Time

	// Define the retryable function
	retryableFn := func(attempt int) (any, int, []byte, error) {
		// Reset timing for each retry attempt
		requestStartTime = time.Now()
		firstTokenTime = nil

		body, err := json.Marshal(req)
		if err != nil {
			return nil, 0, nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
		if err != nil {
			return nil, 0, nil, fmt.Errorf("failed to create HTTP request: %w", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")

		resp, err := c.HTTPClient.Do(httpReq)
		if err != nil {
			return nil, 0, nil, err
		}
		defer resp.Body.Close()

		// If we get here and status is not OK, it's an error
		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return nil, resp.StatusCode, bodyBytes, &ChatCompletionError{
				Message:    fmt.Sprintf("groq API error %d", resp.StatusCode),
				StatusCode: resp.StatusCode,
				RawBody:    json.RawMessage(bodyBytes),
			}
		}

		// Parse the streaming response with callback that tracks first token time
		var wrappedCallback func(string)
		if callback != nil {
			wrappedCallback = func(token string) {
				if firstTokenTime == nil {
					now := time.Now()
					firstTokenTime = &now
				}
				callback(token)
			}
		}

		response, err := c.parseStreamingResponse(ctx, resp.Body, wrappedCallback)
		if err != nil {
			// If it's a context cancellation, don't retry
			if ctx.Err() != nil {
				return nil, resp.StatusCode, nil, ctx.Err()
			}
			return nil, resp.StatusCode, nil, fmt.Errorf("failed to parse streaming response: %w", err)
		}

		// Check if we should dump the request/response for streaming
		if os.Getenv("DUMP_LLM_REQUESTS") == "true" {
			responseJSON, _ := json.Marshal(response)
			saveResponseToFile(req.Model, req, responseJSON, resp.StatusCode)
		}

		// Calculate TTFT if we captured first token time
		var ttftMs *int
		if firstTokenTime != nil {
			ttft := int(firstTokenTime.Sub(requestStartTime).Milliseconds())
			ttftMs = &ttft
		}

		// Create streaming result with metadata
		result := &StreamingResult{
			Response:         response,
			TimeToFirstToken: ttftMs,
		}

		return result, resp.StatusCode, nil, nil
	}

	// Execute with retry logic
	result, err := retry.Execute(ctx, opts, retryableFn)
	if err != nil {
		return nil, err
	}

	return result.(*StreamingResult), nil
}

// parseStreamingResponse parses Server-Sent Events from the response body
func (c *GroqClient) parseStreamingResponse(ctx context.Context, body io.Reader, callback func(token string)) (*ChatCompletionResponse, error) {
	scanner := bufio.NewScanner(body)
	var finalResponse *ChatCompletionResponse
	var fullContent strings.Builder

	// Track tool calls being built from deltas
	toolCallsMap := make(map[int]*ToolCallRequest) // Map by index to accumulate tool calls

	for scanner.Scan() {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse SSE data line
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Check for stream end
			if data == "[DONE]" {
				break
			}

			// Parse JSON chunk
			var chunk ChatCompletionStreamResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				// Skip malformed chunks but continue processing
				continue
			}

			// Process choices and extract content
			if len(chunk.Choices) > 0 {
				choice := chunk.Choices[0]

				// Handle content delta
				if choice.Delta.Content != nil && *choice.Delta.Content != "" {
					token := *choice.Delta.Content
					fullContent.WriteString(token)
					callback(token)
				}

				// Handle tool call deltas
				if choice.Delta.ToolCalls != nil {
					for _, toolCallDelta := range *choice.Delta.ToolCalls {
						index := toolCallDelta.Index

						// Initialize tool call if this is the first delta for this index
						if toolCallsMap[index] == nil {
							toolCallsMap[index] = &ToolCallRequest{
								Type: "function",
								Function: ToolCallFunction{
									Arguments: "",
								},
							}
						}

						toolCall := toolCallsMap[index]

						// Accumulate tool call ID
						if toolCallDelta.ID != nil {
							toolCall.ID = *toolCallDelta.ID
						}

						// Accumulate tool call type
						if toolCallDelta.Type != nil {
							toolCall.Type = *toolCallDelta.Type
						}

						// Accumulate function name and arguments
						if toolCallDelta.Function != nil {
							if toolCallDelta.Function.Name != nil {
								toolCall.Function.Name = *toolCallDelta.Function.Name
							}
							if toolCallDelta.Function.Arguments != nil {
								toolCall.Function.Arguments += *toolCallDelta.Function.Arguments
							}
						}
					}
				}

				// Build final response from first chunk or update existing
				if finalResponse == nil {
					finalResponse = &ChatCompletionResponse{
						ID:     chunk.ID,
						Object: chunk.Object,
						Choices: []ChatCompletionChoice{
							{
								Index: choice.Index,
								Message: ChatMessage{
									Role:    MessageRoleAssistant,
									Content: &[]string{""}[0], // Will be updated with full content
								},
								FinishReason: "",
							},
						},
					}
				}

				// Update finish reason if provided
				if choice.FinishReason != nil {
					finalResponse.Choices[0].FinishReason = *choice.FinishReason
				}

				// Update usage if provided (typically in final chunk)
				if chunk.Usage != nil {
					finalResponse.Usage = *chunk.Usage
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading streaming response: %w", err)
	}

	if finalResponse == nil {
		return nil, fmt.Errorf("no valid response received from stream")
	}

	// Set the full content
	fullContentStr := fullContent.String()
	finalResponse.Choices[0].Message.Content = &fullContentStr

	// Assemble complete tool calls from accumulated deltas
	if len(toolCallsMap) > 0 {
		toolCalls := make([]ToolCallRequest, 0, len(toolCallsMap))

		// Convert map to ordered slice (by index)
		for i := 0; i < len(toolCallsMap); i++ {
			if toolCall := toolCallsMap[i]; toolCall != nil {
				toolCalls = append(toolCalls, *toolCall)
			}
		}

		if len(toolCalls) > 0 {
			finalResponse.Choices[0].Message.ToolCalls = &toolCalls
		}
	}

	return finalResponse, nil
}

// pushToRetryChan pushes the attempt to the retry channel
func (c *GroqClient) pushToRetryChan(attempt int) {
	select {
	case c.RetryChan <- attempt:
	default:
		// Channel full or no receiver, continue without blocking
	}
}

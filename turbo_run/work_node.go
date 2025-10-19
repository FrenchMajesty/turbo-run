package turbo_run

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/FrenchMajesty/turbo-run/clients/groq"
	"github.com/FrenchMajesty/turbo-run/utils/logger"
	"github.com/FrenchMajesty/turbo-run/utils/retry"
	"github.com/FrenchMajesty/turbo-run/utils/token_counter"
	"github.com/google/uuid"
	openai "github.com/openai/openai-go/v2"
)

type WorkNodeStatus int

const (
	WorkNodeStatusPending WorkNodeStatus = iota
	WorkNodeStatusRunning
	WorkNodeStatusCompleted
	WorkNodeStatusFailed
)

var tokenCounter, _ = token_counter.NewTokenCounter()

type WorkNode struct {
	ID              uuid.UUID
	workFn          func(w *WorkNode, groq *groq.GroqClientInterface, openai *openai.Client) RunResult
	provider        groq.Provider
	estimatedTokens int
	groqReq         groq.ChatCompletionRequest
	openaiReq       openai.ChatCompletionNewParams
	retryConfig     *retry.Config

	// Misc
	verboseLog bool
	status     WorkNodeStatus
	logger     logger.Logger

	// Orchestration
	mu              sync.RWMutex
	resultChan      chan RunResult
	statusChan      chan WorkNodeStatus
	resultCallbacks []func(RunResult)
}

type RunResult struct {
	Value      groq.ChatCompletionResponse
	Error      error
	TokensUsed int
	Duration   time.Duration
}

type WorkNodeExecutableFunc func(w *WorkNode, groq *groq.GroqClientInterface, openai *openai.Client) RunResult

func newWorkNode(provider groq.Provider) *WorkNode {
	node := &WorkNode{
		ID:              uuid.New(),
		provider:        provider,
		resultChan:      make(chan RunResult, 1),
		statusChan:      make(chan WorkNodeStatus, 2),
		status:          WorkNodeStatusPending,
		mu:              sync.RWMutex{},
		resultCallbacks: []func(RunResult){},
		logger:          logger.NewNoopLogger(), // Default to noop
	}

	return node
}

// SetLogger sets the logger for this WorkNode
func (w *WorkNode) SetLogger(l logger.Logger) *WorkNode {
	w.logger = l
	return w
}

// NewWorkNodeForGroq creates a new work node for a groq request
func NewWorkNodeForGroq(body groq.ChatCompletionRequest) *WorkNode {
	node := newWorkNode(groq.ProviderGroq)
	node.groqReq = body
	node.estimatedTokens = tokenCounter.CountRequestTokens(body)
	node.estimatedTokens += (node.estimatedTokens * 20 / 100) // 20% overhead for response tokens
	node.workFn = executeStandardGroqRequest
	return node
}

// NewWorkNodeForOpenAI creates a new work node for an openai request
func NewWorkNodeForOpenAI(body openai.ChatCompletionNewParams) *WorkNode {
	node := newWorkNode(groq.ProviderOpenAI)
	node.openaiReq = body
	bodyBytes, _ := json.Marshal(body)
	node.estimatedTokens = tokenCounter.CountTextTokens(string(bodyBytes))
	return node
}

// GetStatus returns the status of the work node
func (w *WorkNode) GetStatus() WorkNodeStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.status
}

// SetStatus sets the status of the work node
func (w *WorkNode) SetStatus(status WorkNodeStatus) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = status

	select {
	case w.statusChan <- status:
		// Status sent successfully
	default:
		// Channel full or no receiver, continue without blocking
	}
}

// SetWorkFn sets the work function for the work node
func (w *WorkNode) SetWorkFn(workFn WorkNodeExecutableFunc) *WorkNode {
	w.workFn = workFn
	return w
}

// AddResultCallback adds a callback to the work node
func (wn *WorkNode) AddResultCallback(cb func(RunResult)) {
	wn.resultCallbacks = append(wn.resultCallbacks, cb)
}

// GetID returns the id of the work node
func (w *WorkNode) GetID() uuid.UUID {
	return w.ID
}

// EmitResult emits the result to the its dedicated channel
func (w *WorkNode) EmitResult(result RunResult) {
	w.resultChan <- result

	for _, cb := range w.resultCallbacks {
		cb(result)
	}
}

// ListenForResult listens for the result from the its dedicated channel
func (w *WorkNode) ListenForResult() RunResult {
	return <-w.resultChan
}

// GetEstimatedTokens returns the estimated tokens for the work node
func (w *WorkNode) GetEstimatedTokens() int {
	return w.estimatedTokens
}

// GetProvider returns the provider for the work node
func (w *WorkNode) GetProvider() groq.Provider {
	return w.provider
}

// GetOpenaiReq returns the openai request for the work node
func (w *WorkNode) GetOpenaiReq() openai.ChatCompletionNewParams {
	return w.openaiReq
}

// GetGroqReq returns the groq request for the work node
func (w *WorkNode) GetGroqReq() groq.ChatCompletionRequest {
	return w.groqReq
}

// ListenForStatus listens for the status from the its dedicated channel
func (w *WorkNode) ListenForStatus() WorkNodeStatus {
	return <-w.statusChan
}

// NewRetryableWorkNodeForOpenAI creates a new work node with retry support for OpenAI requests
func NewRetryableWorkNodeForOpenAI(body openai.ChatCompletionNewParams) *WorkNode {
	node := NewWorkNodeForOpenAI(body)
	retryConfig := retry.DefaultConfig()
	node.retryConfig = &retryConfig
	node.verboseLog = false
	return node
}

// NewRetryableWorkNodeForGroq creates a new work node with retry support for Groq requests
func NewRetryableWorkNodeForGroq(body groq.ChatCompletionRequest) *WorkNode {
	node := NewWorkNodeForGroq(body)
	retryConfig := retry.DefaultConfig()
	node.retryConfig = &retryConfig
	node.verboseLog = false
	return node
}

// SetRetryConfig sets the retry configuration for the work node
func (w *WorkNode) SetRetryConfig(config retry.Config) *WorkNode {
	w.retryConfig = &config
	return w
}

// SetVerboseLog sets the verbose logging flag for retry operations
func (w *WorkNode) SetVerboseLog(verbose bool) *WorkNode {
	w.verboseLog = verbose
	return w
}

// SetWorkFnWithRetry sets a work function with retry logic
func (w *WorkNode) SetWorkFnWithRetry(workFn WorkNodeExecutableFunc) *WorkNode {
	if w.retryConfig == nil {
		// No retry config, use regular SetWorkFn
		return w.SetWorkFn(workFn)
	}

	retryWorkFn := w.wrapWithRetry(workFn)
	return w.SetWorkFn(retryWorkFn)
}

// wrapWithRetry wraps a work function with retry logic
func (w *WorkNode) wrapWithRetry(workFn WorkNodeExecutableFunc) WorkNodeExecutableFunc {
	return func(workNode *WorkNode, groqClient *groq.GroqClientInterface, openaiClient *openai.Client) RunResult {
		var lastResult RunResult

		for attempt := 0; attempt <= w.retryConfig.MaxRetries; attempt++ {
			// Add delay before retry (but not on first attempt)
			if attempt > 0 {
				delay := w.calculateDelay(attempt - 1)

				// Emit retry event
				turboRun, err := GetTurboRun()
				if err == nil {
					turboRun.emitEvent(EventNodeRetrying, w.ID, map[string]any{
						"attempt":     attempt + 1,
						"max_retries": w.retryConfig.MaxRetries + 1,
						"delay":       delay.String(),
						"error":       lastResult.Error.Error(),
					})
				}

				if w.verboseLog {
					w.logger.Printf("WorkNode retry attempt %d/%d after %v delay for node %s",
						attempt+1, w.retryConfig.MaxRetries+1, delay, w.ID)
				}
				time.Sleep(delay)
			}

			// Execute the work function
			result := workFn(workNode, groqClient, openaiClient)
			lastResult = result

			// If successful, return immediately
			if result.Error == nil {
				if attempt > 0 && w.verboseLog {
					w.logger.Printf("WorkNode succeeded on attempt %d/%d for node %s",
						attempt+1, w.retryConfig.MaxRetries+1, w.ID)
				}
				return result
			}

			// Check if this is a retryable error
			if w.isRetryableError(result.Error) && attempt < w.retryConfig.MaxRetries {
				if w.verboseLog {
					w.logger.Printf("WorkNode retryable error (attempt %d/%d) for node %s: %v",
						attempt+1, w.retryConfig.MaxRetries+1, w.ID, result.Error)
				}
				continue
			}

			// Non-retryable error or max retries exceeded
			if w.verboseLog && attempt < w.retryConfig.MaxRetries {
				w.logger.Printf("WorkNode non-retryable error for node %s: %v", w.ID, result.Error)
			}
			break
		}

		// All retries exhausted, return the last result
		if w.verboseLog {
			w.logger.Printf("WorkNode failed after %d attempts for node %s, last error: %v",
				w.retryConfig.MaxRetries+1, w.ID, lastResult.Error)
		}
		return lastResult
	}
}

// calculateDelay computes the delay for the given attempt using exponential backoff
func (w *WorkNode) calculateDelay(attempt int) time.Duration {
	delay := time.Duration(float64(w.retryConfig.BaseDelay) * math.Pow(w.retryConfig.BackoffMultiple, float64(attempt)))
	if delay > w.retryConfig.MaxDelay {
		delay = w.retryConfig.MaxDelay
	}
	return delay
}

// isRetryableError determines if an error should trigger a retry
func (w *WorkNode) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Generation errors (LLM returned unexpected output) are retryable
	if strings.Contains(errStr, "generation error") {
		return true
	}

	// Network-related errors are retryable
	if strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout") || strings.Contains(errStr, "connection") {
		return true
	}

	// Rate limiting errors are retryable
	if strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "too many requests") {
		return true
	}

	// Server errors (5xx) are retryable
	if strings.Contains(errStr, "internal server error") || strings.Contains(errStr, "bad gateway") ||
		strings.Contains(errStr, "service unavailable") || strings.Contains(errStr, "gateway timeout") {
		return true
	}

	// OpenAI specific errors that are retryable
	if strings.Contains(errStr, "model is currently overloaded") || strings.Contains(errStr, "server is overloaded") {
		return true
	}

	// Groq specific errors that are retryable
	if strings.Contains(errStr, "groq API error 429") || strings.Contains(errStr, "groq API error 5") {
		return true
	}

	// Default to non-retryable
	return false
}

// executeStandardGroqRequest executes a standard groq request
func executeStandardGroqRequest(w *WorkNode, groq *groq.GroqClientInterface, openai *openai.Client) RunResult {
	now := time.Now()
	if groq == nil {
		return RunResult{
			Error: fmt.Errorf("groq is nil"),
		}
	}

	w.SetStatus(WorkNodeStatusRunning)

	// Emit running event
	turboRun, err := GetTurboRun()
	if err == nil {
		turboRun.emitEvent(EventNodeRunning, w.ID, map[string]any{
			"provider": "groq",
		})
	}

	response, err := (*groq).ChatCompletion(context.Background(), w.GetGroqReq())
	if err != nil {
		w.SetStatus(WorkNodeStatusFailed)
		return RunResult{
			Error:    err,
			Duration: time.Since(now),
		}
	}

	w.SetStatus(WorkNodeStatusCompleted)
	return RunResult{
		Value:    *response,
		Error:    nil,
		Duration: time.Since(now),
	}
}

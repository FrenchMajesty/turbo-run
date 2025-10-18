package turbo_run

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// SharedRateLimitState represents the shared rate limit state across all processes
type SharedRateLimitState struct {
	CurrentMinute string               `json:"current_minute"`
	Usage         map[string]usageData `json:"usage"`
	LastUpdated   time.Time            `json:"last_updated"`
}

// SharedRateLimiter manages rate limits across multiple processes using file-based coordination
type SharedRateLimiter struct {
	filePath string
	budgets  map[Provider]RateLimit
}

// NewSharedRateLimiter creates a new shared rate limiter
func NewSharedRateLimiter() *SharedRateLimiter {
	budgets := map[Provider]RateLimit{
		ProviderGroq:   GroqRateLimit,
		ProviderOpenAI: OpenAIRateLimit,
	}

	return &SharedRateLimiter{
		filePath: filepath.Join(projectRoot(), "_rate_limits.json"),
		budgets:  budgets,
	}
}

// BudgetAvailableForCycle returns available budget for the given provider
func (srl *SharedRateLimiter) BudgetAvailableForCycle(provider Provider) (tokensAvailable int, requestsAvailable int) {
	state, err := srl.readStateWithLock()
	if err != nil {
		// Fallback to full budget if we can't read the file
		budget := srl.budgets[provider]
		return budget.TPM, budget.RPM
	}

	currentMinute := time.Now().Truncate(time.Minute).Format(time.RFC3339)

	// If we're in a new minute, reset the usage
	if state.CurrentMinute != currentMinute {
		state = &SharedRateLimitState{
			CurrentMinute: currentMinute,
			Usage:         make(map[string]usageData),
			LastUpdated:   time.Now(),
		}
		// Try to write the reset state, but don't fail if we can't
		srl.writeStateWithLock(state)
	}

	providerKey := srl.getProviderKey(provider)
	usage := state.Usage[providerKey]
	budget := srl.budgets[provider]

	tokensAvailable = budget.TPM - usage.Tokens
	requestsAvailable = budget.RPM - usage.Requests

	if tokensAvailable < 0 {
		tokensAvailable = 0
	}
	if requestsAvailable < 0 {
		requestsAvailable = 0
	}

	return tokensAvailable, requestsAvailable
}

// RecordConsumption records usage for the given provider
func (srl *SharedRateLimiter) RecordConsumption(provider Provider, tokens int) error {
	return srl.updateUsage(provider, tokens, 1)
}

// TimeUntilReset returns time until the next minute boundary
func (srl *SharedRateLimiter) TimeUntilReset() time.Duration {
	now := time.Now()
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	return nextMinute.Sub(now)
}

// updateUsage atomically updates the usage for a provider
func (srl *SharedRateLimiter) updateUsage(provider Provider, tokens, requests int) error {
	state, err := srl.readStateWithLock()
	if err != nil {
		// If we can't read, try to create a new state
		state = &SharedRateLimitState{
			CurrentMinute: time.Now().Truncate(time.Minute).Format(time.RFC3339),
			Usage:         make(map[string]usageData),
			LastUpdated:   time.Now(),
		}
	}

	currentMinute := time.Now().Truncate(time.Minute).Format(time.RFC3339)

	// Reset if we're in a new minute
	if state.CurrentMinute != currentMinute {
		state.CurrentMinute = currentMinute
		state.Usage = make(map[string]usageData)
	}

	providerKey := srl.getProviderKey(provider)
	usage := state.Usage[providerKey]
	usage.Tokens += tokens
	usage.Requests += requests
	state.Usage[providerKey] = usage
	state.LastUpdated = time.Now()

	return srl.writeStateWithLock(state)
}

// readStateWithLock reads the state file with file locking
func (srl *SharedRateLimiter) readStateWithLock() (*SharedRateLimitState, error) {
	file, err := os.OpenFile(srl.filePath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open rate limit file: %w", err)
	}
	defer file.Close()

	// Lock the file for reading
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_SH); err != nil {
		return nil, fmt.Errorf("failed to lock file for reading: %w", err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	var state SharedRateLimitState
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&state); err != nil {
		// File might be empty or corrupted, return empty state
		if err.Error() == "EOF" {
			return &SharedRateLimitState{
				CurrentMinute: time.Now().Truncate(time.Minute).Format(time.RFC3339),
				Usage:         make(map[string]usageData),
				LastUpdated:   time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("failed to decode state: %w", err)
	}

	if state.Usage == nil {
		state.Usage = make(map[string]usageData)
	}

	return &state, nil
}

// writeStateWithLock writes the state file with file locking
func (srl *SharedRateLimiter) writeStateWithLock(state *SharedRateLimitState) error {
	file, err := os.OpenFile(srl.filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open rate limit file for writing: %w", err)
	}
	defer file.Close()

	// Lock the file for writing
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("failed to lock file for writing: %w", err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(state); err != nil {
		return fmt.Errorf("failed to encode state: %w", err)
	}

	return nil
}

// getProviderKey returns a string key for the provider
func (srl *SharedRateLimiter) getProviderKey(provider Provider) string {
	switch provider {
	case ProviderGroq:
		return "groq"
	case ProviderOpenAI:
		return "openai"
	default:
		return "unknown"
	}
}

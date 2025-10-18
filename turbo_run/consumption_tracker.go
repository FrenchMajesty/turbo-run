package turbo_run

import (
	"sync"
	"time"

	"github.com/FrenchMajesty/turbo-run/rate_limit"
	"github.com/FrenchMajesty/turbo-run/rate_limit/backends/memory"
)

type usageData struct {
	Tokens   int
	Requests int
}
type consumptionTracker struct {
	// Current cycle state
	current       map[Provider]usageData
	currentMinute time.Time

	// total stats
	total map[Provider]usageData

	// Historical data
	history map[string]map[Provider]usageData

	// Thread safety
	mu sync.RWMutex

	// Budgets
	groqBudgetTokens     int
	openaiBudgetTokens   int
	groqBudgetRequests   int
	openaiBudgetRequests int

	// Backend for rate limit persistence (supports UDS, Redis, in-memory, etc.)
	backend rate_limit.Backend
}

type ConsumptionTrackerStats struct {
	// Groq stats
	GroqCurrentTokens int
	GroqTotalTokens   int

	// OpenAI stats
	OpenAICurrentTokens int
	OpenAITotalTokens   int

	// Global stats
	TotalTokens    int
	TotalRequests  int
	TimeUntilReset time.Duration
	IsBlocked      bool
}

func NewConsumptionTracker(backend rate_limit.Backend) *consumptionTracker {
	// Use in-memory backend as fallback if none provided
	if backend == nil {
		backend = memory.NewMemory()
	}

	return &consumptionTracker{
		current:              make(map[Provider]usageData),
		total:                make(map[Provider]usageData),
		history:              make(map[string]map[Provider]usageData),
		currentMinute:        time.Now().Truncate(time.Minute),
		groqBudgetTokens:     int(rate_limit.GroqRateLimit.TPM),
		openaiBudgetTokens:   int(rate_limit.OpenAIRateLimit.TPM),
		groqBudgetRequests:   rate_limit.GroqRateLimit.RPM,
		openaiBudgetRequests: rate_limit.OpenAIRateLimit.RPM,
		backend:              backend,
	}
}

// Cycle resets the current minute and records the historical data
func (ct *consumptionTracker) Cycle() {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	newMinute := time.Now().Truncate(time.Minute)

	// Only cycle if we're actually in a new minute to avoid double-cycling
	if ct.currentMinute.Equal(newMinute) {
		return
	}

	writeIndex := ct.currentMinute.Format(time.RFC3339)
	ct.history[writeIndex] = map[Provider]usageData{
		ProviderGroq:   ct.current[ProviderGroq],
		ProviderOpenAI: ct.current[ProviderOpenAI],
	}

	ct.currentMinute = newMinute
	ct.resetUsageData(ProviderGroq)
	ct.resetUsageData(ProviderOpenAI)
}

// SetBudgetsForTests sets the budgets for the consumption tracker (used primarily for testing)
func (ct *consumptionTracker) SetBudgetsForTests(groqBudgetTokens int, openaiBudgetTokens int, groqBudgetRequests int, openaiBudgetRequests int) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.groqBudgetTokens = groqBudgetTokens
	ct.openaiBudgetTokens = openaiBudgetTokens
	ct.groqBudgetRequests = groqBudgetRequests
	ct.openaiBudgetRequests = openaiBudgetRequests

	// Also update backend budgets for cross-process coordination
	ct.backend.SetBudgetForTests(convertProvider(ProviderGroq), groqBudgetTokens, groqBudgetRequests)
	ct.backend.SetBudgetForTests(convertProvider(ProviderOpenAI), openaiBudgetTokens, openaiBudgetRequests)
}

// RecordConsumption records the consumption for the current minute
func (ct *consumptionTracker) RecordConsumption(provider Provider, tokens int) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.increaseTokens(provider, tokens)
	ct.increaseRequests(provider, 1)

	// Update totals as well
	totalData := ct.total[provider]
	totalData.Tokens += tokens
	totalData.Requests += 1
	ct.total[provider] = totalData

	// Also record in backend for cross-process coordination
	// Don't fail if this doesn't work - local tracking is still functional
	ct.backend.RecordConsumption(convertProvider(provider), tokens, 1)
}

// TimeUntilReset returns the time until the current minute resets
func (ct *consumptionTracker) TimeUntilReset() time.Duration {
	// Use backend for consistent timing across processes
	return ct.backend.TimeUntilReset()
}

// BudgetAvailableForCycle returns the budget available for the current cycle for the given provider
func (ct *consumptionTracker) BudgetAvailableForCycle(provider Provider) (int, int) {
	// Use backend for cross-process coordination
	sharedTokens, sharedRequests := ct.backend.BudgetAvailable(convertProvider(provider))

	// Check and sync local state if needed
	ct.mu.Lock()
	currentMinute := time.Now().Truncate(time.Minute)
	if !ct.currentMinute.Equal(currentMinute) {
		// Local state is stale, sync it
		ct.currentMinute = currentMinute
		ct.resetUsageData(ProviderGroq)
		ct.resetUsageData(ProviderOpenAI)
	}

	localTokens := ct.getAvailableTokens(provider)
	localRequests := ct.getAvailableRequests(provider)

	// Check if we're in a minute transition period (within 2 seconds of minute boundary)
	now := time.Now()
	secondsIntoMinute := float64(now.Second()) + float64(now.Nanosecond())/1000000000.0
	isTransitionPeriod := secondsIntoMinute < 2.0 || secondsIntoMinute > 58.0
	ct.mu.Unlock()

	// During transitions, be more lenient but still respect limits to avoid temporary blocks
	var tokens, requests int
	if isTransitionPeriod {
		// During transitions, use maximum unless one system shows zero budget
		// This handles the case where one tracker reset but the other hasn't
		if sharedTokens == 0 && localTokens > 0 {
			tokens = localTokens
		} else if localTokens == 0 && sharedTokens > 0 {
			tokens = sharedTokens
		} else {
			// Both have budget or both are zero - use minimum for safety
			tokens = sharedTokens
			if localTokens < tokens {
				tokens = localTokens
			}
		}

		if sharedRequests == 0 && localRequests > 0 {
			requests = localRequests
		} else if localRequests == 0 && sharedRequests > 0 {
			requests = sharedRequests
		} else {
			// Both have budget or both are zero - use minimum for safety
			requests = sharedRequests
			if localRequests < requests {
				requests = localRequests
			}
		}
	} else {
		// Normal operation: use minimum of shared and local limits for conservative budgeting
		tokens = sharedTokens
		if localTokens < tokens {
			tokens = localTokens
		}

		requests = sharedRequests
		if localRequests < requests {
			requests = localRequests
		}
	}

	return tokens, requests
}

// GetTotalConsumption returns the total consumption for all providers
func (ct *consumptionTracker) GetTotalConsumption() (int, int) {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	totalTokens := ct.total[ProviderGroq].Tokens + ct.total[ProviderOpenAI].Tokens
	totalRequests := ct.total[ProviderGroq].Requests + ct.total[ProviderOpenAI].Requests
	return totalTokens, totalRequests
}

// GetCurrentConsumption returns the current consumption for all providers
func (ct *consumptionTracker) GetCurrentConsumption() (int, int) {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	totalTokens := ct.current[ProviderGroq].Tokens + ct.current[ProviderOpenAI].Tokens
	totalRequests := ct.current[ProviderGroq].Requests + ct.current[ProviderOpenAI].Requests
	return totalTokens, totalRequests
}

// GetStats returns the stats of the consumption tracker
func (ct *consumptionTracker) GetStats() *ConsumptionTrackerStats {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	return &ConsumptionTrackerStats{
		GroqCurrentTokens:   ct.current[ProviderGroq].Tokens,
		GroqTotalTokens:     ct.total[ProviderGroq].Tokens,
		OpenAICurrentTokens: ct.current[ProviderOpenAI].Tokens,
		OpenAITotalTokens:   ct.total[ProviderOpenAI].Tokens,
		TotalTokens:         ct.total[ProviderGroq].Tokens + ct.total[ProviderOpenAI].Tokens,
		TotalRequests:       ct.total[ProviderGroq].Requests + ct.total[ProviderOpenAI].Requests,
		TimeUntilReset:      ct.TimeUntilReset(),
		IsBlocked:           ct.IsBlocked(),
	}
}

// IsBlocked returns true if the consumption tracker is blocked
func (ct *consumptionTracker) IsBlocked() bool {
	return ct.TimeUntilReset() == 0
}

// getAvailableTokens returns the available tokens for the given provider
// Note: This method assumes the caller already holds a lock
func (ct *consumptionTracker) getAvailableTokens(provider Provider) int {
	var tokenBudget int

	switch provider {
	case ProviderOpenAI:
		tokenBudget = ct.openaiBudgetTokens
	default:
		tokenBudget = ct.groqBudgetTokens
	}

	return tokenBudget - ct.current[provider].Tokens
}

// getAvailableRequests returns the available requests for the given provider
// Note: This method assumes the caller already holds a lock
func (ct *consumptionTracker) getAvailableRequests(provider Provider) int {
	var requestBudget int

	switch provider {
	case ProviderOpenAI:
		requestBudget = ct.openaiBudgetRequests
	default:
		requestBudget = ct.groqBudgetRequests
	}

	return requestBudget - ct.current[provider].Requests
}

// resetUsageData resets the usage data for the given provider
// Note: This method assumes the caller already holds a lock
func (ct *consumptionTracker) resetUsageData(provider Provider) {
	ct.current[provider] = usageData{
		Tokens:   0,
		Requests: 0,
	}
}

// increaseTokens increases the tokens for the given provider
// Note: This method assumes the caller already holds a lock
func (ct *consumptionTracker) increaseTokens(provider Provider, tokens int) {
	currentData := ct.current[provider]
	currentData.Tokens += tokens
	ct.current[provider] = currentData
}

// increaseRequests increases the requests for the given provider
// Note: This method assumes the caller already holds a lock
func (ct *consumptionTracker) increaseRequests(provider Provider, requests int) {
	currentData := ct.current[provider]
	currentData.Requests += requests
	ct.current[provider] = currentData
}

// convertProvider converts turbo_run.Provider to ratelimit.Provider
func convertProvider(p Provider) rate_limit.Provider {
	switch p {
	case ProviderGroq:
		return rate_limit.ProviderGroq
	case ProviderOpenAI:
		return rate_limit.ProviderOpenAI
	default:
		return rate_limit.ProviderGroq
	}
}

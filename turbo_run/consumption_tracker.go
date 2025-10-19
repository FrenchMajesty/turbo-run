package turbo_run

import (
	"sync"
	"time"

	"github.com/FrenchMajesty/turbo-run/clients/groq"
	"github.com/FrenchMajesty/turbo-run/rate_limit"
)

type usageData struct {
	Tokens   int
	Requests int
}
type consumptionTracker struct {
	// Current cycle state
	current       map[groq.Provider]usageData
	currentMinute time.Time

	// total stats
	total map[groq.Provider]usageData

	// Historical data
	history map[string]map[groq.Provider]usageData

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
	return &consumptionTracker{
		current:              make(map[groq.Provider]usageData),
		total:                make(map[groq.Provider]usageData),
		history:              make(map[string]map[groq.Provider]usageData),
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
	ct.history[writeIndex] = map[groq.Provider]usageData{
		groq.ProviderGroq:   ct.current[groq.ProviderGroq],
		groq.ProviderOpenAI: ct.current[groq.ProviderOpenAI],
	}

	ct.currentMinute = newMinute
	ct.resetUsageData(groq.ProviderGroq)
	ct.resetUsageData(groq.ProviderOpenAI)
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
	ct.backend.SetBudgetForTests(groq.ProviderGroq, groqBudgetTokens, groqBudgetRequests)
	ct.backend.SetBudgetForTests(groq.ProviderOpenAI, openaiBudgetTokens, openaiBudgetRequests)
}

// RecordConsumption records the consumption for the current minute
func (ct *consumptionTracker) RecordConsumption(provider groq.Provider, tokens int) {
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
	ct.backend.RecordConsumption(provider, tokens, 1)
}

// TimeUntilReset returns the time until the current minute resets
func (ct *consumptionTracker) TimeUntilReset() time.Duration {
	// Use backend for consistent timing across processes
	return ct.backend.TimeUntilReset()
}

// BudgetAvailableForCycle returns the budget available for the current cycle for the given provider
func (ct *consumptionTracker) BudgetAvailableForCycle(provider groq.Provider) (int, int) {
	// Use backend for cross-process coordination
	sharedTokens, sharedRequests := ct.backend.BudgetAvailable(provider)

	// Check and sync local state if needed
	ct.mu.Lock()
	currentMinute := time.Now().Truncate(time.Minute)
	if !ct.currentMinute.Equal(currentMinute) {
		// Local state is stale, sync it
		ct.currentMinute = currentMinute
		ct.resetUsageData(groq.ProviderGroq)
		ct.resetUsageData(groq.ProviderOpenAI)
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

	totalTokens := ct.total[groq.ProviderGroq].Tokens + ct.total[groq.ProviderOpenAI].Tokens
	totalRequests := ct.total[groq.ProviderGroq].Requests + ct.total[groq.ProviderOpenAI].Requests
	return totalTokens, totalRequests
}

// GetCurrentConsumption returns the current consumption for all providers
func (ct *consumptionTracker) GetCurrentConsumption() (int, int) {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	totalTokens := ct.current[groq.ProviderGroq].Tokens + ct.current[groq.ProviderOpenAI].Tokens
	totalRequests := ct.current[groq.ProviderGroq].Requests + ct.current[groq.ProviderOpenAI].Requests
	return totalTokens, totalRequests
}

// GetStats returns the stats of the consumption tracker
func (ct *consumptionTracker) GetStats() *ConsumptionTrackerStats {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	return &ConsumptionTrackerStats{
		GroqCurrentTokens:   ct.current[groq.ProviderGroq].Tokens,
		GroqTotalTokens:     ct.total[groq.ProviderGroq].Tokens,
		OpenAICurrentTokens: ct.current[groq.ProviderOpenAI].Tokens,
		OpenAITotalTokens:   ct.total[groq.ProviderOpenAI].Tokens,
		TotalTokens:         ct.total[groq.ProviderGroq].Tokens + ct.total[groq.ProviderOpenAI].Tokens,
		TotalRequests:       ct.total[groq.ProviderGroq].Requests + ct.total[groq.ProviderOpenAI].Requests,
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
func (ct *consumptionTracker) getAvailableTokens(provider groq.Provider) int {
	var tokenBudget int

	switch provider {
	case groq.ProviderOpenAI:
		tokenBudget = ct.openaiBudgetTokens
	default:
		tokenBudget = ct.groqBudgetTokens
	}

	return tokenBudget - ct.current[provider].Tokens
}

// getAvailableRequests returns the available requests for the given provider
// Note: This method assumes the caller already holds a lock
func (ct *consumptionTracker) getAvailableRequests(provider groq.Provider) int {
	var requestBudget int

	switch provider {
	case groq.ProviderOpenAI:
		requestBudget = ct.openaiBudgetRequests
	default:
		requestBudget = ct.groqBudgetRequests
	}

	return requestBudget - ct.current[provider].Requests
}

// resetUsageData resets the usage data for the given provider
// Note: This method assumes the caller already holds a lock
func (ct *consumptionTracker) resetUsageData(provider groq.Provider) {
	ct.current[provider] = usageData{
		Tokens:   0,
		Requests: 0,
	}
}

// increaseTokens increases the tokens for the given provider
// Note: This method assumes the caller already holds a lock
func (ct *consumptionTracker) increaseTokens(provider groq.Provider, tokens int) {
	currentData := ct.current[provider]
	currentData.Tokens += tokens
	ct.current[provider] = currentData
}

// increaseRequests increases the requests for the given provider
// Note: This method assumes the caller already holds a lock
func (ct *consumptionTracker) increaseRequests(provider groq.Provider, requests int) {
	currentData := ct.current[provider]
	currentData.Requests += requests
	ct.current[provider] = currentData
}

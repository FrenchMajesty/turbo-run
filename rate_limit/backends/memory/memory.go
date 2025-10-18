package memory

import (
	"sync"
	"time"

	"github.com/FrenchMajesty/turbo-run/rate_limit"
)

// usageData tracks token and request consumption
type usageData struct {
	Tokens   int
	Requests int
}

// Memory is an in-memory rate limit backend for single-process scenarios.
// It tracks rate limits locally without any inter-process communication.
type Memory struct {
	state         map[rate_limit.Provider]usageData
	currentMinute time.Time
	budgets       map[rate_limit.Provider]rate_limit.RateLimit
	mu            sync.RWMutex
}

// NewMemory creates a new in-memory rate limit backend with default budgets
func NewMemory() *Memory {
	return &Memory{
		state:         make(map[rate_limit.Provider]usageData),
		currentMinute: time.Now().Truncate(time.Minute),
		budgets: map[rate_limit.Provider]rate_limit.RateLimit{
			rate_limit.ProviderGroq:   rate_limit.GroqRateLimit,
			rate_limit.ProviderOpenAI: rate_limit.OpenAIRateLimit,
		},
	}
}

// BudgetAvailable returns the available token and request budget for the given provider
func (m *Memory) BudgetAvailable(provider rate_limit.Provider) (tokensAvailable int, requestsAvailable int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checkAndResetMinute()

	usage := m.state[provider]
	budget := m.budgets[provider]

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

// RecordConsumption records token and request usage for the given provider
func (m *Memory) RecordConsumption(provider rate_limit.Provider, tokens int, requests int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checkAndResetMinute()

	usage := m.state[provider]
	usage.Tokens += tokens
	usage.Requests += requests
	m.state[provider] = usage

	return nil
}

// TimeUntilReset returns the duration until the next minute boundary
func (m *Memory) TimeUntilReset() time.Duration {
	now := time.Now()
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	return nextMinute.Sub(now)
}

// SetBudgetForTests sets custom budgets for testing purposes
func (m *Memory) SetBudgetForTests(provider rate_limit.Provider, tokens int, requests int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.budgets[provider] = rate_limit.RateLimit{
		TPM: tokens,
		RPM: requests,
	}

	return nil
}

// Close is a no-op for in-memory backend (no resources to clean up)
func (m *Memory) Close() error {
	return nil
}

// checkAndResetMinute resets state if we're in a new minute
// Note: caller must hold the lock
func (m *Memory) checkAndResetMinute() {
	currentMinute := time.Now().Truncate(time.Minute)
	if !m.currentMinute.Equal(currentMinute) {
		m.currentMinute = currentMinute
		m.state = make(map[rate_limit.Provider]usageData)
	}
}

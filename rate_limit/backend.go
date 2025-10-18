package rate_limit

import "time"

// Provider represents an LLM provider
type Provider int

const (
	ProviderGroq Provider = iota
	ProviderOpenAI
)

// Backend defines the interface for rate limit persistence backends.
// Implementations can use different mechanisms (UDS, Redis, file-based, in-memory, etc.)
// to track and enforce rate limits across single or multiple processes.
type Backend interface {
	// BudgetAvailable returns the available token and request budget for the given provider
	// for the current time window (typically per minute).
	BudgetAvailable(provider Provider) (tokensAvailable int, requestsAvailable int)

	// RecordConsumption records token and request usage for the given provider.
	RecordConsumption(provider Provider, tokens int, requests int) error

	// TimeUntilReset returns the duration until the next budget reset (typically next minute boundary).
	TimeUntilReset() time.Duration

	// SetBudgetForTests allows overriding rate limits for testing purposes.
	SetBudgetForTests(provider Provider, tokens int, requests int) error

	// Close cleans up any resources held by the backend (connections, files, etc.)
	Close() error
}

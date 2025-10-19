package turbo_run

import (
	"time"

	"github.com/FrenchMajesty/turbo-run/clients/groq"
	"github.com/FrenchMajesty/turbo-run/rate_limit"
	"github.com/google/uuid"
)

// startMinuteTimer starts a timer that will call onMinuteChange every minute
func (tr *TurboRun) startMinuteTimer() {
	// Calculate time until next minute boundary
	now := time.Now()
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	timeUntilNextMinute := nextMinute.Sub(now)

	// Wait until next minute boundary (with quit check)
	select {
	case <-tr.quit:
		return // Shutdown before first tick
	case <-time.After(timeUntilNextMinute):
		// Continue to ticker
	}

	// Now start ticker for every minute
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-tr.quit:
			return // Shutdown signal
		case <-ticker.C:
			tr.onMinuteChange()
		}
	}
}

// onMinuteChange is called when the minute changes
func (tr *TurboRun) onMinuteChange() {
	tr.tracker.Cycle()

	// Emit budget reset event
	tr.emitEvent(EventBudgetReset, uuid.Nil, map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"providers": []string{"groq", "openai"},
	})
}

// getBudgetTotal returns the total token budget for a provider
func (tr *TurboRun) getBudgetTotal(provider groq.Provider) int {
	switch provider {
	case groq.ProviderGroq:
		return int(rate_limit.GroqRateLimit.TPM)
	case groq.ProviderOpenAI:
		return int(rate_limit.OpenAIRateLimit.TPM)
	default:
		return 0
	}
}

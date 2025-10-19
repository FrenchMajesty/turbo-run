package turbo_run

import (
	"fmt"
	"time"

	"github.com/FrenchMajesty/turbo-run/clients/groq"
	"github.com/google/uuid"
)

type EventType string

const (
	// Node lifecycle events
	EventNodeCreated     EventType = "node_created"
	EventNodeReady       EventType = "node_ready"
	EventNodePrioritized EventType = "node_prioritized"
	EventNodeDispatched  EventType = "node_dispatched"
	EventNodeRunning     EventType = "node_running"
	EventNodeRetrying    EventType = "node_retrying"
	EventNodeCompleted   EventType = "node_completed"
	EventNodeFailed      EventType = "node_failed"
	EventNodeCancelled   EventType = "node_cancelled"

	// Rate limit budget events
	EventBudgetConsumed EventType = "budget_consumed"
	EventBudgetBlocked  EventType = "budget_blocked"
	EventBudgetReset    EventType = "budget_reset"
	EventBudgetWarning  EventType = "budget_warning"

	// Graph capacity events
	EventGraphFull    EventType = "graph_full"
	EventGraphResumed EventType = "graph_resumed"
)

type Event struct {
	Type      EventType      `json:"type"`
	NodeID    string         `json:"node_id"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

// GetEventChan returns the event channel for external listeners
func (tr *TurboRun) GetEventChan() <-chan *Event {
	return tr.eventChan
}

// emitEvent sends an event to the event channel (non-blocking)
func (tr *TurboRun) emitEvent(eventType EventType, nodeID uuid.UUID, data map[string]any) {
	if tr.eventChan == nil {
		return
	}

	event := &Event{
		Type:      eventType,
		NodeID:    nodeID.String(),
		Timestamp: time.Now(),
		Data:      data,
	}

	select {
	case tr.eventChan <- event:
		// Event sent successfully
	default:
		// Channel full, drop event to avoid blocking
	}
}

// GetStats returns the stats of the turbo run
func (tr *TurboRun) GetStats() *TurboRunStats {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return &TurboRunStats{
		GraphSize:         tr.graph.Size(),
		PriorityQueueSize: tr.priorityQueue.Size(),
		LaunchpadSize:     len(tr.launchpad),
		PushQueueSize:     len(tr.pushChan),
		LaunchedCount:     tr.launchedCount,
		CompletedCount:    tr.completedCount,
		FailedCount:       tr.failedCount,
		WorkersPoolSize:   tr.workersPool.GetWorkerCount(),
		WorkersPoolBusy:   tr.workersPool.GetBusyWorkers(),
		TrackerStats:      tr.tracker.GetStats(),
	}
}

// logStats logs current stats if there's activity
func (tr *TurboRun) logStats(shutdown bool) {
	stats := tr.GetStats()

	if shutdown {
		tr.logger.Printf(
			"TurboRun %s: Shutting down. Total nodes launched: %d. Total nodes failed: %d. Total tokens used: %s. Requests made: %d. Time taken: %s",
			tr.uniqueID,
			stats.LaunchedCount,
			stats.FailedCount,
			formatTokens(stats.TrackerStats.TotalTokens),
			stats.TrackerStats.TotalRequests,
			time.Since(tr.startTime),
		)
		return
	}

	// Only log if there's actual processing activity (not just queued nodes while paused)
	// Check if paused - if paused and nothing launched yet, don't log
	tr.mu.RLock()
	isPaused := tr.paused
	tr.mu.RUnlock()

	// Log if: actively processing (not paused) OR workers are busy OR nodes have been launched
	if (!isPaused && (stats.PriorityQueueSize > 0 || stats.GraphSize > 0)) || stats.WorkersPoolBusy > 0 || stats.LaunchedCount > 0 {
		trackerStats := stats.TrackerStats
		tr.logger.Printf("TurboRun %s: Workers(%d/%d) Queue(%d) Graph(%d) Launched(%d) Failed(%d) Tokens(groq:%s openai:%s total:%s)",
			tr.uniqueID,
			stats.WorkersPoolBusy,
			stats.WorkersPoolSize,
			stats.PriorityQueueSize,
			stats.GraphSize,
			stats.LaunchedCount,
			stats.FailedCount,
			formatTokens(trackerStats.GroqCurrentTokens),
			formatTokens(trackerStats.OpenAICurrentTokens),
			formatTokens(trackerStats.TotalTokens),
		)
	}
}

// formatTokens formats token counts in a human-readable way
func formatTokens(tokens int) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1_000_000)
	} else if tokens >= 1000 {
		return fmt.Sprintf("%.1fK", float64(tokens)/1_000)
	}
	return fmt.Sprintf("%d", tokens)
}

// providerName converts Provider enum to string for event data
func providerName(p groq.Provider) string {
	switch p {
	case groq.ProviderGroq:
		return "groq"
	case groq.ProviderOpenAI:
		return "openai"
	default:
		return "unknown"
	}
}

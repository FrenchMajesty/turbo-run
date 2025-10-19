package turbo_run

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/FrenchMajesty/turbo-run/clients/groq"
	"github.com/FrenchMajesty/turbo-run/rate_limit"
	"github.com/FrenchMajesty/turbo-run/rate_limit/backends/memory"
	"github.com/FrenchMajesty/turbo-run/utils/logger"
	"github.com/FrenchMajesty/turbo-run/utils/priority_queue"
	"github.com/google/uuid"
	openai "github.com/openai/openai-go/v2"
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

	// Rate limit budget events
	EventBudgetConsumed EventType = "budget_consumed"
	EventBudgetBlocked  EventType = "budget_blocked"
	EventBudgetReset    EventType = "budget_reset"
	EventBudgetWarning  EventType = "budget_warning"
)

type Event struct {
	Type      EventType      `json:"type"`
	NodeID    string         `json:"node_id"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

type TurboRun struct {
	// Core components
	graph         *Graph
	priorityQueue *priority_queue.PriorityQueue[*WorkNode]
	tracker       *consumptionTracker
	workersPool   *workerPool

	// Channels
	quit            chan struct{}
	launchpad       chan any
	eventChan       chan *Event
	workerStateChan chan int

	// Misc
	mu     sync.RWMutex  // protects launchedCount and stats reading
	logger logger.Logger // pluggable logger

	// Stats attributes
	uniqueID       string
	startTime      time.Time
	failedCount    int
	completedCount int
	launchedCount  int
}

type TurboRunStats struct {
	GraphSize         int
	PriorityQueueSize int
	LaunchpadSize     int
	LaunchedCount     int
	CompletedCount    int
	FailedCount       int
	WorkersPoolSize   int
	WorkersPoolBusy   int
	TrackerStats      *ConsumptionTrackerStats
}

var instance *TurboRun
var once sync.Once

// TurboRunOption is a functional option for configuring TurboRun
type TurboRunOption func(*TurboRun)

// WithLogger sets a custom logger for TurboRun
func WithLogger(l logger.Logger) TurboRunOption {
	return func(tr *TurboRun) {
		tr.logger = l
	}
}

func NewTurboRun(
	groq groq.GroqClientInterface,
	openai *openai.Client,
	opts ...TurboRunOption,
) *TurboRun {
	return NewTurboRunWithBackend(groq, openai, nil, opts...)
}

// NewTurboRunWithBackend creates a new TurboRun instance with a custom rate limit backend.
// If backend is nil, defaults to in-memory backend for cross-process coordination.
func NewTurboRunWithBackend(
	groq groq.GroqClientInterface,
	openai *openai.Client,
	backend rate_limit.Backend,
	opts ...TurboRunOption,
) *TurboRun {
	once.Do(func() {
		env := os.Getenv("ENV")
		uniqueID := uuid.New().String()[:6]

		// Default to in-memory backend
		if backend == nil {
			backend = memory.NewBackend()
		}

		workerStateChan := make(chan int, 100)

		instance = &TurboRun{
			uniqueID:        uniqueID,
			graph:           NewGraph(),
			priorityQueue:   priority_queue.NewMaxPriorityQueue[*WorkNode](),
			tracker:         NewConsumptionTracker(backend),
			quit:            make(chan struct{}),
			launchpad:       make(chan any, 100),
			eventChan:       make(chan *Event, 1000),
			workerStateChan: workerStateChan,
			logger:          logger.NewStdoutLogger(), // Default to stdout
			startTime:       time.Now(),
		}

		// Apply functional options
		for _, opt := range opts {
			opt(instance)
		}

		instance.workersPool = NewWorkerPool(120, &groq, openai, workerStateChan)

		instance.Start()

		// Log initial startup
		if env == "dev" || env == "testing" {
			instance.logger.Printf("TurboRun %s: Started with %d workers", instance.uniqueID, 120)
		} else {
			instance.logger.Printf("TurboRun %s: Started with %d workers (env=%s)", instance.uniqueID, 120, env)
		}
	})

	return instance
}

// GetTurboRun returns the singleton instance of the turbo run
func GetTurboRun() (*TurboRun, error) {
	if instance == nil {
		return nil, fmt.Errorf("turbo run instance is nil")
	}

	return instance, nil
}

// Start starts the turbo runner
func (tr *TurboRun) Start() {
	go instance.listenForReadyNodes()
	go instance.listenForLaunchPad()
	go instance.startMinuteTimer()
	go instance.listenForWorkerStateChanges()

	// Start analytics logging if in dev or testing environment
	env := os.Getenv("ENV")
	if env == "dev" || env == "testing" {
		go instance.startAnalyticsLogger()
	}

	time.Sleep(10 * time.Millisecond) // give time for the goroutines to start
}

// Stop stops the turbo runner
func (tr *TurboRun) Stop() {
	tr.workersPool.Stop()
	close(tr.quit)

	// Log shutdown
	tr.logger.Printf("TurboRun %s: Shutting down", tr.uniqueID)

	// Close file logger if it's a FileLogger
	if fileLogger, ok := tr.logger.(*logger.FileLogger); ok {
		fileLogger.Close()
	}

	if tr.eventChan != nil {
		close(tr.eventChan)
	}
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

// Push adds a work node to the graph with no dependencies
func (tr *TurboRun) Push(workNode *WorkNode) *TurboRun {
	// Set logger on the WorkNode if it doesn't have one
	if _, ok := workNode.logger.(*logger.NoopLogger); workNode.logger == nil || ok {
		workNode.SetLogger(tr.logger)
	}

	tr.graph.Add(workNode, []uuid.UUID{})

	// Emit node created event
	tr.emitEvent(EventNodeCreated, workNode.ID, map[string]any{
		"dependencies":     []string{},
		"estimated_tokens": workNode.GetEstimatedTokens(),
		"provider":         string(workNode.GetProvider()),
	})

	return tr
}

// PushWithDependencies adds a work node to the graph with dependencies
func (tr *TurboRun) PushWithDependencies(workNode *WorkNode, dependencies []uuid.UUID) *TurboRun {
	// Set logger on the WorkNode if it doesn't have one
	if _, ok := workNode.logger.(*logger.NoopLogger); workNode.logger == nil || ok {
		workNode.SetLogger(tr.logger)
	}

	tr.graph.Add(workNode, dependencies)

	// Convert dependencies to strings for JSON
	depStrings := make([]string, len(dependencies))
	for i, dep := range dependencies {
		depStrings[i] = dep.String()
	}

	// Emit node created event
	tr.emitEvent(EventNodeCreated, workNode.ID, map[string]any{
		"dependencies":     depStrings,
		"estimated_tokens": workNode.GetEstimatedTokens(),
		"provider":         string(workNode.GetProvider()),
	})

	return tr
}

// WaitFor waits for the result of a work node
func (tr *TurboRun) WaitFor(workNode *WorkNode) RunResult {
	return workNode.ListenForResult()
}

// OverrideBudgetsForTests overrides the budgets for the consumption tracker (used primarily for testing)
func (tr *TurboRun) OverrideBudgetsForTests(groqBudgetTokens int, openaiBudgetTokens int, groqBudgetRequests int, openaiBudgetRequests int) {
	tr.tracker.SetBudgetsForTests(groqBudgetTokens, openaiBudgetTokens, groqBudgetRequests, openaiBudgetRequests)
}

// GetStats returns the stats of the turbo run
func (tr *TurboRun) GetStats() *TurboRunStats {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return &TurboRunStats{
		GraphSize:         tr.graph.Size(),
		PriorityQueueSize: tr.priorityQueue.Size(),
		LaunchpadSize:     len(tr.launchpad),
		LaunchedCount:     tr.launchedCount,
		CompletedCount:    tr.completedCount,
		FailedCount:       tr.failedCount,
		WorkersPoolSize:   tr.workersPool.GetWorkerCount(),
		WorkersPoolBusy:   tr.workersPool.GetBusyWorkers(),
		TrackerStats:      tr.tracker.GetStats(),
	}
}

// listenForReadyNodes listens for ready nodes published from the Graph and pushes them into the priority queue
func (tr *TurboRun) listenForReadyNodes() {
	for {
		select {
		case <-tr.quit:
			return // Shutdown signal

		case node := <-tr.graph.readyNodesChan:
			// Emit node ready event
			tr.emitEvent(EventNodeReady, node.ID, map[string]any{
				"estimated_tokens": node.GetEstimatedTokens(),
			})

			tr.priorityQueue.Push(&priority_queue.QueueItem[*WorkNode]{
				Item:     node,
				Priority: node.GetEstimatedTokens(),
			})

			// Emit node prioritized event
			tr.emitEvent(EventNodePrioritized, node.ID, map[string]any{
				"priority":   node.GetEstimatedTokens(),
				"queue_size": tr.priorityQueue.Size(),
			})

			// Wait for the PQ to get re-organized
			time.Sleep(5 * time.Millisecond)
			tr.launchpad <- node
		}
	}
}

// listenForLaunchPad reads the value being put on the launchpad and sends them off to the workers pool
func (tr *TurboRun) listenForLaunchPad() {
	for {
		select {
		case <-tr.quit:
			return // Shutdown signal
		case <-tr.launchpad:

			if tr.priorityQueue.Size() == 0 {
				continue
			}

			node, _ := tr.priorityQueue.Pop()

			// Wait until we have enough budget for this request
			blocked := false
		budgetWaitLoop:
			for {
				tokensBudget, requestBudget := tr.tracker.BudgetAvailableForCycle(node.GetProvider())
				if tokensBudget >= node.GetEstimatedTokens() && requestBudget >= 1 {
					break // We have enough budget, proceed
				}

				// Emit blocking event only once per node
				if !blocked {
					blocked = true
					tr.emitEvent(EventBudgetBlocked, node.ID, map[string]any{
						"provider":           providerName(node.GetProvider()),
						"needed_tokens":      node.GetEstimatedTokens(),
						"available_tokens":   tokensBudget,
						"available_requests": requestBudget,
						"time_until_reset":   tr.tracker.TimeUntilReset().String(),
					})
				}

				// Not enough budget, wait until cycle reset (but check for quit signal)
				randomStagger := time.Duration(rand.Intn(100)) * time.Millisecond
				waitTime := tr.tracker.TimeUntilReset() + randomStagger

				select {
				case <-tr.quit:
					break budgetWaitLoop
				case <-time.After(waitTime):
					// Continue waiting
				}
			}

			tr.tracker.RecordConsumption(node.GetProvider(), node.GetEstimatedTokens())

			// Emit budget consumption event
			tokensAvailable, requestsAvailable := tr.tracker.BudgetAvailableForCycle(node.GetProvider())
			totalTokens := tr.getBudgetTotal(node.GetProvider())
			utilizationPct := float64(totalTokens-tokensAvailable) / float64(totalTokens) * 100

			tr.emitEvent(EventBudgetConsumed, uuid.Nil, map[string]any{
				"provider":           providerName(node.GetProvider()),
				"tokens_consumed":    node.GetEstimatedTokens(),
				"tokens_available":   tokensAvailable,
				"requests_available": requestsAvailable,
				"tokens_total":       totalTokens,
				"utilization_pct":    utilizationPct,
			})

			// Emit budget warning if utilization is high
			if utilizationPct >= 80 {
				tr.emitEvent(EventBudgetWarning, uuid.Nil, map[string]any{
					"provider":         providerName(node.GetProvider()),
					"utilization_pct":  utilizationPct,
					"tokens_available": tokensAvailable,
				})
			}

			// Emit node dispatched event
			tr.emitEvent(EventNodeDispatched, node.ID, map[string]any{
				"worker_pool_busy": tr.workersPool.GetBusyWorkers(),
				"worker_pool_size": tr.workersPool.GetWorkerCount(),
			})

			tr.workersPool.Dispatch(node)
			tr.removeNodeFromGraphOnCompletion(node)

			tr.mu.Lock()
			tr.launchedCount++
			tr.mu.Unlock()
		}
	}
}

// removeNodeFromGraphOnCompletion removes a node from the graph after it has completed
func (tr *TurboRun) removeNodeFromGraphOnCompletion(node *WorkNode) {
	node.AddResultCallback(func(result RunResult) {
		// Emit completion or failure event
		if result.Error != nil {
			tr.emitEvent(EventNodeFailed, node.ID, map[string]any{
				"error":    result.Error.Error(),
				"duration": result.Duration.String(),
			})
			tr.mu.Lock()
			tr.failedCount++
			tr.mu.Unlock()
		} else {
			tr.emitEvent(EventNodeCompleted, node.ID, map[string]any{
				"duration":    result.Duration.String(),
				"tokens_used": result.TokensUsed,
			})
			tr.mu.Lock()
			tr.completedCount++
			tr.mu.Unlock()
		}

		tr.graph.Remove(node.ID)
	})
}

// listenForWorkerStateChanges listens for worker state changes and broadcasts stats
func (tr *TurboRun) listenForWorkerStateChanges() {
	for {
		select {
		case <-tr.quit:
			return // Shutdown signal
		case <-tr.workerStateChan:
			// Worker state changed, broadcast updated stats
			// The actual broadcasting will be handled by the server via GetStats()
			// We just need to trigger a stats update event
			tr.emitEvent("worker_state_changed", uuid.Nil, map[string]any{
				"workers_busy": tr.workersPool.GetBusyWorkers(),
			})
		}
	}
}

// startMinuteTimer starts a timer that will call onMinuteChange every minute
func (tr *TurboRun) startMinuteTimer() {
	// Calculate time until next minute boundary
	now := time.Now()
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	timeUntilNextMinute := nextMinute.Sub(now)

	// Wait until next minute boundary
	time.Sleep(timeUntilNextMinute)

	// Now start ticker for every minute
	ticker := time.NewTicker(time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-tr.quit:
				return // Shutdown signal
			case <-ticker.C:
				tr.onMinuteChange()
			}
		}
	}()
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

// startAnalyticsLogger starts a timer that logs stats every 20 seconds
func (tr *TurboRun) startAnalyticsLogger() {
	ticker := time.NewTicker(20 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-tr.quit:
				tr.logStats(true)
				return // Shutdown signal
			case <-ticker.C:
				tr.logStats(false)
			}
		}
	}()
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

	// Only log if there's activity (busy workers, items in queue, or recent launches)
	if stats.WorkersPoolBusy > 0 || stats.PriorityQueueSize > 0 || stats.LaunchedCount > 0 {
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
		return fmt.Sprintf("%.1fM", float64(tokens)/1000000)
	} else if tokens >= 1000 {
		return fmt.Sprintf("%.1fK", float64(tokens)/1000)
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

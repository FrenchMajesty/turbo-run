package turbo_run

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/FrenchMajesty/turbo-run/clients/groq"
	"github.com/FrenchMajesty/turbo-run/rate_limit/backends/uds"
	"github.com/FrenchMajesty/turbo-run/ratelimit"
	"github.com/FrenchMajesty/turbo-run/utils/priority_queue"
	"github.com/google/uuid"
	openai "github.com/openai/openai-go/v2"
)

type EventType string

const (
	EventNodeCreated     EventType = "node_created"
	EventNodeReady       EventType = "node_ready"
	EventNodePrioritized EventType = "node_prioritized"
	EventNodeDispatched  EventType = "node_dispatched"
	EventNodeRunning     EventType = "node_running"
	EventNodeRetrying    EventType = "node_retrying"
	EventNodeCompleted   EventType = "node_completed"
	EventNodeFailed      EventType = "node_failed"
)

type Event struct {
	Type      EventType              `json:"type"`
	NodeID    string                 `json:"node_id"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

type TurboRun struct {
	// Core components
	graph         *Graph
	priorityQueue *priority_queue.PriorityQueue[*WorkNode]
	tracker       *consumptionTracker
	workersPool   *workerPool

	// Channels
	quit      chan struct{}
	launchpad chan any
	eventChan chan *Event

	// Misc
	mu         sync.RWMutex // protects launchedCount and stats reading
	fileLogger *log.Logger  // for logging to file
	logFile    *os.File     // file handle for locking

	// Stats attributes
	uniqueID      string
	startTime     time.Time
	failedCount   int
	launchedCount int
}

type TurboRunStats struct {
	GraphSize         int
	PriorityQueueSize int
	LaunchpadSize     int
	LaunchedCount     int
	FailedCount       int
	WorkersPoolSize   int
	WorkersPoolBusy   int
	TrackerStats      *ConsumptionTrackerStats
}

var instance *TurboRun
var once sync.Once

func NewTurboRun(
	groq groq.GroqClientInterface,
	openai *openai.Client,
) *TurboRun {
	return NewTurboRunWithBackend(groq, openai, nil)
}

// NewTurboRunWithBackend creates a new TurboRun instance with a custom rate limit backend.
// If backend is nil, defaults to UDS backend for cross-process coordination.
func NewTurboRunWithBackend(
	groq groq.GroqClientInterface,
	openai *openai.Client,
	backend ratelimit.Backend,
) *TurboRun {
	once.Do(func() {
		// Setup file logger
		env := os.Getenv("ENV")
		uniqueID := uuid.New().String()[:6]
		fileLogger, logFile := prepareFileLogger(env, uniqueID)

		// Default to UDS backend for cross-process coordination if none provided
		if backend == nil {
			backend = uds.NewClient()
		}

		instance = &TurboRun{
			uniqueID:      uniqueID,
			graph:         NewGraph(),
			priorityQueue: priority_queue.NewMaxPriorityQueue[*WorkNode](),
			tracker:       NewConsumptionTracker(backend),
			quit:          make(chan struct{}),
			workersPool:   NewWorkerPool(120, &groq, openai),
			launchpad:     make(chan any, 100),
			eventChan:     make(chan *Event, 1000),
			fileLogger:    fileLogger,
			logFile:       logFile,
			startTime:     time.Now(),
		}

		instance.Start()

		// Log initial startup in dev/testing
		if env == "dev" || env == "testing" {
			fmt.Printf("TurboRun %s: Started with %d workers\n", instance.uniqueID, 120)
			instance.logToFileWithLockf("TurboRun %s: Started with %d workers", instance.uniqueID, 120)
		} else {
			// Always log startup to file, regardless of environment, to track all instances
			instance.logToFileWithLockf("TurboRun %s: Started with %d workers (env=%s)", instance.uniqueID, 120, env)
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
	tr.logToFileWithLockf("TurboRun %s: Shutting down", tr.uniqueID)

	if tr.logFile != nil {
		tr.logFile.Close()
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
func (tr *TurboRun) emitEvent(eventType EventType, nodeID uuid.UUID, data map[string]interface{}) {
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
	tr.graph.Add(workNode, []uuid.UUID{})

	// Emit node created event
	tr.emitEvent(EventNodeCreated, workNode.ID, map[string]interface{}{
		"dependencies":     []string{},
		"estimated_tokens": workNode.GetEstimatedTokens(),
		"provider":         tr.providerToString(workNode.GetProvider()),
	})

	return tr
}

// PushWithDependencies adds a work node to the graph with dependencies
func (tr *TurboRun) PushWithDependencies(workNode *WorkNode, dependencies []uuid.UUID) *TurboRun {
	tr.graph.Add(workNode, dependencies)

	// Convert dependencies to strings for JSON
	depStrings := make([]string, len(dependencies))
	for i, dep := range dependencies {
		depStrings[i] = dep.String()
	}

	// Emit node created event
	tr.emitEvent(EventNodeCreated, workNode.ID, map[string]interface{}{
		"dependencies":     depStrings,
		"estimated_tokens": workNode.GetEstimatedTokens(),
		"provider":         tr.providerToString(workNode.GetProvider()),
	})

	return tr
}

// providerToString converts Provider to string
func (tr *TurboRun) providerToString(p Provider) string {
	switch p {
	case ProviderGroq:
		return "groq"
	case ProviderOpenAI:
		return "openai"
	default:
		return "unknown"
	}
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
			tr.emitEvent(EventNodeReady, node.ID, map[string]interface{}{
				"estimated_tokens": node.GetEstimatedTokens(),
			})

			tr.priorityQueue.Push(&priority_queue.QueueItem[*WorkNode]{
				Item:     node,
				Priority: node.GetEstimatedTokens(),
			})

			// Emit node prioritized event
			tr.emitEvent(EventNodePrioritized, node.ID, map[string]interface{}{
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
				return
			}

			node, _ := tr.priorityQueue.Pop()

			// Wait until we have enough budget for this request
			for {
				tokensBudget, requestBudget := tr.tracker.BudgetAvailableForCycle(node.GetProvider())
				if tokensBudget >= node.GetEstimatedTokens() && requestBudget >= 1 {
					break // We have enough budget, proceed
				}

				// Not enough budget, wait until cycle reset
				randomStagger := time.Duration(rand.Intn(100)) * time.Millisecond
				time.Sleep(tr.tracker.TimeUntilReset() + randomStagger)
			}

			tr.tracker.RecordConsumption(node.GetProvider(), node.GetEstimatedTokens())

			// Emit node dispatched event
			tr.emitEvent(EventNodeDispatched, node.ID, map[string]interface{}{
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
			tr.emitEvent(EventNodeFailed, node.ID, map[string]interface{}{
				"error":    result.Error.Error(),
				"duration": result.Duration.String(),
			})
			tr.mu.Lock()
			tr.failedCount++
			tr.mu.Unlock()
		} else {
			tr.emitEvent(EventNodeCompleted, node.ID, map[string]interface{}{
				"duration":    result.Duration.String(),
				"tokens_used": result.TokensUsed,
			})
		}

		tr.graph.Remove(node.ID)
	})
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
		logMessage := fmt.Sprintf(
			"TurboRun %s: Shutting down. Total nodes launched: %d. Total nodes failed: %d. Total tokens used: %s. Requests made: %d. Time taken: %s",
			tr.uniqueID,
			stats.LaunchedCount,
			stats.FailedCount,
			formatTokens(stats.TrackerStats.TotalTokens),
			stats.TrackerStats.TotalRequests,
			time.Since(tr.startTime),
		)

		fmt.Println(logMessage)
		tr.logToFileWithLock(logMessage)

		return
	}

	// Only log if there's activity (busy workers, items in queue, or recent launches)
	if stats.WorkersPoolBusy > 0 || stats.PriorityQueueSize > 0 || stats.LaunchedCount > 0 {
		trackerStats := stats.TrackerStats
		logMessage := fmt.Sprintf("TurboRun %s: Workers(%d/%d) Queue(%d) Graph(%d) Launched(%d) Failed(%d) Tokens(groq:%s openai:%s total:%s)",
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

		// Log to terminal
		fmt.Println(logMessage)

		// Log to file if available
		tr.logToFileWithLock(logMessage)
	}
}

// logToFileWithLock writes to the log file with file-level locking for cross-process safety
func (tr *TurboRun) logToFileWithLock(message string) {
	if tr.fileLogger == nil || tr.logFile == nil {
		return
	}

	// Acquire exclusive file lock (blocks until available)
	err := syscall.Flock(int(tr.logFile.Fd()), syscall.LOCK_EX)
	if err != nil {
		// If we can't get the lock, skip logging to avoid blocking
		return
	}
	defer syscall.Flock(int(tr.logFile.Fd()), syscall.LOCK_UN) // Release lock

	// Write to log
	tr.fileLogger.Println(message)
}

// logToFileWithLockf writes formatted message to the log file with file-level locking
func (tr *TurboRun) logToFileWithLockf(format string, args ...interface{}) {
	if tr.fileLogger == nil || tr.logFile == nil {
		return
	}

	// Acquire exclusive file lock (blocks until available)
	err := syscall.Flock(int(tr.logFile.Fd()), syscall.LOCK_EX)
	if err != nil {
		// If we can't get the lock, skip logging to avoid blocking
		return
	}
	defer syscall.Flock(int(tr.logFile.Fd()), syscall.LOCK_UN) // Release lock

	// Write to log
	tr.fileLogger.Printf(format, args...)
}

// prepareFileLogger prepares the file logger for the turbo run
func prepareFileLogger(env string, uniqueID string) (*log.Logger, *os.File) {
	filePath := filepath.Join(projectRoot(), "turbo_run.log")
	if env == "dev" || env == "testing" {
		// Delete old log file if it exists
		if _, err := os.Stat(filePath); err == nil {
			os.Remove(filePath)
		}

		if file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
			logger := log.New(file, "", log.LstdFlags)
			return logger, file
		}
	}

	return nil, nil
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

// projectRoot returns the root directory of the project in absolute path
func projectRoot() string {
	currentDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	for {
		goModPath := filepath.Join(currentDir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			break
		}

		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			panic(fmt.Errorf("go.mod not found"))
		}
		currentDir = parent
	}

	return currentDir
}

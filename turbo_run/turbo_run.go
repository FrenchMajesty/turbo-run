package turbo_run

import (
	"fmt"
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

// pushRequest represents a request to add a node to the graph
type pushRequest struct {
	node         *WorkNode
	dependencies []uuid.UUID
}

type TurboRun struct {
	// Core components
	graph         *Graph
	priorityQueue *priority_queue.PriorityQueue[*WorkNode]
	tracker       *consumptionTracker
	workersPool   *workerPool

	// Channels
	quit               chan struct{}
	launchpad          chan struct{}
	eventChan          chan *Event
	workerStateChan    chan int
	pushChan           chan *pushRequest // buffered channel for incoming graph nodes
	graphSpaceNotify   chan struct{}     // signals when graph space becomes available

	// Misc
	mu           sync.RWMutex   // protects launchedCount and stats reading
	wg           sync.WaitGroup // tracks goroutines for graceful shutdown
	logger       logger.Logger  // pluggable logger
	maxGraphSize int            // 0 = unlimited

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
	PushQueueSize     int
	LaunchedCount     int
	CompletedCount    int
	FailedCount       int
	WorkersPoolSize   int
	WorkersPoolBusy   int
	TrackerStats      *ConsumptionTrackerStats
}

var (
	instance *TurboRun
	once     sync.Once
)

// TurboRunOption is a functional option for configuring TurboRun
type TurboRunOption func(*TurboRun)

// WithLogger sets a custom logger for TurboRun
func WithLogger(l logger.Logger) TurboRunOption {
	return func(tr *TurboRun) {
		tr.logger = l
	}
}

// WithMaxGraphSize sets the maximum number of nodes allowed in the graph.
// When the graph is full, Push() will block until nodes complete.
// A value of 0 means unlimited (default).
func WithMaxGraphSize(size int) TurboRunOption {
	return func(tr *TurboRun) {
		tr.maxGraphSize = size
	}
}

// NewTurboRun creates a new singleton instance of TurboRun.
// Subsequent calls return the same instance.
func NewTurboRun(
	groqClient groq.GroqClientInterface,
	openaiClient *openai.Client,
	opts ...TurboRunOption,
) *TurboRun {
	return NewTurboRunWithBackend(groqClient, openaiClient, nil, opts...)
}

// NewTurboRunWithBackend creates a new TurboRun instance with a custom rate limit backend.
// If backend is nil, defaults to in-memory backend for cross-process coordination.
func NewTurboRunWithBackend(
	groqClient groq.GroqClientInterface,
	openaiClient *openai.Client,
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
			uniqueID:         uniqueID,
			graph:            NewGraph(),
			priorityQueue:    priority_queue.NewMaxPriorityQueue[*WorkNode](),
			tracker:          NewConsumptionTracker(backend),
			quit:             make(chan struct{}),
			launchpad:        make(chan struct{}, 100),
			eventChan:        make(chan *Event, 1000),
			workerStateChan:  workerStateChan,
			graphSpaceNotify: make(chan struct{}, 1), // Buffered to prevent blocking
			logger:           logger.NewStdoutLogger(), // Default to stdout
			maxGraphSize:     0,                        // Default unlimited
			startTime:        time.Now(),
		}

		// Apply functional options (may set maxGraphSize)
		for _, opt := range opts {
			opt(instance)
		}

		// Calculate push channel buffer size
		// Use maxGraphSize as buffer, or default to 1000 if unlimited
		pushBufferSize := instance.maxGraphSize
		if pushBufferSize == 0 {
			pushBufferSize = 1000 // Default buffer for unlimited graphs
		}
		instance.pushChan = make(chan *pushRequest, pushBufferSize)

		instance.workersPool = NewWorkerPool(120, &groqClient, openaiClient, workerStateChan)

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

// GetTurboRun returns the singleton instance of TurboRun.
// Returns an error if the instance hasn't been initialized yet via NewTurboRun.
func GetTurboRun() (*TurboRun, error) {
	if instance == nil {
		return nil, fmt.Errorf("turbo run instance is nil")
	}
	return instance, nil
}

// Push adds a work node to the graph with no dependencies.
// This will block if the graph is at max capacity (configured via WithMaxGraphSize).
func (tr *TurboRun) Push(workNode *WorkNode) *TurboRun {
	if workNode.logger == nil || workNode.logger.Type() == logger.LoggerTypeNoop {
		workNode.SetLogger(tr.logger)
	}

	// Send to push channel (blocks if channel is full - natural backpressure)
	tr.pushChan <- &pushRequest{
		node:         workNode,
		dependencies: []uuid.UUID{},
	}

	// Small sleep to allow async processing to complete
	// This maintains quasi-synchronous behavior for tests
	time.Sleep(1 * time.Millisecond)

	return tr
}

// PushWithDependencies adds a work node to the graph with dependencies.
// This will block if the graph is at max capacity (configured via WithMaxGraphSize).
func (tr *TurboRun) PushWithDependencies(workNode *WorkNode, dependencies []uuid.UUID) *TurboRun {
	if workNode.logger == nil || workNode.logger.Type() == logger.LoggerTypeNoop {
		workNode.SetLogger(tr.logger)
	}

	// Send to push channel (blocks if channel is full - natural backpressure)
	tr.pushChan <- &pushRequest{
		node:         workNode,
		dependencies: dependencies,
	}

	// Small sleep to allow async processing to complete
	// This maintains quasi-synchronous behavior for tests
	time.Sleep(1 * time.Millisecond)

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

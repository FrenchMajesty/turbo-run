package turbo_run

import (
	"fmt"
	"math"
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
	quit             chan struct{}
	launchpad        chan struct{}
	eventChan        chan *Event
	workerStateChan  chan int
	pushChan         chan *pushRequest // buffered channel for incoming graph nodes
	graphSpaceNotify chan struct{}     // signals when graph space becomes available

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

// Options contains configuration options for TurboRun
type Options struct {
	// GroqClient is the Groq API client (required)
	GroqClient groq.GroqClientInterface

	// OpenAIClient is the OpenAI API client (required)
	OpenAIClient *openai.Client

	// Backend is the rate limit backend for cross-process coordination
	// If nil, defaults to in-memory backend
	Backend rate_limit.Backend

	// Logger is the logger instance to use
	// If nil, defaults to stdout logger
	Logger logger.Logger

	// MaxGraphSize limits the number of nodes allowed in the graph to prevent unbounded memory usage. 0 = unlimited. Default is 500K
	MaxGraphSize int

	// WorkerPoolSize is the number of workers to use in the worker pool
	WorkerPoolSize int
}

// NewTurboRun creates a new singleton instance of TurboRun.
// Subsequent calls return the same instance.
func NewTurboRun(opts Options) *TurboRun {
	once.Do(func() {
		uniqueID := uuid.New().String()[:6]

		// Apply defaults
		if opts.Backend == nil {
			opts.Backend = memory.NewBackend()
		}

		if opts.Logger == nil {
			opts.Logger = logger.NewStdoutLogger()
		}

		if opts.MaxGraphSize <= 0 {
			opts.MaxGraphSize = 500_000 // Default max graph size
		}

		if opts.WorkerPoolSize <= 0 {
			opts.WorkerPoolSize = 120
		}

		// Calculate buffer sizes
		pushBufferSize := opts.MaxGraphSize
		if pushBufferSize == 0 {
			pushBufferSize = 1000 // Default buffer for unlimited graphs
		}

		eventChannelSize := int(math.Max(1000, float64(opts.WorkerPoolSize*10))) // 1K or 10 times the worker pool size, whichever is greater
		workerStateChan := make(chan int, opts.WorkerPoolSize*2)                 // 2 events per worker (busy/idle), channel is pass-through for state

		instance = &TurboRun{
			uniqueID:         uniqueID,
			graph:            NewGraph(opts.MaxGraphSize),
			priorityQueue:    priority_queue.NewMaxPriorityQueue[*WorkNode](),
			tracker:          NewConsumptionTracker(opts.Backend),
			quit:             make(chan struct{}),
			launchpad:        make(chan struct{}, opts.WorkerPoolSize), // Makes nodes available to the worker pool
			eventChan:        make(chan *Event, eventChannelSize),      // ~10 events per node, channel is pass-through for observability
			workerStateChan:  workerStateChan,
			graphSpaceNotify: make(chan struct{}, 1), // Buffered to prevent blocking
			pushChan:         make(chan *pushRequest, pushBufferSize),
			logger:           opts.Logger,
			maxGraphSize:     opts.MaxGraphSize,
			startTime:        time.Now(),
		}

		instance.workersPool = NewWorkerPool(
			opts.WorkerPoolSize,
			&opts.GroqClient,
			opts.OpenAIClient,
			workerStateChan,
		)

		instance.Start()
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

	// Send to push channel (blocks if channel is full)
	tr.pushChan <- &pushRequest{
		node:         workNode,
		dependencies: []uuid.UUID{},
	}

	// Small sleep to allow async processing to complete
	// This maintains quasi-synchronous behavior for tests
	time.Sleep(time.Nanosecond)

	return tr
}

// PushWithDependencies adds a work node to the graph with dependencies.
// This will block if the graph is at max capacity (configured via WithMaxGraphSize).
func (tr *TurboRun) PushWithDependencies(workNode *WorkNode, dependencies []uuid.UUID) *TurboRun {
	if workNode.logger == nil || workNode.logger.Type() == logger.LoggerTypeNoop {
		workNode.SetLogger(tr.logger)
	}

	// Send to push channel (blocks if channel is full)
	tr.pushChan <- &pushRequest{
		node:         workNode,
		dependencies: dependencies,
	}

	// Small sleep to allow async processing to complete
	// This maintains quasi-synchronous behavior for tests
	time.Sleep(time.Nanosecond)

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

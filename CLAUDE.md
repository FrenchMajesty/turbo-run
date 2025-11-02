# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TurboRun is a concurrent task execution engine for orchestrating API requests to LLM providers (Groq and OpenAI) while respecting rate limits and managing task dependencies. It uses a dependency graph, priority queue, and worker pool architecture to maximize throughput.

## Development Commands

### Testing
```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./turbo_run
go test ./utils/parallel
go test ./utils/priority_queue
go test ./utils/token_counter

# Run specific test
go test -run TestTurboRun_Singleton ./turbo_run

# Coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Building
```bash
# Build the library (no main.go - this is a library package)
go build ./...

# Run the rate limit manager server (for cross-process coordination)
go run . rate-limiter
```

## Core Architecture

### Singleton Pattern
TurboRun uses a singleton pattern initialized via `NewTurboRun(Options{...})`. Once created, subsequent calls return the same instance. Access via `GetTurboRun()` after initialization, which returns `(*TurboRun, error)` - returns an error if the instance hasn't been initialized yet.

### Initialization and Configuration

TurboRun uses options-based configuration:

```go
turboRun := turbo_run.NewTurboRun(turbo_run.Options{
    GroqClient:              mockGroq,           // Required if using Groq
    OpenAIClient:            mockOpenAI,          // Required if using OpenAI
    WorkerPoolSize:          120,                 // Default: 120
    MaxGraphSize:            500000,              // Default: 500K nodes
    Logger:                  customLogger,        // Default: stdout
    RateLimitBackend:        customBackend,       // Default: memory backend
    FailureHandlingStrategy: FailureStrategyPropagate, // Default: propagate
})
```

**IMPORTANT:** TurboRun starts in **PAUSED** state by default. You must call `turboRun.Start()` to begin processing nodes.

### Main Components

1. **Graph** ([turbo_run/graph.go](turbo_run/graph.go))
   - Manages WorkNode dependencies using directed acyclic graph (DAG)
   - Tracks indegree for each node to determine when dependencies are satisfied
   - Publishes ready nodes to `readyNodesChan` when indegree reaches 0
   - Thread-safe with RWMutex protection

2. **WorkNode** ([turbo_run/work_node.go](turbo_run/work_node.go))
   - Represents a unit of work (LLM API request)
   - Supports two providers: `ProviderGroq` and `ProviderOpenAI`
   - Has built-in retry logic with exponential backoff (configurable via `RetryConfig`)
   - Tracks status: Pending → Running → Completed/Failed
   - Uses channels for result delivery and status updates

3. **Priority Queue** ([utils/priority_queue/](utils/priority_queue/))
   - Max-priority queue that orders WorkNodes by estimated token consumption
   - Larger token requests get processed first when budget allows
   - Generic implementation supporting any item type with priority

4. **Worker Pool** ([turbo_run/worker_pool.go](turbo_run/worker_pool.go))
   - Manages 120 concurrent workers by default
   - Workers execute WorkNode `workFn` concurrently via goroutines
   - Tracks busy/idle state of each worker
   - Graceful shutdown via quit channel

5. **Consumption Tracker** ([turbo_run/consumption_tracker.go](turbo_run/consumption_tracker.go))
   - Tracks token and request consumption per provider per minute
   - Enforces rate limits: Groq (360K TPM, 1K RPM), OpenAI (9M TPM, 10K RPM) - with 10% safety buffers
   - Automatically cycles/resets budgets every 60 seconds
   - Coordinates with backend (memory or UDS) using minimum of local/backend budgets during normal operation
   - Special handling during minute transitions (seconds 0-2 and 58-60) to prevent false blocking

6. **Rate Limit Backends** ([rate_limit/backend.go](rate_limit/backend.go))
   - Pluggable backend architecture for rate limiting
   - **Memory Backend** ([rate_limit/backends/memory/](rate_limit/backends/memory/)): Single-process tracking (default)
   - **UDS Backend** ([rate_limit/backends/uds/](rate_limit/backends/uds/)): Unix Domain Socket for cross-process coordination
   - Backend interface supports future implementations (Redis, etc.)

7. **Logger System** ([utils/logger/](utils/logger/))
   - 6 logger types: Stdout, File, Noop, Writer, Multi, with type identification
   - File logger uses `syscall.Flock` for safe concurrent writes across processes
   - MultiLogger writes to multiple destinations simultaneously
   - Default: stdout logger, configurable via Options

### Execution Flow

1. WorkNode pushed via `Push()` or `PushWithDependencies()` → sent to `pushChan`
2. `listenForWorkNodePushRequests()` goroutine:
   - Enforces graph size limit (`MaxGraphSize`)
   - Blocks on `graphSpaceNotify` when full
   - Adds node to Graph
   - Emits: `EventNodeCreated`, `EventGraphFull`/`EventGraphResumed`
3. Graph publishes to `readyNodesChan` when dependencies satisfied (indegree = 0)
4. `listenForGraphReadyNodes()` goroutine:
   - Adds ready nodes to PriorityQueue
   - Signals `launchpad` channel
   - Emits: `EventNodeReady`, `EventPriorityQueueAdd`
5. `listenForLaunchPad()` goroutine (only if not paused):
   - Pops highest priority node from queue
   - Waits for sufficient rate limit budget (may block)
   - Records consumption with backend
   - Dispatches to WorkerPool
   - Emits: `EventPriorityQueueRemove`, `EventBudgetBlocked`, `EventBudgetConsumed`, `EventBudgetWarning`, `EventNodeDispatched`
6. Worker executes `workFn`:
   - Updates status to Running
   - Executes with retry logic if configured
   - Emits: `EventNodeRunning`, `EventNodeRetrying`, `EventNodeCompleted`/`EventNodeFailed`
7. Node removed from Graph (or subtree removed on failure), triggering dependent nodes to become ready

### Lifecycle Management

TurboRun supports full lifecycle control:

```go
turboRun.Start()      // Begin/resume processing (required after creation)
turboRun.Pause()      // Pause dispatch (nodes continue to queue)
turboRun.IsPaused()   // Check pause state
turboRun.Reset()      // Cancel all work, clear graph/queues, emit EventNodeCancelled
turboRun.Stop()       // Graceful shutdown, wait for workers
```

**Lifecycle States:**
- **Created (Paused)** - Default state, nodes queue but don't dispatch
- **Started/Running** - Processing nodes from queue
- **Paused** - Temporarily suspended, nodes continue to queue
- **Stopped** - Shutdown complete, resources cleaned up

### Concurrency Model

- **5 control goroutines**:
  1. `listenForGraphReadyNodes()` - Graph → PriorityQueue
  2. `listenForLaunchPad()` - PriorityQueue → WorkerPool (with rate limiting)
  3. `startMinuteTimer()` - Budget cycling every 60s
  4. `listenForWorkerStateChanges()` - Broadcast worker state updates
  5. `listenForWorkNodePushRequests()` - Backpressure-controlled graph insertion
- **120 worker goroutines** (default): Process WorkNodes concurrently
- **Total: 125 goroutines** per TurboRun instance (5 control + 120 workers)
- All components use channels and mutexes for thread-safety

## Working with WorkNodes

### Creating WorkNodes
```go
// Basic Groq request
node := NewWorkNodeForGroq(groq.ChatCompletionRequest{...})

// Basic OpenAI request
node := NewWorkNodeForOpenAI(openai.ChatCompletionNewParams{...})

// With retry support (default: 3 retries with exponential backoff)
node := NewRetryableWorkNodeForGroq(groq.ChatCompletionRequest{...})

// Custom retry config
node := NewRetryableWorkNodeForGroq(req).SetRetryConfig(RetryConfig{
    MaxRetries:      5,
    BaseDelay:       100 * time.Millisecond,
    MaxDelay:        10 * time.Second,
    BackoffMultiple: 2.0,
})
```

### Custom Work Functions
```go
node := NewWorkNodeForGroq(req)
node.SetWorkFn(func(w *WorkNode, groq *groq.GroqClientInterface, openai *openai.Client) RunResult {
    // Custom logic here
    return RunResult{...}
})

// With retry wrapper
node.SetWorkFnWithRetry(customWorkFn)
```

### Dependency Patterns
```go
// No dependencies
turboRun.Push(node1)

// Single dependency
turboRun.PushWithDependencies(node2, []uuid.UUID{node1.ID})

// Multiple dependencies (fan-in)
turboRun.PushWithDependencies(aggregateNode, []uuid.UUID{node1.ID, node2.ID, node3.ID})

// Waiting for results
result := turboRun.WaitFor(node1) // Blocks until complete
if result.Error != nil {
    // Handle error
}
```

## Observability System

### Event Types

TurboRun emits 17 event types for complete observability:

**Node Lifecycle:**
- `EventNodeCreated` - Node added to graph with dependencies
- `EventNodeReady` - Dependencies satisfied, added to priority queue
- `EventNodeDispatched` - Sent to worker pool
- `EventNodeRunning` - Worker started execution (includes worker_id)
- `EventNodeRetrying` - Retry attempt with delay info
- `EventNodeCompleted` - Success with duration/tokens
- `EventNodeFailed` - Failure with error message
- `EventNodeCancelled` - Cancelled due to parent failure or reset

**Priority Queue:**
- `EventPriorityQueueAdd` - Node added to queue
- `EventPriorityQueueRemove` - Node popped from queue

**Rate Limit Budget:**
- `EventBudgetConsumed` - Token consumption recorded (includes utilization %)
- `EventBudgetBlocked` - Waiting for budget reset
- `EventBudgetReset` - Minute boundary, budgets refreshed
- `EventBudgetWarning` - Utilization >= 80%

**Graph Capacity:**
- `EventGraphFull` - Max size reached, blocking pushes
- `EventGraphResumed` - Space available after blocking

### Consuming Events

```go
eventChan := turboRun.GetEventChan()
for event := range eventChan {
    // Event structure:
    // - Type: EventType
    // - NodeID: string (UUID)
    // - Timestamp: time.Time
    // - Data: map[string]any (event-specific data)
}
```

**IMPORTANT:** Events use non-blocking sends. If the channel is full, events will be dropped to prevent blocking the engine.

### Stats API

```go
stats := turboRun.GetStats()
// Returns TurboRunStats with:
// - GraphSize, PriorityQueueSize, PriorityQueueSnapshot
// - LaunchpadSize, PushQueueSize
// - LaunchedCount, CompletedCount, FailedCount
// - WorkersPoolSize, WorkersPoolBusy, WorkerStates
// - TrackerStats (consumption data per provider)
```

## Failure Handling Strategies

TurboRun supports two failure handling modes:

### Propagate Mode (Default)

Failed nodes cascade failures to all descendants:

```go
turboRun := NewTurboRun(Options{
    FailureHandlingStrategy: FailureStrategyPropagate, // Default
})
```

- Failed node triggers `graph.RemoveSubtree(nodeID)`
- Recursively cancels ALL descendants
- Emits `EventNodeCancelled` for each child with reason "parent_failure"
- Use when: Dependencies are critical and failures should stop dependent work

### Isolate Mode

Failed nodes don't cascade failures:

```go
turboRun := NewTurboRun(Options{
    FailureHandlingStrategy: FailureStrategyIsolate,
})
```

- Failed node removed individually
- Dependent children proceed normally (may fail independently if they need parent's result)
- Use when: Independent retry paths or failures shouldn't cascade

## Token Estimation

Token counting uses `tiktoken-go` library ([utils/token_counter/](utils/token_counter/)). WorkNodes automatically estimate tokens for budget management:
- Groq requests: counted + 20% overhead for response
- OpenAI requests: JSON-marshaled body counted

## Logger Configuration

Multiple logger types available:

```go
// Default: stdout
turboRun := NewTurboRun(Options{})

// File logger (thread-safe across processes)
turboRun := NewTurboRun(Options{
    Logger: logger.NewFileLogger("turbo_run.log"),
})

// Multi-destination logging
turboRun := NewTurboRun(Options{
    Logger: logger.NewMultiLogger(
        logger.NewFileLogger("turbo_run.log"),
        logger.NewStdoutLogger(),
    ),
})

// Custom logger (implements Logger interface)
turboRun := NewTurboRun(Options{
    Logger: customLogger,
})
```

**Logger Types:**
- `StdoutLogger` - Console output
- `FileLogger` - File with `syscall.Flock` for cross-process safety
- `NoopLogger` - Silent
- `WriterLogger` - Custom io.Writer
- `MultiLogger` - Multiple destinations

## Rate Limit Backends

### Memory Backend (Default)

Single-process tracking, no overhead:

```go
turboRun := NewTurboRun(Options{}) // Uses memory backend by default
```

### UDS Backend (Cross-Process)

Unix Domain Socket for multi-process coordination:

```go
import "github.com/FrenchMajesty/turbo-run/rate_limit/backends/uds"

turboRun := NewTurboRun(Options{
    RateLimitBackend: uds.NewBackend(),
})
```

**UDS Manager:** Run `go run . rate-limiter` to start the manager process. Clients automatically connect and start the manager if not running.

### Custom Backend

Implement the `rate_limit.Backend` interface:

```go
type Backend interface {
    BudgetAvailable(provider) (tokens, requests int)
    RecordConsumption(provider, tokens, requests) error
    TimeUntilReset() time.Duration
    SetBudgetForTests(provider, tokens, requests) error
    Close() error
}
```

## Testing Strategy

### Mock Clients

```go
type MockGroqClient struct{}

func (m *MockGroqClient) ChatCompletion(ctx context.Context, req groq.ChatCompletionRequest) (*groq.ChatCompletionResponse, error) {
    // Mock implementation
}
```

### Budget Overrides

```go
turboRun.OverrideBudgetsForTests(
    groqTokens,    // Groq TPM
    openaiTokens,  // OpenAI TPM
    groqRequests,  // Groq RPM
    openaiRequests, // OpenAI RPM
)
```

### Singleton Reset (Tests Only)

```go
// In turbo_run_test.go
instance = nil
once = sync.Once{}
```

### Key Test Areas

- Singleton behavior
- Buffer state progression (graph → queue → launchpad)
- Budget enforcement and blocking
- Node lifecycle states
- Worker pool utilization
- Buffer overflow handling (no node dropping)
- Failure propagation vs isolation strategies

## Utility Packages

- **parallel** ([utils/parallel/](utils/parallel/)): Builder pattern for concurrent operations with typed result collection
- **retry** ([utils/retry/](utils/retry/)): Exponential backoff retry utilities (documented in [utils/retry/doc.go](utils/retry/doc.go))
- **priority_queue** ([utils/priority_queue/](utils/priority_queue/)): Generic heap-based priority queue implementation
- **token_counter** ([utils/token_counter/](utils/token_counter/)): tiktoken-go wrapper for token estimation
- **logger** ([utils/logger/](utils/logger/)): Pluggable logging with 6 types

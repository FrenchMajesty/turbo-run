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
```

### Building
```bash
# Build the project
go build -o turbo-run .

# Run the main program
go run main.go
```

## Core Architecture

### Singleton Pattern
TurboRun uses a singleton pattern initialized via `NewTurboRun(groqClient, openaiClient)`. Once created, subsequent calls return the same instance. Access via `GetTurboRun()` after initialization, which returns `(*TurboRun, error)` - returns an error if the instance hasn't been initialized yet.

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
   - Enforces rate limits for Groq and OpenAI
   - Automatically cycles/resets budgets every 60 seconds
   - Provides stats on current usage and historical totals

6. **Rate Limiting** ([turbo_run/rate_limit.go](turbo_run/rate_limit.go))
   - Manages provider-specific rate limits (tokens/min, requests/min)
   - LaunchPad controller blocks WorkNodes until sufficient budget available
   - Uses randomized stagger after budget reset to avoid thundering herd

### Execution Flow

1. WorkNode pushed via `Push()` or `PushWithDependencies()` → added to Graph
2. Graph publishes to `readyNodesChan` when dependencies satisfied
3. `listenForReadyNodes()` goroutine adds ready nodes to PriorityQueue
4. Node signal sent to `launchpad` channel
5. `listenForLaunchPad()` goroutine:
   - Pops highest priority node from queue
   - Waits for sufficient rate limit budget
   - Records consumption and dispatches to WorkerPool
6. Worker executes `workFn`, emits result via channel
7. Node removed from Graph, triggering dependent nodes to become ready

### Concurrency Model

- **3 main goroutines**: ready node listener, launch pad controller, minute timer
- **120 worker goroutines**: process WorkNodes concurrently
- **Optional analytics logger**: logs stats every 20s in dev/testing environments
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

## Token Estimation

Token counting uses `tiktoken-go` library ([utils/token_counter/](utils/token_counter/)). WorkNodes automatically estimate tokens for budget management:
- Groq requests: counted + 20% overhead for response
- OpenAI requests: JSON-marshaled body counted

## Logging

- File logging to `turbo_run.log` in project root
- Uses file-level locking (`syscall.Flock`) for concurrent writes across processes
- Environment-aware: verbose in `dev`/`testing` modes
- Each TurboRun instance has 6-character unique ID for log tracking

## Testing Strategy

- Mock clients provided: `groq_mock.go`, `token_counter_mock.go`
- Test budget overrides via `OverrideBudgetsForTests()`
- Key test areas: singleton behavior, dependency resolution, rate limiting, worker utilization
- Use `testify` for assertions and mocks

## Utility Packages

- **parallel** ([utils/parallel/](utils/parallel/)): Builder pattern for concurrent operations with typed result collection
- **retry** ([utils/retry/](utils/retry/)): Exponential backoff retry utilities (documented in [utils/retry/doc.go](utils/retry/doc.go))
- **priority_queue** ([utils/priority_queue/](utils/priority_queue/)): Generic heap-based priority queue implementation

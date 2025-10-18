# TurboRun: Concurrent Task Execution Engine

TurboRun is a sophisticated task orchestration system designed to efficiently manage and execute API requests to multiple LLM providers while respecting rate limits and handling complex task dependencies.

## Core Concept

Think of TurboRun as a smart traffic controller for API requests. Instead of making requests one at a time, it allows you to queue up hundreds of tasks with dependencies between them, then executes them as efficiently as possible while staying within each provider's rate limits.

The system is built around the concept of **WorkNodes** - individual units of work that can depend on other WorkNodes. When you submit a WorkNode, TurboRun automatically figures out when it can be executed based on its dependencies and current rate limit budgets.

## How It Works

### The Flow

When you create a WorkNode and push it to TurboRun, here's what happens:

1. **Graph Registration**: The WorkNode gets added to an internal dependency graph along with any dependencies you specify. This graph keeps track of which tasks need to complete before others can start.

2. **Dependency Tracking**: The system monitors completed tasks and automatically identifies when dependencies are satisfied. When a WorkNode has no remaining dependencies, it becomes "ready" for execution.

3. **Priority Queuing**: Ready WorkNodes are placed into a priority queue, ordered by their estimated token consumption. This ensures that larger, more expensive requests get processed first when resources are available.

4. **Rate Limit Management**: Before dispatching any WorkNode, TurboRun checks whether there's sufficient budget remaining for that provider (Groq or OpenAI). Each provider has different rate limits for tokens per minute and requests per minute.

5. **Worker Pool Execution**: Once a WorkNode passes the budget check, it gets sent to a pool of 120 concurrent workers. Each worker handles the actual API call and manages the response.

6. **Result Delivery**: When a WorkNode completes, its result is made available through a channel-based system that allows you to wait for specific results without blocking other operations.

### Rate Limit Intelligence

One of TurboRun's key features is its intelligent rate limit management. The system tracks consumption in real-time and automatically resets budgets every minute to align with provider rate limits. If you try to execute a task that would exceed the current budget, TurboRun will wait until the next reset cycle rather than failing the request.

The consumption tracker maintains both current minute usage and historical data, giving you full visibility into your API consumption patterns. This helps with cost analysis and performance optimization.

### Concurrent Architecture

TurboRun runs several specialized goroutines that work together:

- **Ready Node Listener**: Monitors the dependency graph for newly available tasks
- **Launch Pad Controller**: Manages the flow from priority queue to worker pool, enforcing rate limits
- **Minute Timer**: Handles rate limit resets every 60 seconds
- **Worker Pool**: 120 concurrent workers that execute the actual API calls

This architecture allows TurboRun to handle hundreds of concurrent tasks while maintaining strict rate limit compliance and dependency ordering.

## API Usage

### Basic Example

```go
// Initialize TurboRun with your API clients
turboRun := NewTurboRun(groqClient, openaiClient)

// Create a WorkNode for initial processing using Groq
initRequest := groq.ChatCompletionRequest{
    Model: "llama-3.1-70b-versatile",
    Messages: []groq.ChatMessage{
        {Role: "user", Content: "Analyze this text..."},
    },
}
initNode := NewWorkNodeForGroq(initRequest)

// Push it to the system (no dependencies)
turboRun.Push(initNode, nil)

// Create a dependent task for scoring
scoreRequest := groq.ChatCompletionRequest{
    Model: "llama-3.1-8b-instant", 
    Messages: []groq.ChatMessage{
        {Role: "user", Content: "Score the analysis..."},
    },
}
scoreNode := NewWorkNodeForGroq(scoreRequest)

// This task depends on the first one completing
turboRun.Push(scoreNode, []uuid.UUID{initNode.ID})

// Wait for results (blocks until completion)
initialResult := turboRun.WaitFor(initNode)
if initialResult.Error != nil {
    fmt.Printf("Initial analysis failed: %v", initialResult.Error)
} else {
    fmt.Printf("Analysis completed in %v, used %d tokens", 
        initialResult.Duration, initialResult.TokensUsed)
}

scoreResult := turboRun.WaitFor(scoreNode)
```

### Complex Dependency Chains

TurboRun shines when handling complex workflows with multiple dependencies:

```go
// Create a fan-out pattern: one initial task spawns multiple parallel tasks
baseRequest := groq.ChatCompletionRequest{
    Model: "llama-3.1-70b-versatile",
    Messages: []groq.ChatMessage{
        {Role: "user", Content: "Perform base analysis on this dataset..."},
    },
}
baseNode := NewWorkNodeForGroq(baseRequest)
turboRun.Push(baseNode, nil)

// Multiple parallel tasks that all depend on the base analysis
var parallelNodes []*WorkNode
for i := 0; i < 10; i++ {
    parallelRequest := groq.ChatCompletionRequest{
        Model: "llama-3.1-8b-instant",
        Messages: []groq.ChatMessage{
            {Role: "user", Content: fmt.Sprintf("Process segment %d...", i)},
        },
    }
    node := NewWorkNodeForGroq(parallelRequest)
    turboRun.Push(node, []uuid.UUID{baseNode.ID})
    parallelNodes = append(parallelNodes, node)
}

// Final aggregation task that depends on all parallel tasks
var allIDs []uuid.UUID
for _, node := range parallelNodes {
    allIDs = append(allIDs, node.ID)
}

finalRequest := groq.ChatCompletionRequest{
    Model: "llama-3.1-70b-versatile", 
    Messages: []groq.ChatMessage{
        {Role: "user", Content: "Aggregate all the parallel results..."},
    },
}
finalNode := NewWorkNodeForGroq(finalRequest)
turboRun.Push(finalNode, allIDs)

// The system automatically manages execution order and timing
result := turboRun.WaitFor(finalNode)
fmt.Printf("Final aggregation: %s", result.Value.Choices[0].Message.Content)
```

## Benefits

**Efficiency**: By running tasks concurrently and respecting rate limits, TurboRun can dramatically reduce total execution time for complex workflows.

**Reliability**: Built-in rate limit compliance prevents API rejections and automatic retries.

**Simplicity**: Complex dependency management is handled automatically - you just specify what depends on what.

**Observability**: Full tracking of token consumption, timing, and execution patterns.

**Scalability**: The worker pool and priority queue system can handle hundreds of concurrent tasks without degradation.

## Thread Safety

All components of TurboRun are designed to be thread-safe. The consumption tracker, priority queue, and worker pool all use appropriate locking mechanisms to ensure safe concurrent access. This allows the system to run efficiently across multiple CPU cores while maintaining data consistency.

## Use Cases

TurboRun is particularly well-suited for:

- **Evaluation Pipelines**: Running large batches of test cases against LLMs
- **Multi-Step Analysis**: Workflows that require sequential processing with LLM calls at each step  
- **Comparison Studies**: Testing the same inputs across multiple providers
- **Data Processing**: Batch processing of documents or datasets through LLM pipelines
- **A/B Testing**: Running parallel experiments with different prompts or models

The system's ability to maximize throughput while respecting rate limits makes it ideal for any scenario where you need to process large volumes of LLM requests efficiently and reliably.

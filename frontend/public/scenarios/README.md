# TurboRun Scenario Examples

This directory contains example scenario JSON files that demonstrate different TurboRun behaviors.

## Available Scenarios

### 1. `simple-chain.json` - Linear Dependency Chain
**Purpose**: Introduction to basic dependency flow
**Nodes**: 5 nodes in a linear chain (node-1 → node-2 → node-3 → node-4 → node-5)
**Providers**: Mix of Groq and OpenAI
**Duration**: ~10 seconds total
**What it demonstrates**:
- Sequential execution based on dependencies
- Priority queue ordering by token count
- Basic worker assignment

### 2. `fan-out-fan-in.json` - Parallel Execution (Diamond Pattern)
**Purpose**: Show concurrent execution and synchronization
**Nodes**: 6 nodes (1 root → 4 parallel → 1 aggregation)
**Providers**: Mix of Groq and OpenAI
**Duration**: ~8 seconds total
**What it demonstrates**:
- Fan-out: Multiple nodes starting in parallel after root completes
- Priority queue with multiple ready nodes
- Fan-in: Aggregation node waiting for all dependencies
- Worker pool utilization with concurrent tasks

### 3. `rate-limit-demo.json` - Budget Exhaustion & Blocking
**Purpose**: Demonstrate rate limiting behavior
**Nodes**: 8 independent nodes (all ready immediately)
**Providers**: All Groq
**Rate Limits**: **Intentionally low** (2000 tokens/8 seconds)
**Duration**: ~25 seconds (with multiple budget cycles)
**What it demonstrates**:
- Nodes getting blocked when budget insufficient
- Budget reset every 8 seconds
- Priority queue with blocked nodes
- Staggered execution due to rate limits
- Budget consumption tracking

**Expected behavior**:
- First 2-3 nodes launch immediately
- Remaining nodes get blocked (status: "blocked")
- After 8 seconds, budget resets and next batch launches
- Cycle repeats until all nodes complete

### 4. `retry-showcase.json` - Retry Logic & Failures
**Purpose**: Show retry behavior and failure propagation
**Nodes**: 6 nodes with varying retry counts
**Providers**: Mix of Groq and OpenAI
**Duration**: ~15 seconds
**What it demonstrates**:
- Nodes succeeding on first attempt
- Nodes retrying 1-2 times before success
- Nodes failing after all retries exhausted
- Failed nodes staying visible in graph
- Blocked nodes (dependencies never satisfied due to failure)

**Node behaviors**:
- `success-first-try`: 1 attempt, succeeds
- `success-after-retry`: 2 attempts, succeeds on second
- `success-after-two-retries`: 3 attempts, succeeds on third
- `fails-after-retries`: 3 attempts, **fails** after all retries
- `depends-on-success`: Launches after first two succeed
- `blocked-by-failure`: **Never launches** (depends on failed node)

## Creating Your Own Scenarios

### JSON Schema

```json
{
  "nodes": [
    {
      "id": "unique-node-id",
      "provider": "groq" | "openai",
      "estimated_tokens": 500,
      "duration_ms": [1500, 2000, 2500],  // First attempt, retry 1, retry 2
      "final_status": "success" | "failed",
      "dependencies": ["other-node-id"]
    }
  ],
  "rate_limits": {
    "groq": {
      "tokens_per_window": 10000,
      "requests_per_window": 50,
      "window_ms": 15000
    },
    "openai": {
      "tokens_per_window": 10000,
      "requests_per_window": 50,
      "window_ms": 15000
    }
  }
}
```

### Field Descriptions

#### Node Fields
- **`id`**: Unique string identifier
- **`provider`**: Either `"groq"` or `"openai"`
- **`estimated_tokens`**: Used for priority queue ordering (higher = higher priority) and budget tracking
- **`duration_ms`**: Array of durations in milliseconds
  - Length determines number of attempts
  - `[1500]` = 1 attempt (1.5 seconds)
  - `[1000, 1500, 2000]` = 3 attempts (1s, then 1.5s, then 2s)
- **`final_status`**:
  - `"success"` = Last attempt succeeds
  - `"failed"` = All attempts fail
- **`dependencies`**: Array of node IDs that must complete before this node can start

#### Rate Limit Fields
- **`tokens_per_window`**: Maximum tokens allowed in the window
- **`requests_per_window`**: Maximum requests allowed in the window
- **`window_ms`**: Window duration in milliseconds
  - Use shorter windows (8000-15000ms) for faster demos
  - Backend uses 60000ms (1 minute)

### Tips for Creating Scenarios

1. **Start simple**: Begin with a few nodes, test, then expand
2. **Test dependencies**: Ensure no circular dependencies (validator will catch this)
3. **Rate limits**: Set intentionally low to demonstrate blocking, or high to show unthrottled execution
4. **Token distribution**: Vary estimated_tokens to see priority queue reordering
5. **Retry patterns**: Mix nodes with different retry counts for visual variety
6. **Failures**: Use sparingly - they block dependent nodes permanently

### Validation

The scenario loader automatically validates:
- ✅ All node IDs are unique
- ✅ All dependencies reference existing nodes
- ✅ No circular dependencies
- ✅ All required fields present
- ✅ All numeric values are valid
- ✅ All enum values are valid

Errors will be displayed in an alert when loading.

## Playback Tips

- **Speed**: Use 2x or 4x speed for long scenarios
- **Pause**: Pause to inspect current state in detail
- **Reset**: Reset to reload the same scenario and try again
- **Console**: Check browser console for detailed event logs (if enabled in dev mode)

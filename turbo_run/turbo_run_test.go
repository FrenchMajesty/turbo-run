package turbo_run

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FrenchMajesty/turbo-run/clients/groq"
	"github.com/FrenchMajesty/turbo-run/rate_limit"
	openai "github.com/openai/openai-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestableGroqClient wraps the mock for testing
type TestableGroqClient struct {
	*groq.MockGroqClient
	callCount int
	mu        sync.Mutex
}

func (m *TestableGroqClient) ChatCompletion(ctx context.Context, req groq.ChatCompletionRequest) (*groq.ChatCompletionResponse, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	return m.MockGroqClient.ChatCompletion(ctx, req)
}

func (m *TestableGroqClient) ChatCompletionStream(ctx context.Context, req groq.ChatCompletionRequest, callback func(token string)) (*groq.StreamingResult, error) {
	return m.MockGroqClient.ChatCompletionStream(ctx, req, callback)
}

func (m *TestableGroqClient) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func NewTestableGroqClient() *TestableGroqClient {
	return &TestableGroqClient{
		MockGroqClient: groq.NewMockGroqClient(),
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

// TestTurboRun_Singleton verifies singleton pattern works correctly
func TestTurboRun_Singleton(t *testing.T) {
	// Reset singleton for test
	instance = nil
	once = sync.Once{}

	mockGroq := NewTestableGroqClient()
	mockOpenAI := &openai.Client{}

	// Create multiple instances - should all be the same
	tr1 := NewTurboRun(mockGroq, mockOpenAI)
	tr2 := NewTurboRun(mockGroq, mockOpenAI)
	tr3 := NewTurboRun(mockGroq, mockOpenAI)

	assert.Equal(t, tr1, tr2, "Should return same instance")
	assert.Equal(t, tr2, tr3, "Should return same instance")
	assert.Equal(t, tr1, instance, "Should set global instance")

	tr1.Stop()
}

// TestTurboRun_BufferStateProgression tests buffer states change correctly as nodes are added
func TestTurboRun_BufferStateProgression(t *testing.T) {
	// Reset singleton
	instance = nil
	once = sync.Once{}

	mockGroq := NewTestableGroqClient()
	mockOpenAI := &openai.Client{}

	// Mock with slow responses to prevent processing during buffer testing
	mockGroq.On("ChatCompletion", mock.Anything, mock.Anything).Return(
		&groq.ChatCompletionResponse{}, nil,
	).Run(func(args mock.Arguments) {
		time.Sleep(200 * time.Millisecond) // Slow to keep nodes in buffers
	})

	tr := NewTurboRun(mockGroq, mockOpenAI)
	defer tr.Stop()

	// Verify initial state
	initialStats := tr.GetStats()
	assert.Equal(t, 0, initialStats.GraphSize, "Graph should start empty")
	assert.Equal(t, 0, initialStats.LaunchedCount, "No nodes should be launched initially")

	// Push nodes and verify buffer progression
	nodeCount := 50
	nodes := make([]*WorkNode, nodeCount)

	for i := 0; i < nodeCount; i++ {
		node := NewWorkNodeForGroq(groq.ChatCompletionRequest{
			Model: "llama3-8b-8192",
			Messages: []groq.ChatMessage{
				{Role: groq.MessageRoleUser, Content: stringPtr(fmt.Sprintf("buffer test %d", i))},
			},
		})

		// Verify node starts in pending state
		assert.Equal(t, WorkNodeStatusPending, node.GetStatus(), "New node should be pending")

		nodes[i] = node
		tr.Push(node)

		// Check buffer states after each push
		stats := tr.GetStats()

		// Graph should contain all pushed nodes
		assert.Equal(t, i+1, stats.GraphSize, "Graph should contain %d nodes after push %d", i+1, i+1)

		// Log progression every 10 nodes
		if (i+1)%10 == 0 {
			t.Logf("After %d nodes - Graph: %d, PQ: %d, Launched: %d",
				i+1, stats.GraphSize, stats.PriorityQueueSize, stats.LaunchedCount)
		}
	}

	finalStats := tr.GetStats()
	t.Logf("Final state - Graph: %d, PQ: %d, Launched: %d",
		finalStats.GraphSize, finalStats.PriorityQueueSize, finalStats.LaunchedCount)

	// Basic verification: system should accept nodes
	assert.Greater(t, finalStats.GraphSize, 0, "Graph should contain pushed nodes")
}

// TestTurboRun_BudgetEnforcementDirectly tests budget enforcement by checking actual budget state
func TestTurboRun_BudgetEnforcementDirectly(t *testing.T) {
	// Reset singleton
	instance = nil
	once = sync.Once{}

	mockGroq := NewTestableGroqClient()
	mockOpenAI := &openai.Client{}

	processingCount := int64(0)

	// Mock responses that consume budget
	mockGroq.On("ChatCompletion", mock.Anything, mock.Anything).Return(
		&groq.ChatCompletionResponse{}, nil,
	).Run(func(args mock.Arguments) {
		atomic.AddInt64(&processingCount, 1)
		time.Sleep(50 * time.Millisecond)
	})

	tr := NewTurboRun(mockGroq, mockOpenAI)
	defer tr.Stop()

	// Check initial budget state
	initialStats := tr.GetStats()
	assert.Equal(t, 0, initialStats.TrackerStats.GroqTotalTokens, "Should start with 0 total consumed")

	// Create nodes with substantial token estimates
	nodeCount := 10
	nodes := make([]*WorkNode, nodeCount)
	totalEstimatedTokens := 0

	for i := 0; i < nodeCount; i++ {
		// Create longer message to increase token estimate
		longMessage := fmt.Sprintf("Budget test %d. This is a longer message designed to consume more tokens for accurate budget testing. We want to verify the budget tracking system works correctly with realistic token consumption patterns.", i)

		node := NewWorkNodeForGroq(groq.ChatCompletionRequest{
			Model: "llama3-8b-8192",
			Messages: []groq.ChatMessage{
				{Role: groq.MessageRoleUser, Content: stringPtr(longMessage)},
			},
		})

		estimatedTokens := node.GetEstimatedTokens()
		totalEstimatedTokens += estimatedTokens
		nodes[i] = node

		t.Logf("Node %d: %d estimated tokens", i+1, estimatedTokens)
		tr.Push(node)
	}

	t.Logf("Total estimated tokens for %d nodes: %d", nodeCount, totalEstimatedTokens)

	// Get initial available budget (N)
	availableBudget, _ := tr.tracker.BudgetAvailableForCycle(groq.ProviderGroq)
	t.Logf("Initial available budget: %d tokens", availableBudget)

	// Allow system time to process/launch all nodes and consume budget
	time.Sleep(100 * time.Millisecond)

	// Check budget after nodes are launched (budget consumed upfront)
	budgetAfterLaunch, _ := tr.tracker.BudgetAvailableForCycle(groq.ProviderGroq)
	t.Logf("Budget after launch: %d tokens", budgetAfterLaunch)

	// Validate: budget was consumed when nodes were launched
	assert.Less(t, budgetAfterLaunch, availableBudget,
		"Available budget should decrease after nodes are launched (was %d, now %d)",
		availableBudget, budgetAfterLaunch)

	// Budget decrease should match total estimated consumption
	actualDecrease := availableBudget - budgetAfterLaunch
	assert.Equal(t, totalEstimatedTokens, actualDecrease,
		"Budget decrease should match total estimated token consumption (%d tokens)", totalEstimatedTokens)

	t.Logf("Budget tracking: consumed %d tokens upfront when nodes launched, available: %d → %d",
		actualDecrease, availableBudget, budgetAfterLaunch)

	// Process nodes and verify they complete (budget already consumed)
	for i, node := range nodes {
		result := tr.WaitFor(node)
		assert.NoError(t, result.Error, "Node %d should complete successfully", i+1)
		assert.Equal(t, WorkNodeStatusCompleted, node.GetStatus(), "Node should be completed")
	}

	// Verify processing occurred
	finalProcessingCount := atomic.LoadInt64(&processingCount)
	assert.Equal(t, int64(nodeCount), finalProcessingCount, "Should have processed exact number of nodes")

	// Final budget should be initial - total consumed
	finalStats := tr.GetStats()
	assert.Equal(t, totalEstimatedTokens, finalStats.TrackerStats.GroqTotalTokens,
		"Total consumed should match sum of estimated tokens")
}

// TestTurboRun_NodeLifecycleStates tests node status transitions through the lifecycle
func TestTurboRun_NodeLifecycleStates(t *testing.T) {
	// Reset singleton
	instance = nil
	once = sync.Once{}

	mockGroq := NewTestableGroqClient()
	mockOpenAI := &openai.Client{}

	// Mock for processing with a small delay to ensure state transitions are observable
	mockGroq.On("ChatCompletion", mock.Anything, mock.Anything).Return(
		&groq.ChatCompletionResponse{}, nil,
	).Run(func(args mock.Arguments) {
		time.Sleep(50 * time.Millisecond) // Small delay to ensure state transitions
	})

	tr := NewTurboRun(mockGroq, mockOpenAI)
	defer tr.Stop()

	// Create and process nodes
	nodeCount := 3
	nodes := make([]*WorkNode, nodeCount)

	for i := 0; i < nodeCount; i++ {
		node := NewWorkNodeForGroq(groq.ChatCompletionRequest{
			Model: "llama3-8b-8192",
			Messages: []groq.ChatMessage{
				{Role: groq.MessageRoleUser, Content: stringPtr(fmt.Sprintf("lifecycle test %d", i))},
			},
		})

		assert.Equal(t, WorkNodeStatusPending, node.GetStatus(), "New node should start pending")
		nodes[i] = node
		tr.Push(node)
	}

	// Process each node and monitor state transitions using the new status channel
	var wg sync.WaitGroup
	for i, node := range nodes {
		wg.Add(1)
		go func(index int, n *WorkNode) {
			defer wg.Done()

			// Track status transitions using a timeout-based approach
			var statusHistory []WorkNodeStatus
			done := make(chan bool)

			// Start monitoring status changes
			go func() {
				defer close(done)

				// Listen for all status changes until we see completed
				for {
					status := n.ListenForStatus()
					statusHistory = append(statusHistory, status)
					t.Logf("Node %d status change: %d", index+1, int(status))

					// Stop when we reach a final state
					if status == WorkNodeStatusCompleted || status == WorkNodeStatusFailed {
						break
					}
				}
			}()

			// Wait for the node to complete with timeout
			result := tr.WaitFor(n)
			assert.NoError(t, result.Error, "Node %d should complete successfully", index+1)

			// Wait for status monitoring to complete with timeout
			select {
			case <-done:
				// Status monitoring completed
			case <-time.After(1 * time.Second):
				t.Logf("Node %d status monitoring timed out", index+1)
			}

			// Verify we captured the expected state transitions
			if len(statusHistory) >= 1 {
				// Should have at least seen Running status
				foundRunning := false
				foundCompleted := false

				for _, status := range statusHistory {
					if status == WorkNodeStatusRunning {
						foundRunning = true
					}
					if status == WorkNodeStatusCompleted {
						foundCompleted = true
					}
				}

				assert.True(t, foundRunning, "Node %d should have transitioned to Running", index+1)
				assert.True(t, foundCompleted, "Node %d should have transitioned to Completed", index+1)

				// Final status should be Completed
				finalStatus := statusHistory[len(statusHistory)-1]
				assert.Equal(t, WorkNodeStatusCompleted, finalStatus, "Node %d final status should be Completed", index+1)
			} else {
				t.Errorf("Node %d captured no status changes", index+1)
			}

			// Final verification: node should be completed
			assert.Equal(t, WorkNodeStatusCompleted, n.GetStatus(), "Node %d should be completed after WaitFor", index+1)

		}(i, node)
	}

	// Wait for all nodes to complete
	wg.Wait()
}

// TestTurboRun_WorkerPoolUtilization tests worker pool metrics under load
func TestTurboRun_WorkerPoolUtilization(t *testing.T) {
	// Reset singleton
	instance = nil
	once = sync.Once{}

	mockGroq := NewTestableGroqClient()
	mockOpenAI := &openai.Client{}

	maxObservedBusy := 0
	var utilizationMutex sync.Mutex

	// Mock with variable processing time to observe utilization
	mockGroq.On("ChatCompletion", mock.Anything, mock.Anything).Return(
		&groq.ChatCompletionResponse{}, nil,
	).Run(func(args mock.Arguments) {
		time.Sleep(80 * time.Millisecond) // Moderate processing time
	})

	tr := NewTurboRun(mockGroq, mockOpenAI)
	defer tr.Stop()

	// Monitor worker utilization in background
	stopMonitoring := make(chan bool)
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-stopMonitoring:
				return
			case <-ticker.C:
				stats := tr.GetStats()

				utilizationMutex.Lock()
				if stats.WorkersPoolBusy > maxObservedBusy {
					maxObservedBusy = stats.WorkersPoolBusy
				}
				utilizationMutex.Unlock()
			}
		}
	}()

	// Push many nodes quickly to create utilization
	nodeCount := 30
	nodes := make([]*WorkNode, nodeCount)

	for i := 0; i < nodeCount; i++ {
		node := NewWorkNodeForGroq(groq.ChatCompletionRequest{
			Model: "llama3-8b-8192",
			Messages: []groq.ChatMessage{
				{Role: groq.MessageRoleUser, Content: stringPtr(fmt.Sprintf("utilization test %d", i))},
			},
		})

		nodes[i] = node
		tr.Push(node)
	}

	// Wait for some processing to occur
	time.Sleep(200 * time.Millisecond)

	currentStats := tr.GetStats()
	t.Logf("Current worker utilization: %d/%d busy", currentStats.WorkersPoolBusy, 120) // 120 is worker pool size

	// Process a few nodes to completion
	for i := 0; i < 10 && i < nodeCount; i++ {
		result := tr.WaitFor(nodes[i])
		assert.NoError(t, result.Error, "Node should complete successfully")
	}

	// Stop monitoring
	close(stopMonitoring)

	// Verify worker utilization occurred
	utilizationMutex.Lock()
	finalMaxBusy := maxObservedBusy
	utilizationMutex.Unlock()

	assert.Greater(t, finalMaxBusy, 0, "Should have observed busy workers")

	finalStats := tr.GetStats()
	t.Logf("Max observed busy workers: %d, Final busy: %d, Worker pool size: 120",
		finalMaxBusy, finalStats.WorkersPoolBusy)
}

// TestTurboRun_BufferOverflowHandling tests behavior when buffers reach capacity limits
func TestTurboRun_BufferOverflowHandling(t *testing.T) {
	// Reset singleton
	instance = nil
	once = sync.Once{}

	mockGroq := NewTestableGroqClient()
	mockOpenAI := &openai.Client{}

	// Make processing very slow to fill buffers
	mockGroq.On("ChatCompletion", mock.Anything, mock.Anything).Return(
		&groq.ChatCompletionResponse{}, nil,
	).Run(func(args mock.Arguments) {
		time.Sleep(500 * time.Millisecond) // Very slow processing
	})

	tr := NewTurboRun(mockGroq, mockOpenAI)
	defer tr.Stop()

	// Push more nodes than launchpad buffer capacity (100)
	nodeCount := 150
	bufferSnapshots := make([]*TurboRunStats, 0)

	for i := 0; i < nodeCount; i++ {
		node := NewWorkNodeForGroq(groq.ChatCompletionRequest{
			Model: "llama3-8b-8192",
			Messages: []groq.ChatMessage{
				{Role: groq.MessageRoleUser, Content: stringPtr(fmt.Sprintf("overflow test %d", i))},
			},
		})

		tr.Push(node)

		// Take snapshots at key points
		if (i+1)%25 == 0 || i+1 == nodeCount {
			stats := tr.GetStats()
			bufferSnapshots = append(bufferSnapshots, stats)

			t.Logf("After %d nodes - Graph: %d, PQ: %d, Launchpad: %d, WorkerPool: %d",
				i+1, stats.GraphSize, stats.PriorityQueueSize, stats.LaunchpadSize, stats.WorkersPoolSize)
		}
	}

	// Analyze buffer behavior
	maxGraphSize := 0
	maxLaunchpadSize := 0
	maxPriorityQueueSize := 0

	for _, snapshot := range bufferSnapshots {
		if snapshot.GraphSize > maxGraphSize {
			maxGraphSize = snapshot.GraphSize
		}
		if snapshot.LaunchpadSize > maxLaunchpadSize {
			maxLaunchpadSize = snapshot.LaunchpadSize
		}
		if snapshot.PriorityQueueSize > maxPriorityQueueSize {
			maxPriorityQueueSize = snapshot.PriorityQueueSize
		}
	}

	// Just verify the system processed nodes despite buffer pressure

	// Verify system continues working despite buffer pressure
	testNode := NewWorkNodeForGroq(groq.ChatCompletionRequest{
		Model: "llama3-8b-8192",
		Messages: []groq.ChatMessage{
			{Role: groq.MessageRoleUser, Content: stringPtr("test processing under pressure")},
		},
	})

	tr.Push(testNode)
	result := tr.WaitFor(testNode)
	assert.NoError(t, result.Error, "Should still process nodes under buffer pressure")

	finalStats := tr.GetStats()
	t.Logf("Buffer analysis - Max Graph: %d, Max PQ: %d, Max Launchpad: %d, Final busy workers: %d",
		maxGraphSize, maxPriorityQueueSize, maxLaunchpadSize, finalStats.WorkersPoolBusy)
}

// TestTurboRun_NoNodeDropping verifies no nodes are lost using direct state tracking
func TestTurboRun_NoNodeDropping(t *testing.T) {
	// Reset singleton
	instance = nil
	once = sync.Once{}

	mockGroq := NewTestableGroqClient()
	mockOpenAI := &openai.Client{}

	processedNodeIDs := make(map[string]bool)
	var trackingMutex sync.Mutex

	mockGroq.On("ChatCompletion", mock.Anything, mock.Anything).Return(
		&groq.ChatCompletionResponse{}, nil,
	).Run(func(args mock.Arguments) {
		req := args.Get(1).(groq.ChatCompletionRequest)
		if len(req.Messages) > 0 && req.Messages[0].Content != nil {
			trackingMutex.Lock()
			processedNodeIDs[*req.Messages[0].Content] = true
			trackingMutex.Unlock()
		}
		time.Sleep(30 * time.Millisecond)
	})

	tr := NewTurboRun(mockGroq, mockOpenAI)
	defer tr.Stop()

	// Create unique identifiable nodes
	nodeCount := 60
	expectedNodeIDs := make(map[string]bool)
	nodes := make([]*WorkNode, nodeCount)

	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("unique-node-%d-%d", i, time.Now().UnixNano()%10000)
		expectedNodeIDs[nodeID] = true

		node := NewWorkNodeForGroq(groq.ChatCompletionRequest{
			Model: "llama3-8b-8192",
			Messages: []groq.ChatMessage{
				{Role: groq.MessageRoleUser, Content: stringPtr(nodeID)},
			},
		})

		nodes[i] = node
		tr.Push(node)

		// Verify node is tracked in system
		if (i+1)%10 == 0 {
			stats := tr.GetStats()
			totalInSystem := stats.GraphSize + stats.PriorityQueueSize + stats.LaunchpadSize + stats.WorkersPoolSize
			t.Logf("After pushing %d nodes: %d total in system buffers", i+1, totalInSystem)
		}
	}

	// Process all nodes
	var wg sync.WaitGroup
	completedCount := int64(0)
	errorCount := int64(0)

	for _, node := range nodes {
		wg.Add(1)
		go func(n *WorkNode) {
			defer wg.Done()

			result := tr.WaitFor(n)
			if result.Error == nil {
				atomic.AddInt64(&completedCount, 1)
				assert.Equal(t, WorkNodeStatusCompleted, n.GetStatus(), "Completed node should have completed status")
			} else {
				atomic.AddInt64(&errorCount, 1)
				assert.Equal(t, WorkNodeStatusFailed, n.GetStatus(), "Failed node should have failed status")
			}
		}(node)
	}

	wg.Wait()

	// Verify no nodes were dropped
	trackingMutex.Lock()
	actualProcessedCount := len(processedNodeIDs)
	trackingMutex.Unlock()

	finalCompletedCount := atomic.LoadInt64(&completedCount)
	finalErrorCount := atomic.LoadInt64(&errorCount)

	assert.Equal(t, int64(0), finalErrorCount, "No nodes should fail")
	assert.Equal(t, int64(nodeCount), finalCompletedCount, "All nodes should complete")
	assert.Equal(t, nodeCount, actualProcessedCount, "All unique nodes should be processed")

	// Verify every expected node was actually processed
	for expectedID := range expectedNodeIDs {
		assert.True(t, processedNodeIDs[expectedID], "Node %s should have been processed", expectedID)
	}

	// Verify system state is clean after processing
	finalStats := tr.GetStats()
	t.Logf("Final system state - Graph: %d, PQ: %d, Launchpad: %d, WorkerPool: %d, Busy: %d",
		finalStats.GraphSize, finalStats.PriorityQueueSize, finalStats.LaunchpadSize,
		finalStats.WorkersPoolSize, finalStats.WorkersPoolBusy)

	assert.Equal(t, nodeCount, mockGroq.GetCallCount(), "Should make exact number of API calls")
}

// TestTurboRun_SlowProcessingShowsWorkerUtilization - Overwhelm workers with slow tasks
func TestTurboRun_SlowProcessingShowsWorkerUtilization(t *testing.T) {
	// Reset singleton
	instance = nil
	once = sync.Once{}

	mockGroq := NewTestableGroqClient()
	mockOpenAI := &openai.Client{}

	processingCount := int64(0)
	maxObservedBusy := 0
	maxObservedWorkerPool := 0
	var utilizationMutex sync.Mutex

	// Mock with VERY slow processing - 3 seconds per node
	mockGroq.On("ChatCompletion", mock.Anything, mock.Anything).Return(
		&groq.ChatCompletionResponse{}, nil,
	).Run(func(args mock.Arguments) {
		count := atomic.AddInt64(&processingCount, 1)
		t.Logf("🔧 Mock called for node %d - starting 3sec processing", count)
		time.Sleep(3 * time.Second) // Very slow processing
		t.Logf("✅ Mock completed for node %d", count)
	})

	tr := NewTurboRun(mockGroq, mockOpenAI)
	defer tr.Stop()

	// Monitor stats continuously in background
	stopMonitoring := make(chan bool)
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond) // Check every 200ms
		defer ticker.Stop()

		for {
			select {
			case <-stopMonitoring:
				return
			case <-ticker.C:
				stats := tr.GetStats()

				utilizationMutex.Lock()
				if stats.WorkersPoolBusy > maxObservedBusy {
					maxObservedBusy = stats.WorkersPoolBusy
					t.Logf("📊 New max busy workers: %d (WorkerPool: %d, Launched: %d)",
						maxObservedBusy, stats.WorkersPoolSize, stats.LaunchedCount)
				}
				if stats.WorkersPoolSize > maxObservedWorkerPool {
					maxObservedWorkerPool = stats.WorkersPoolSize
					t.Logf("📊 New max worker pool size: %d", maxObservedWorkerPool)
				}
				utilizationMutex.Unlock()

				// Log interesting states
				if stats.WorkersPoolBusy > 0 || stats.WorkersPoolSize > 0 || stats.LaunchedCount > 0 {
					t.Logf("📊 Stats: Graph: %d, PQ: %d, Launchpad: %d, Launched: %d, WorkerPool: %d, Busy: %d",
						stats.GraphSize, stats.PriorityQueueSize, stats.LaunchpadSize,
						stats.LaunchedCount, stats.WorkersPoolSize, stats.WorkersPoolBusy)
				}
			}
		}
	}()

	// Push 10 slow nodes rapidly to overwhelm the worker pool (120 workers)
	nodeCount := 10
	nodes := make([]*WorkNode, nodeCount)

	t.Logf("🚀 Pushing %d slow nodes (3sec each)...", nodeCount)

	for i := 0; i < nodeCount; i++ {
		node := NewWorkNodeForGroq(groq.ChatCompletionRequest{
			Model: "llama3-8b-8192",
			Messages: []groq.ChatMessage{
				{Role: groq.MessageRoleUser, Content: stringPtr("slow processing test")},
			},
		})

		nodes[i] = node
		tr.Push(node)
		t.Logf("📤 Pushed node %d", i+1)
	}

	t.Logf("⏱️ All nodes pushed, waiting 1 second for them to start processing...")
	time.Sleep(1 * time.Second)

	// Check stats after nodes have had time to start processing
	currentStats := tr.GetStats()
	t.Logf("📊 Stats after 1sec: Graph: %d, PQ: %d, Launchpad: %d, Launched: %d, WorkerPool: %d, Busy: %d",
		currentStats.GraphSize, currentStats.PriorityQueueSize, currentStats.LaunchpadSize,
		currentStats.LaunchedCount, currentStats.WorkersPoolSize, currentStats.WorkersPoolBusy)

	// We should definitely see busy workers by now
	assert.Greater(t, currentStats.WorkersPoolBusy, 0, "Should have busy workers processing slow nodes")
	assert.Greater(t, currentStats.LaunchedCount, 0, "Should have launched some nodes")

	// Wait a bit longer to see more utilization
	t.Logf("⏱️ Waiting 2 more seconds to observe peak utilization...")
	time.Sleep(2 * time.Second)

	midStats := tr.GetStats()
	t.Logf("📊 Stats after 3sec total: Graph: %d, PQ: %d, Launchpad: %d, Launched: %d, WorkerPool: %d, Busy: %d",
		midStats.GraphSize, midStats.PriorityQueueSize, midStats.LaunchpadSize,
		midStats.LaunchedCount, midStats.WorkersPoolSize, midStats.WorkersPoolBusy)

	// Now process first 3 nodes to completion and verify they work
	t.Logf("⏱️ Waiting for first 3 nodes to complete...")
	completedCount := 0
	for i := 0; i < 3; i++ {
		result := tr.WaitFor(nodes[i])
		if result.Error == nil {
			completedCount++
			t.Logf("✅ Node %d completed successfully", i+1)
		} else {
			t.Logf("❌ Node %d failed: %v", i+1, result.Error)
		}
	}

	// Stop monitoring
	close(stopMonitoring)

	// Final verification
	utilizationMutex.Lock()
	finalMaxBusy := maxObservedBusy
	finalMaxWorkerPool := maxObservedWorkerPool
	utilizationMutex.Unlock()

	finalStats := tr.GetStats()
	currentProcessingCount := atomic.LoadInt64(&processingCount)

	t.Logf("🔍 Final Results:")
	t.Logf("  Max observed busy workers: %d", finalMaxBusy)
	t.Logf("  Max observed worker pool size: %d", finalMaxWorkerPool)
	t.Logf("  Total nodes launched: %d", finalStats.LaunchedCount)
	t.Logf("  Nodes that started processing: %d", currentProcessingCount)
	t.Logf("  Nodes completed in test: %d", completedCount)

	// Assertions
	assert.Greater(t, finalMaxBusy, 0, "Should have observed busy workers during slow processing")
	assert.Equal(t, nodeCount, int(finalStats.LaunchedCount), "All nodes should have been launched")
	assert.GreaterOrEqual(t, int(currentProcessingCount), completedCount, "Processing count should match or exceed completed")
	assert.Equal(t, completedCount, 3, "Should have completed 3 nodes")

	// The key test: we should see multiple workers busy when processing is slow
	if finalMaxBusy == 0 {
		t.Errorf("CRITICAL: Never observed any busy workers despite 3-second processing time")
		t.Errorf("This suggests GetBusyWorkers() or worker tracking is broken")
	} else {
		t.Logf("✅ SUCCESS: Observed up to %d busy workers processing slow tasks", finalMaxBusy)
	}
}

func TestTurboRun_BudgetExhaustionBlocking(t *testing.T) {
	// Reset singleton
	instance = nil
	once = sync.Once{}

	mockGroq := NewTestableGroqClient()
	mockOpenAI := &openai.Client{}

	// Store original limits to restore after test
	originalGroqTokens := int(rate_limit.GroqRateLimit.TPM)
	originalGroqRequests := rate_limit.GroqRateLimit.RPM
	originalOpenAITokens := int(rate_limit.OpenAIRateLimit.TPM)
	originalOpenAIRequests := rate_limit.OpenAIRateLimit.RPM

	// Mock responses
	mockGroq.On("ChatCompletion", mock.Anything, mock.Anything).Return(
		&groq.ChatCompletionResponse{}, nil,
	).Run(func(args mock.Arguments) {
		time.Sleep(50 * time.Millisecond)
	})

	tr := NewTurboRun(mockGroq, mockOpenAI)
	defer func() {
		// Restore original limits after test (critical for singleton)
		tr.OverrideBudgetsForTests(originalGroqTokens, originalOpenAITokens, originalGroqRequests, originalOpenAIRequests)
		tr.Stop()
	}()

	// Set intentionally low budget limits (10 tokens max, 5 requests max)
	tr.OverrideBudgetsForTests(10, 10, 5, 5)

	// Create 20 nodes with moderate content to exceed the low budget
	nodeCount := 20
	nodes := make([]*WorkNode, nodeCount)
	totalEstimatedTokens := 0

	for i := 0; i < nodeCount; i++ {
		// Create very short content to stay within our tiny budget initially
		content := fmt.Sprintf("test %d", i)

		node := NewWorkNodeForGroq(groq.ChatCompletionRequest{
			Model: "llama3-8b-8192",
			Messages: []groq.ChatMessage{
				{Role: groq.MessageRoleUser, Content: stringPtr(content)},
			},
		})

		estimatedTokens := node.GetEstimatedTokens()
		totalEstimatedTokens += estimatedTokens
		nodes[i] = node
		tr.Push(node)

		t.Logf("Node %d: %d estimated tokens", i+1, estimatedTokens)
	}

	t.Logf("Total estimated tokens for %d nodes: %d (budget limit: 10)", nodeCount, totalEstimatedTokens)

	// Allow initial processing time
	time.Sleep(100 * time.Millisecond)

	// Get stats after initial processing attempt
	stats := tr.GetStats()
	t.Logf("After processing attempt - Launched: %d, Graph: %d, PQ: %d",
		stats.LaunchedCount, stats.GraphSize, stats.PriorityQueueSize)

	// Validate that we hit budget limits quickly - should have launched very few nodes
	assert.True(t, stats.LaunchedCount <= 1, "Should have launched very few nodes due to budget constraints, got %d", stats.LaunchedCount)

	// Check budget availability - should be at or near the limit
	availableTokens, availableRequests := tr.tracker.BudgetAvailableForCycle(groq.ProviderGroq)
	t.Logf("Budget after initial processing - Available tokens: %d, requests: %d", availableTokens, availableRequests)

	// Budget should be insufficient for remaining nodes (each node needs 33 tokens, we have 10)
	budgetInsufficient := availableTokens < 33 // Not enough for a single node
	assert.True(t, budgetInsufficient, "Budget should be insufficient for remaining nodes - tokens: %d (need 33 per node)", availableTokens)

	// Check that there are still nodes waiting to be processed
	nodesWaiting := stats.GraphSize > 0 || stats.PriorityQueueSize > 0
	assert.True(t, nodesWaiting, "Should have nodes waiting due to budget exhaustion - graph: %d, pq: %d", stats.GraphSize, stats.PriorityQueueSize)

	// Record current state before waiting
	initialLaunchedCount := stats.LaunchedCount
	initialAvailableTokens := availableTokens
	initialAvailableRequests := availableRequests

	// Wait longer - system should be blocked waiting for budget reset
	t.Logf("Waiting for budget blocking behavior...")
	time.Sleep(500 * time.Millisecond)

	// Get updated stats after waiting
	statsAfterWait := tr.GetStats()
	finalAvailableTokens, finalAvailableRequests := tr.tracker.BudgetAvailableForCycle(groq.ProviderGroq)

	t.Logf("After waiting - Launched: %d (was %d), Available tokens: %d (was %d), requests: %d (was %d)",
		statsAfterWait.LaunchedCount, initialLaunchedCount,
		finalAvailableTokens, initialAvailableTokens,
		finalAvailableRequests, initialAvailableRequests)

	// If budget is still insufficient, no additional nodes should launch
	if finalAvailableTokens < 33 { // Still insufficient for a single node
		assert.Equal(t, initialLaunchedCount, statsAfterWait.LaunchedCount,
			"No additional nodes should launch while budget is insufficient")
		t.Logf("✅ SUCCESS: System correctly blocked processing when budget insufficient")
	} else {
		// If budget somehow became sufficient, some nodes might have launched
		t.Logf("Budget became sufficient during wait, allowing some additional processing")
	}

	// Validate the blocking mechanism works - total nodes launched should be much less than pushed
	launchRatio := float64(statsAfterWait.LaunchedCount) / float64(nodeCount)
	assert.Less(t, launchRatio, 0.5, "Should have launched less than half the nodes due to budget limits (launched %d/%d = %.2f)",
		statsAfterWait.LaunchedCount, nodeCount, launchRatio*100)

	t.Logf("Final validation - Launched %d/%d nodes (%.1f%%) due to budget exhaustion",
		statsAfterWait.LaunchedCount, nodeCount, launchRatio*100)
}

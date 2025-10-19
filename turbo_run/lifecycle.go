package turbo_run

import (
	"math/rand"
	"time"

	"github.com/FrenchMajesty/turbo-run/utils/priority_queue"
	"github.com/google/uuid"
)

// FailureHandlingStrategy determines how to handle failures that occur after retries are exhausted
type FailureHandlingStrategy string

const (
	// FailureStrategyPropagate causes failures to cascade down the dependency tree,
	// recursively cancelling all dependent children
	FailureStrategyPropagate FailureHandlingStrategy = "propagate"

	// FailureStrategyIsolate isolates failures to just the failed node,
	// allowing dependent children to proceed with execution
	FailureStrategyIsolate FailureHandlingStrategy = "isolate"
)

// Start starts the turbo runner
func (tr *TurboRun) Start() {
	// Start core goroutines with WaitGroup tracking
	tr.wg.Add(6)
	go func() {
		defer tr.wg.Done()
		instance.listenForGraphReadyNodes()
	}()
	go func() {
		defer tr.wg.Done()
		instance.listenForLaunchPad()
	}()
	go func() {
		defer tr.wg.Done()
		instance.startMinuteTimer()
	}()
	go func() {
		defer tr.wg.Done()
		instance.listenForWorkerStateChanges()
	}()
	go func() {
		defer tr.wg.Done()
		instance.listenForWorkNodePushRequests()
	}()
	go func() {
		defer tr.wg.Done()
		instance.startAnalyticsLogger()
	}()

	time.Sleep(5 * time.Millisecond) // give time for the goroutines to start
	tr.logger.Printf("TurboRun %s: Started with %d workers", tr.uniqueID, tr.workersPool.GetWorkerCount())
}

// Stop stops the turbo runner gracefully
func (tr *TurboRun) Stop() {
	// Stop worker pool first
	tr.workersPool.Stop()

	close(tr.quit)

	tr.wg.Wait()

	if tr.pushChan != nil {
		close(tr.pushChan)
	}

	if tr.eventChan != nil {
		close(tr.eventChan)
	}

	tr.logger.Printf("TurboRun %s: Shutting down", tr.uniqueID)
	tr.logger.Close()
}

// Reset cancels all in-flight work and clears the graph.
// This should be called when you want to cancel current processing and prepare a new graph.
func (tr *TurboRun) Reset() {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.logger.Printf("TurboRun %s: Resetting - cancelling all in-flight work", tr.uniqueID)

	// Clear the graph and get all removed node IDs
	removedIDs := tr.graph.Clear()

	// Drain the priority queue
	for tr.priorityQueue.Size() > 0 {
		tr.priorityQueue.Pop()
	}

	// Drain the launchpad channel (non-blocking)
	for {
		select {
		case <-tr.launchpad:
			// Drained one item
		default:
			// Channel is empty
			goto doneDrainingLaunchpad
		}
	}
doneDrainingLaunchpad:

	// Drain the pushChan (non-blocking)
	for {
		select {
		case <-tr.pushChan:
			// Drained one item
		default:
			// Channel is empty
			goto doneDrainingPushChan
		}
	}
doneDrainingPushChan:

	// Reset counters
	tr.launchedCount = 0
	tr.completedCount = 0
	tr.failedCount = 0

	// Emit cancellation events for all removed nodes
	for _, nodeID := range removedIDs {
		tr.emitEvent(EventNodeCancelled, nodeID, map[string]any{
			"reason": "reset",
		})
	}

	tr.logger.Printf("TurboRun %s: Reset complete - cancelled %d nodes", tr.uniqueID, len(removedIDs))
}

// listenForGraphReadyNodes listens for ready nodes published from the Graph and pushes them into the priority queue
func (tr *TurboRun) listenForGraphReadyNodes() {
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

			// Signal that a node is ready to launch
			tr.launchpad <- struct{}{}
		}
	}
}

// Pause pauses the processing of nodes (they will be queued but not dispatched to workers)
func (tr *TurboRun) Pause() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.paused = true
	tr.logger.Printf("TurboRun %s: Paused", tr.uniqueID)
}

// Resume resumes the processing of nodes
func (tr *TurboRun) Resume() {
	tr.mu.Lock()
	wasPaused := tr.paused
	tr.paused = false
	queueSize := tr.priorityQueue.Size()
	tr.mu.Unlock()

	if wasPaused {
		tr.logger.Printf("TurboRun %s: Resumed with %d nodes in queue", tr.uniqueID, queueSize)

		// Signal the launchpad for each queued node to start processing
		for i := 0; i < queueSize; i++ {
			select {
			case tr.launchpad <- struct{}{}:
			default:
				// Launchpad full, nodes will be processed eventually
			}
		}
	}
}

// IsPaused returns whether TurboRun is currently paused
func (tr *TurboRun) IsPaused() bool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	return tr.paused
}

// listenForLaunchPad reads the value being put on the launchpad and sends them off to the workers pool
func (tr *TurboRun) listenForLaunchPad() {
	for {
		select {
		case <-tr.quit:
			return // Shutdown signal
		case <-tr.launchpad:
			// Check if paused - if so, skip processing this signal
			tr.mu.RLock()
			isPaused := tr.paused
			tr.mu.RUnlock()

			if isPaused {
				// Paused - don't process, signal will be lost but Resume() will re-signal
				continue
			}

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

				eventMetadata := map[string]any{
					"provider":           providerName(node.GetProvider()),
					"needed_tokens":      node.GetEstimatedTokens(),
					"available_tokens":   tokensBudget,
					"available_requests": requestBudget,
					"time_until_reset":   tr.tracker.TimeUntilReset().String(),
				}

				// Emit blocking event only once per node
				if !blocked {
					blocked = true
					tr.emitEvent(EventBudgetBlocked, node.ID, eventMetadata)
				}

				// Not enough budget, wait until cycle reset (but check for quit signal)
				randomStagger := time.Duration(rand.Intn(1000)) * time.Microsecond
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

			metadata := map[string]any{
				"provider":           providerName(node.GetProvider()),
				"tokens_consumed":    node.GetEstimatedTokens(),
				"tokens_available":   tokensAvailable,
				"requests_available": requestsAvailable,
				"tokens_total":       totalTokens,
				"utilization_pct":    utilizationPct,
			}
			tr.emitEvent(EventBudgetConsumed, uuid.Nil, metadata)

			// Emit budget warning if utilization is high
			if utilizationPct >= 80 {
				tr.emitEvent(EventBudgetWarning, uuid.Nil, metadata)
			}

			// Emit node dispatched event
			tr.emitEvent(EventNodeDispatched, node.ID, map[string]any{
				"worker_pool_busy": tr.workersPool.GetBusyWorkers(),
				"worker_pool_size": tr.workersPool.GetWorkerCount(),
			})

			tr.workersPool.Dispatch(node)
			tr.removeNodeFromGraphOnCompletion(node)

			// Increment launched count
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

			// Increment failed count
			tr.mu.Lock()
			tr.failedCount++
			tr.mu.Unlock()

			// Handle failure based on strategy
			if tr.failureHandlingStrategy == FailureStrategyPropagate {
				// Propagate: recursively remove all descendants
				removedIDs := tr.graph.RemoveSubtree(node.ID)

				// Emit cancellation events for all removed descendants (excluding the failed node itself)
				for _, removedID := range removedIDs {
					if removedID != node.ID {
						tr.emitEvent(EventNodeCancelled, removedID, map[string]any{
							"reason":        "parent_failure",
							"failed_parent": node.ID.String(),
						})

						// Increment failed count for each cancelled child
						tr.mu.Lock()
						tr.failedCount++
						tr.mu.Unlock()
					}
				}
			} else {
				// Isolate: just remove the failed node, children proceed normally
				tr.graph.Remove(node.ID)
			}
		} else {
			tr.emitEvent(EventNodeCompleted, node.ID, map[string]any{
				"duration":    result.Duration.String(),
				"tokens_used": result.TokensUsed,
			})

			// Increment completed count
			tr.mu.Lock()
			tr.completedCount++
			tr.mu.Unlock()

			// Always remove successful nodes normally
			tr.graph.Remove(node.ID)
		}

		// Notify waiting goroutines that graph space is available
		// Non-blocking send to avoid blocking the callback
		select {
		case tr.graphSpaceNotify <- struct{}{}:
		default:
			// Channel full or no one waiting, that's fine
		}
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

// listenForWorkNodePushRequests processes incoming push requests and adds nodes to the graph
// with backpressure control based on maxGraphSize
func (tr *TurboRun) listenForWorkNodePushRequests() {
	var blocked bool

	for {
		select {
		case <-tr.quit:
			return

		case req := <-tr.pushChan:
			// Wait if graph is at max capacity (event-driven, no polling)
			for tr.maxGraphSize > 0 && tr.graph.Size() >= tr.maxGraphSize {
				if !blocked {
					blocked = true
					tr.logger.Printf("TurboRun %s: Graph at max capacity (%d/%d), waiting for nodes to complete...",
						tr.uniqueID, tr.graph.Size(), tr.maxGraphSize)
					tr.emitEvent(EventGraphFull, uuid.Nil, map[string]any{
						"graph_size": tr.graph.Size(),
						"max_size":   tr.maxGraphSize,
					})
				}

				// Wait for notification that space is available
				select {
				case <-tr.graphSpaceNotify:
					// Space might be available, loop will check again
				case <-tr.quit:
					return // Shutdown during wait
				}
			}

			// Log resume if we were blocked
			if blocked {
				blocked = false
				tr.logger.Printf("TurboRun %s: Graph has space, resuming (%d/%d)",
					tr.uniqueID, tr.graph.Size(), tr.maxGraphSize)
				tr.emitEvent(EventGraphResumed, uuid.Nil, map[string]any{
					"graph_size": tr.graph.Size(),
					"max_size":   tr.maxGraphSize,
				})
			}

			// Add to graph
			tr.graph.Add(req.node, req.dependencies)

			depStrings := make([]string, len(req.dependencies))
			for i, dep := range req.dependencies {
				depStrings[i] = dep.String()
			}

			tr.emitEvent(EventNodeCreated, req.node.ID, map[string]any{
				"dependencies":     depStrings,
				"estimated_tokens": req.node.GetEstimatedTokens(),
				"provider":         string(req.node.GetProvider()),
			})
		}
	}
}

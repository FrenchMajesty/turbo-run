package turbo_run

import (
	"math/rand"
	"os"
	"time"

	"github.com/FrenchMajesty/turbo-run/utils/priority_queue"
	"github.com/google/uuid"
)

// Start starts the turbo runner
func (tr *TurboRun) Start() {
	// Start core goroutines with WaitGroup tracking
	tr.wg.Add(5)
	go func() {
		defer tr.wg.Done()
		instance.listenForReadyNodes()
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
		instance.listenForPushRequests()
	}()

	// Start analytics logging if in dev or testing environment
	env := os.Getenv("ENV")
	if env == "dev" || env == "testing" {
		tr.wg.Add(1)
		go func() {
			defer tr.wg.Done()
			instance.startAnalyticsLogger()
		}()
	}

	time.Sleep(10 * time.Millisecond) // give time for the goroutines to start
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
			tr.launchpad <- struct{}{}
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
		} else {
			tr.emitEvent(EventNodeCompleted, node.ID, map[string]any{
				"duration":    result.Duration.String(),
				"tokens_used": result.TokensUsed,
			})

			// Increment completed count
			tr.mu.Lock()
			tr.completedCount++
			tr.mu.Unlock()
		}

		tr.graph.Remove(node.ID)

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

// listenForPushRequests processes incoming push requests and adds nodes to the graph
// with backpressure control based on maxGraphSize
func (tr *TurboRun) listenForPushRequests() {
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

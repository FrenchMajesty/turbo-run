package turbo_run

import (
	"fmt"
	"sync"

	"github.com/FrenchMajesty/turbo-run/clients/groq"
	openai "github.com/openai/openai-go/v2"
)

type workerPool struct {
	wg              sync.WaitGroup
	workerState     map[int]bool
	busyWorkers     int
	workerCount     int
	pool            chan *WorkNode
	mu              sync.RWMutex
	quit            chan struct{}
	groq            *groq.GroqClientInterface
	openai          *openai.Client
	workerStateChan chan<- int
}

func NewWorkerPool(workersCount int, groq *groq.GroqClientInterface, openai *openai.Client, workerStateChan chan<- int) *workerPool {
	pool := &workerPool{
		wg:              sync.WaitGroup{},
		workerState:     make(map[int]bool, workersCount),
		workerCount:     workersCount,
		quit:            make(chan struct{}),
		pool:            make(chan *WorkNode, workersCount*2),
		groq:            groq,
		openai:          openai,
		workerStateChan: workerStateChan,
	}

	pool.start(workersCount)
	return pool
}

// Dispatch dispatches a node to the worker pool
func (wp *workerPool) Dispatch(node *WorkNode) {
	wp.pool <- node
}

// Stop stops the worker pool
func (wp *workerPool) Stop() {
	close(wp.quit)
	wp.wg.Wait()
}

// GetSize returns the number of queued jobs in the worker pool
func (wp *workerPool) GetSize() int {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return len(wp.pool)
}

// GetWorkerCount returns the total number of workers in the pool
func (wp *workerPool) GetWorkerCount() int {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.workerCount
}

// GetBusyWorkers returns the number of busy workers
func (wp *workerPool) GetBusyWorkers() int {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.busyWorkers
}

// start starts the worker pool by creating the workers
func (wp *workerPool) start(workersCount int) {
	for i := 0; i < workersCount; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// worker is the worker function that will be used to process the nodes
func (wp *workerPool) worker(workerID int) {
	defer wp.wg.Done()

	for {
		select {
		case node := <-wp.pool:
			wp.changeBusyState(workerID, true)

			// Recover from panics
			func() {
				defer wp.changeBusyState(workerID, false) // Mark not-busy when work completes

				if r := recover(); r != nil {
					result := RunResult{
						Error: fmt.Errorf("panic in node's work function: %v", r),
					}
					node.EmitResult(result)
					return
				}

				if node.workFn == nil {
					result := RunResult{
						Error: fmt.Errorf("work function is nil"),
					}
					node.EmitResult(result)
					return
				}

				node.SetStatus(WorkNodeStatusRunning)
				result := node.workFn(node, wp.groq, wp.openai)
				node.EmitResult(result)
				if result.Error == nil {
					node.SetStatus(WorkNodeStatusCompleted)
				} else {
					node.SetStatus(WorkNodeStatusFailed)
				}
			}()

		case <-wp.quit:
			return // Shutdown signal
		}
	}
}

// IsBusy returns true if any of the workers are busy
func (wp *workerPool) IsBusy() bool {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.busyWorkers > 0
}

// changeBusyState changes the busy state of the worker
func (wp *workerPool) changeBusyState(workerID int, busy bool) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	wp.workerState[workerID] = busy

	if busy {
		wp.busyWorkers++
	} else {
		wp.busyWorkers--
	}

	// Notify TurboRun of worker state change (non-blocking)
	if wp.workerStateChan != nil {
		select {
		case wp.workerStateChan <- wp.busyWorkers:
			// Sent successfully
		default:
			// Channel full, skip this update to avoid blocking workers
		}
	}
}

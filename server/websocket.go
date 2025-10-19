package server

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/FrenchMajesty/turbo-run/turbo_run"
	socketio "github.com/doquangtan/socketio/v4"
	"github.com/gofiber/fiber/v2"
)

var (
	once              sync.Once
	io                *socketio.Io
	turboRun          *turbo_run.TurboRun
	workloadGenerator func(*turbo_run.TurboRun)
	isProcessing      bool
	isGraphPrepared   bool
	mutex             sync.RWMutex
	stopMonitoring    chan struct{}
)

// Initialize sets up the package-level variables
func Initialize(tr *turbo_run.TurboRun, workloadGen func(*turbo_run.TurboRun)) {
	turboRun = tr
	workloadGenerator = workloadGen
	isProcessing = false
	isGraphPrepared = false
	stopMonitoring = make(chan struct{})
}

// initEventListeners initializes the Socket.IO event handlers
func initEventListeners(io *socketio.Io) {
	now := time.Now()

	io.OnConnection(func(socket *socketio.Socket) {
		log.Printf("✅ Client connected: %s", socket.Id)

		// Send initial stats on connection
		stats := turboRun.GetStats()
		statsJSON, _ := json.Marshal(stats)
		socket.Emit("initial_stats", string(statsJSON))

		// Handle prepare_graph event from client
		socket.On("prepare_graph", func(ep *socketio.EventPayload) {
			log.Println("Received prepare_graph request from client")
			PrepareGraph()
		})

		// Handle start_processing event from client
		socket.On("start_processing", func(ep *socketio.EventPayload) {
			log.Println("Received start_processing request from client")
			StartProcessing()
		})

		// Handle disconnect
		socket.On("disconnect", func(ep *socketio.EventPayload) {
			log.Printf("Client disconnected: %s", socket.Id)
		})
	})

	log.Printf("Socket.IO server initialized in %s", time.Since(now))
}

// PrepareGraph prepares the graph by calling the workload generator.
// If already processing, it will cancel the current processing first.
func PrepareGraph() {
	mutex.Lock()

	// If already processing, cancel it first
	if isProcessing {
		mutex.Unlock()
		log.Println("Cancelling current processing before preparing new graph...")

		if turboRun != nil {
			turboRun.Reset()
		}

		// Stop the monitoring goroutine
		select {
		case stopMonitoring <- struct{}{}:
		default:
		}

		mutex.Lock()
		isProcessing = false
		isGraphPrepared = false

		// Notify clients that processing was cancelled
		if io != nil {
			io.Emit("graph_cancelled", "")
		}
	}

	mutex.Unlock()

	log.Println("Preparing graph...")

	// Notify clients that graph preparation has started
	if io != nil {
		io.Emit("graph_preparing", "")
	}

	// Run workload generation in goroutine
	// TurboRun is paused by default, so nodes will be queued but not processed
	go func() {
		if workloadGenerator != nil && turboRun != nil {
			// Ensure TurboRun is paused before adding nodes
			turboRun.Pause()

			// Start periodic stats broadcasting during graph preparation
			statsTicker := time.NewTicker(200 * time.Millisecond)
			stopStats := make(chan struct{})
			go func() {
				for {
					select {
					case <-statsTicker.C:
						BroadcastStats()
					case <-stopStats:
						statsTicker.Stop()
						return
					}
				}
			}()

			// Generate the workload (adds nodes to graph)
			workloadGenerator(turboRun)

			// Stop stats broadcasting
			close(stopStats)

			mutex.Lock()
			isGraphPrepared = true
			mutex.Unlock()

			log.Println("Graph prepared successfully (nodes queued, waiting for start)")

			// Broadcast final stats
			BroadcastStats()

			// Notify clients that graph is prepared
			if io != nil {
				io.Emit("graph_prepared", "")
			}
		}
	}()
}

// StartProcessing resumes TurboRun to start processing the prepared graph
func StartProcessing() {
	mutex.Lock()

	if !isGraphPrepared {
		mutex.Unlock()
		log.Println("Cannot start processing - graph not prepared")
		return
	}

	if isProcessing {
		mutex.Unlock()
		log.Println("Already processing, ignoring start request")
		return
	}

	isProcessing = true
	mutex.Unlock()

	log.Println("Starting workload processing...")

	// Notify clients that processing has started
	if io != nil {
		io.Emit("processing_started", "")
	}

	// Resume TurboRun to start processing the queued nodes
	if turboRun != nil {
		turboRun.Resume()
	}

	// Start monitoring for completion
	go monitorProcessingCompletion()
}

// Start begins broadcasting TurboRun events to all connected clients
func Start() {
	go broadcastEvents()
}

// broadcastEvents listens to TurboRun event channel and broadcasts to all clients
func broadcastEvents() {
	if turboRun == nil {
		log.Println("TurboRun instance is nil, cannot broadcast events")
		return
	}

	eventChan := turboRun.GetEventChan()

	for event := range eventChan {
		if io == nil {
			continue
		}

		// Handle special events that trigger stats updates
		if event.Type == "worker_state_changed" {
			BroadcastStats()
			// Don't broadcast this internal event to clients
			continue
		}

		// Convert event to JSON
		eventJSON, err := json.Marshal(event)
		if err != nil {
			log.Printf("Error marshaling event: %v", err)
			continue
		}

		// Broadcast to all connected clients
		io.Emit("turbo_event", string(eventJSON))
	}
}

// SetupHandlers sets up the Socket.IO routes on a Fiber app
func SetupHandlers(app *fiber.App) {
	once.Do(func() {
		io = socketio.New()
	})

	app.Use("/", io.FiberMiddleware) // This middleware is to attach socketio to the context of fiber
	app.Route("/socket.io", io.FiberRoute)

	initEventListeners(io)
}

// BroadcastStats sends current stats to all connected clients
func BroadcastStats() {
	if io == nil || turboRun == nil {
		return
	}

	stats := turboRun.GetStats()
	statsJSON, err := json.Marshal(stats)
	if err != nil {
		log.Printf("Error marshaling stats: %v", err)
		return
	}

	io.Emit("stats_update", string(statsJSON))
}

// monitorProcessingCompletion monitors the graph for completion and notifies clients
func monitorProcessingCompletion() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	log.Println("Started monitoring for processing completion...")

	for {
		select {
		case <-stopMonitoring:
			log.Println("Stopped monitoring for processing completion")
			return

		case <-ticker.C:
			if turboRun == nil {
				continue
			}

			stats := turboRun.GetStats()

			// Processing is complete when:
			// - Graph is empty (all nodes processed)
			// - Priority queue is empty (no nodes waiting)
			// - No workers are busy
			// - At least one node was launched (to avoid false positives on empty graphs)
			if stats.GraphSize == 0 &&
				stats.PriorityQueueSize == 0 &&
				stats.WorkersPoolBusy == 0 &&
				stats.LaunchedCount > 0 {

				log.Printf("Processing completed! Launched: %d, Completed: %d, Failed: %d",
					stats.LaunchedCount, stats.CompletedCount, stats.FailedCount)

				mutex.Lock()
				isProcessing = false
				isGraphPrepared = false
				mutex.Unlock()

				// Notify clients that processing is complete
				if io != nil {
					io.Emit("processing_completed", "")
				}

				return
			}
		}
	}
}

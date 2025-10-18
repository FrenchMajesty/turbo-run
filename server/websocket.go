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
	mutex             sync.RWMutex
)

// Initialize sets up the package-level variables
func Initialize(tr *turbo_run.TurboRun, workloadGen func(*turbo_run.TurboRun)) {
	turboRun = tr
	workloadGenerator = workloadGen
	isProcessing = false
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

// StartProcessing triggers the workload generation if not already processing
func StartProcessing() {
	mutex.Lock()
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

	// Run workload generation in goroutine
	go func() {
		if workloadGenerator != nil && turboRun != nil {
			workloadGenerator(turboRun)
		}
	}()
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

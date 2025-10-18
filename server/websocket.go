package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/FrenchMajesty/turbo-run/turbo_run"
	socketio "github.com/doquangtan/socketio/v4"
)

type WebSocketServer struct {
	io                *socketio.Io
	turboRun          *turbo_run.TurboRun
	workloadGenerator func(*turbo_run.TurboRun)
	isProcessing      bool
}

// NewWebSocketServer creates a new WebSocket server for broadcasting TurboRun events
func NewWebSocketServer(tr *turbo_run.TurboRun, workloadGen func(*turbo_run.TurboRun)) *WebSocketServer {
	io := socketio.New()

	ws := &WebSocketServer{
		io:                io,
		turboRun:          tr,
		workloadGenerator: workloadGen,
		isProcessing:      false,
	}

	// Setup connection handler
	io.OnConnection(func(socket *socketio.Socket) {
		log.Printf("New WebSocket client connected: %s", socket.Id)

		// Send initial stats on connection
		stats := tr.GetStats()
		statsJSON, _ := json.Marshal(stats)
		socket.Emit("initial_stats", string(statsJSON))

		// Handle start_processing event from client
		socket.On("start_processing", func(event *socketio.EventPayload) {
			log.Println("Received start_processing request from client")
			ws.StartProcessing()
		})

		// Handle disconnect
		socket.On("disconnect", func(event *socketio.EventPayload) {
			log.Printf("Client disconnected: %s", socket.Id)
		})
	})

	return ws
}

// StartProcessing triggers the workload generation if not already processing
func (ws *WebSocketServer) StartProcessing() {
	if ws.isProcessing {
		log.Println("Already processing, ignoring start request")
		return
	}

	ws.isProcessing = true
	log.Println("Starting workload processing...")

	// Notify clients that processing has started
	ws.io.Emit("processing_started", "")

	// Run workload generation in goroutine
	go func() {
		if ws.workloadGenerator != nil {
			ws.workloadGenerator(ws.turboRun)
		}
	}()
}

// Start begins broadcasting TurboRun events to all connected clients
func (ws *WebSocketServer) Start() {
	go ws.broadcastEvents()
}

// broadcastEvents listens to TurboRun event channel and broadcasts to all clients
func (ws *WebSocketServer) broadcastEvents() {
	eventChan := ws.turboRun.GetEventChan()

	for event := range eventChan {
		// Convert event to JSON
		eventJSON, err := json.Marshal(event)
		if err != nil {
			log.Printf("Error marshaling event: %v", err)
			continue
		}

		// Broadcast to all connected clients
		ws.io.Emit("turbo_event", string(eventJSON))
	}
}

// HttpHandler returns the HTTP handler for Socket.IO
func (ws *WebSocketServer) HttpHandler() http.Handler {
	return ws.io.HttpHandler()
}

// BroadcastStats sends current stats to all connected clients
func (ws *WebSocketServer) BroadcastStats() {
	stats := ws.turboRun.GetStats()
	statsJSON, err := json.Marshal(stats)
	if err != nil {
		log.Printf("Error marshaling stats: %v", err)
		return
	}

	ws.io.Emit("stats_update", string(statsJSON))
}

// Shutdown gracefully closes the WebSocket server
func (ws *WebSocketServer) Shutdown() {
	fmt.Println("Shutting down WebSocket server...")
	// Socket.IO cleanup happens automatically
}

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
	io       *socketio.Io
	turboRun *turbo_run.TurboRun
}

// NewWebSocketServer creates a new WebSocket server for broadcasting TurboRun events
func NewWebSocketServer(tr *turbo_run.TurboRun) *WebSocketServer {
	io := socketio.New()

	ws := &WebSocketServer{
		io:       io,
		turboRun: tr,
	}

	// Setup connection handler
	io.OnConnection(func(socket *socketio.Socket) {
		log.Printf("New WebSocket client connected: %s", socket.Id)

		// Send initial stats on connection
		stats := tr.GetStats()
		statsJSON, _ := json.Marshal(stats)
		socket.Emit("initial_stats", string(statsJSON))

		// Handle disconnect
		socket.On("disconnect", func(event *socketio.EventPayload) {
			log.Printf("Client disconnected: %s", socket.Id)
		})
	})

	return ws
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

package uds

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FrenchMajesty/turbo-run/rate_limit"
)

const (
	socketPath = "/tmp/turbo-run-rate-limiter.sock"
)

// usageData tracks token and request consumption
type usageData struct {
	Tokens   int
	Requests int
}

// Manager is the server that manages rate limits across processes via Unix Domain Socket
type Manager struct {
	state         map[string]usageData
	currentMinute string
	budgets       map[string]rate_limit.RateLimit
	mu            sync.RWMutex
	listener      net.Listener
	clients       map[net.Conn]bool
	clientsMu     sync.Mutex
	quit          chan struct{}
}

// NewManager creates a new rate limit manager
func NewManager() *Manager {
	return &Manager{
		state:         make(map[string]usageData),
		currentMinute: time.Now().Truncate(time.Minute).Format(time.RFC3339),
		budgets: map[string]rate_limit.RateLimit{
			"groq":   rate_limit.GroqRateLimit,
			"openai": rate_limit.OpenAIRateLimit,
		},
		clients: make(map[net.Conn]bool),
		quit:    make(chan struct{}),
	}
}

// Start starts the rate limit manager server
func (m *Manager) Start() error {
	// Remove existing socket file if it exists
	os.Remove(socketPath)

	// Create Unix domain socket listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	m.listener = listener

	// Set socket file permissions (readable/writable by all for cross-process access)
	if err := os.Chmod(socketPath, 0666); err != nil {
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	fmt.Printf("Rate limit manager started on %s\n", socketPath)

	// Start minute timer
	go m.startMinuteTimer()

	// Start client connection handler
	go m.acceptConnections()

	// Start idle monitor (exits when no clients for 5 seconds)
	go m.monitorIdleState()

	return nil
}

// Stop stops the rate limit manager
func (m *Manager) Stop() {
	close(m.quit)
	if m.listener != nil {
		m.listener.Close()
	}
	os.Remove(socketPath)
}

// acceptConnections accepts incoming client connections
func (m *Manager) acceptConnections() {
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			select {
			case <-m.quit:
				return
			default:
				fmt.Printf("Accept error: %v\n", err)
				continue
			}
		}

		m.clientsMu.Lock()
		m.clients[conn] = true
		m.clientsMu.Unlock()

		go m.handleClient(conn)
	}
}

// handleClient handles a single client connection
func (m *Manager) handleClient(conn net.Conn) {
	defer func() {
		conn.Close()
		m.clientsMu.Lock()
		delete(m.clients, conn)
		m.clientsMu.Unlock()
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		response := m.handleCommand(line)
		conn.Write([]byte(response + "\n"))
	}
}

// handleCommand processes a single command from a client
func (m *Manager) handleCommand(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "ERROR empty command"
	}

	switch parts[0] {
	case "BUDGET":
		if len(parts) != 2 {
			return "ERROR invalid BUDGET command"
		}
		tokens, requests := m.getBudget(parts[1])
		return fmt.Sprintf("%d %d", tokens, requests)

	case "CONSUME":
		if len(parts) != 4 {
			return "ERROR invalid CONSUME command"
		}
		provider := parts[1]
		tokens, err1 := strconv.Atoi(parts[2])
		requests, err2 := strconv.Atoi(parts[3])
		if err1 != nil || err2 != nil {
			return "ERROR invalid numbers"
		}
		m.recordConsumption(provider, tokens, requests)
		return "OK"

	case "RESET":
		m.resetMinute()
		return "OK"

	case "SET_BUDGET":
		if len(parts) != 4 {
			return "ERROR invalid SET_BUDGET command"
		}
		provider := parts[1]
		tokens, err1 := strconv.Atoi(parts[2])
		requests, err2 := strconv.Atoi(parts[3])
		if err1 != nil || err2 != nil {
			return "ERROR invalid numbers"
		}
		m.setBudget(provider, tokens, requests)
		return "OK"

	case "TIME_UNTIL_RESET":
		duration := m.timeUntilReset()
		return fmt.Sprintf("%d", duration.Milliseconds())

	case "PING":
		return "PONG"

	default:
		return "ERROR unknown command"
	}
}

// getBudget returns available budget for a provider
func (m *Manager) getBudget(provider string) (int, int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checkAndResetMinute()

	usage := m.state[provider]
	budget := m.budgets[provider]

	tokensAvailable := budget.TPM - usage.Tokens
	requestsAvailable := budget.RPM - usage.Requests

	if tokensAvailable < 0 {
		tokensAvailable = 0
	}
	if requestsAvailable < 0 {
		requestsAvailable = 0
	}

	return tokensAvailable, requestsAvailable
}

// recordConsumption records usage for a provider
func (m *Manager) recordConsumption(provider string, tokens, requests int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checkAndResetMinute()

	usage := m.state[provider]
	usage.Tokens += tokens
	usage.Requests += requests
	m.state[provider] = usage
}

// setBudget sets custom budget for a provider (for testing)
func (m *Manager) setBudget(provider string, tokens, requests int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.budgets[provider] = rate_limit.RateLimit{
		TPM: tokens,
		RPM: requests,
	}
}

// resetMinute resets the current minute's usage
func (m *Manager) resetMinute() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.currentMinute = time.Now().Truncate(time.Minute).Format(time.RFC3339)
	m.state = make(map[string]usageData)
}

// timeUntilReset returns duration until next minute boundary
func (m *Manager) timeUntilReset() time.Duration {
	now := time.Now()
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	return nextMinute.Sub(now)
}

// checkAndResetMinute resets state if we're in a new minute
// Note: caller must hold the lock
func (m *Manager) checkAndResetMinute() {
	currentMinute := time.Now().Truncate(time.Minute).Format(time.RFC3339)
	if m.currentMinute != currentMinute {
		m.currentMinute = currentMinute
		m.state = make(map[string]usageData)
	}
}

// startMinuteTimer starts a timer that resets usage every minute
func (m *Manager) startMinuteTimer() {
	now := time.Now()
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	timeUntilNextMinute := nextMinute.Sub(now)

	time.Sleep(timeUntilNextMinute)

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.quit:
			return
		case <-ticker.C:
			m.resetMinute()
		}
	}
}

// monitorIdleState shuts down manager if no clients connected for 5 seconds
func (m *Manager) monitorIdleState() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	idleSince := time.Time{}

	for {
		select {
		case <-m.quit:
			return
		case <-ticker.C:
			m.clientsMu.Lock()
			clientCount := len(m.clients)
			m.clientsMu.Unlock()

			if clientCount == 0 {
				if idleSince.IsZero() {
					idleSince = time.Now()
				} else if time.Since(idleSince) > 5*time.Second {
					fmt.Println("No clients connected for 5 seconds, shutting down manager")
					m.Stop()
					os.Exit(0)
				}
			} else {
				idleSince = time.Time{}
			}
		}
	}
}

// RunServer is the entry point for running the manager as a subprocess
func RunServer() {
	manager := NewManager()
	if err := manager.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start rate limit manager: %v\n", err)
		os.Exit(1)
	}

	// Wait for interrupt signal
	select {}
}

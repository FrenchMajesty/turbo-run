package uds

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FrenchMajesty/turbo-run/clients/groq"
	"github.com/FrenchMajesty/turbo-run/rate_limit"
)

// Client manages rate limits via Unix Domain Socket communication with the Manager
type Client struct {
	conn    net.Conn
	mu      sync.Mutex
	reader  *bufio.Reader
	budgets map[groq.Provider]rate_limit.RateLimit
}

// NewClient creates a new UDS rate limiter client
func NewClient() *Client {
	budgets := map[groq.Provider]rate_limit.RateLimit{
		groq.ProviderGroq:   rate_limit.GroqRateLimit,
		groq.ProviderOpenAI: rate_limit.OpenAIRateLimit,
	}

	client := &Client{
		budgets: budgets,
	}

	// Ensure manager is running and connect
	if err := client.ensureConnection(); err != nil {
		// If we can't connect, we'll fall back to local tracking
		fmt.Fprintf(os.Stderr, "Warning: Failed to connect to rate limit manager: %v\n", err)
	}

	return client
}

// ensureConnection ensures the manager is running and establishes a connection
func (c *Client) ensureConnection() error {
	// Try to connect first
	conn, err := c.dialManager()
	if err == nil {
		c.conn = conn
		c.reader = bufio.NewReader(conn)
		return nil
	}

	// Manager not running, try to start it
	if err := c.startManager(); err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}

	// Wait a bit for manager to start
	time.Sleep(100 * time.Millisecond)

	// Try connecting again
	conn, err = c.dialManager()
	if err != nil {
		return fmt.Errorf("failed to connect after starting manager: %w", err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	return nil
}

// dialManager attempts to connect to the rate limit manager
func (c *Client) dialManager() (net.Conn, error) {
	return net.Dial("unix", socketPath)
}

// startManager starts the rate limit manager as a background process
func (c *Client) startManager() error {
	// Get the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Start manager as background process
	cmd := exec.Command(execPath, "rate-limiter")
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Detach from parent process (platform-specific)
	cmd.SysProcAttr = getSysProcAttr()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start manager process: %w", err)
	}

	// Don't wait for the process
	go cmd.Wait()

	return nil
}

// sendCommand sends a command to the manager and returns the response
func (c *Client) sendCommand(command string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return "", fmt.Errorf("not connected to manager")
	}

	// Send command
	if _, err := c.conn.Write([]byte(command + "\n")); err != nil {
		// Connection broken, try to reconnect
		c.conn.Close()
		c.conn = nil
		c.reader = nil
		if err := c.ensureConnection(); err != nil {
			return "", fmt.Errorf("failed to reconnect: %w", err)
		}
		// Retry the command
		if _, err := c.conn.Write([]byte(command + "\n")); err != nil {
			return "", fmt.Errorf("failed to send command after reconnect: %w", err)
		}
	}

	// Read response
	response, err := c.reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return strings.TrimSpace(response), nil
}

// BudgetAvailable returns available budget for the given provider
func (c *Client) BudgetAvailable(provider groq.Provider) (tokensAvailable int, requestsAvailable int) {
	providerKey := c.getProviderKey(provider)
	response, err := c.sendCommand(fmt.Sprintf("BUDGET %s", providerKey))
	if err != nil {
		// Fallback to full budget if we can't reach manager
		budget := c.budgets[provider]
		return budget.TPM, budget.RPM
	}

	// Parse response: "tokens requests"
	parts := strings.Fields(response)
	if len(parts) != 2 {
		budget := c.budgets[provider]
		return budget.TPM, budget.RPM
	}

	tokens, err1 := strconv.Atoi(parts[0])
	requests, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		budget := c.budgets[provider]
		return budget.TPM, budget.RPM
	}

	return tokens, requests
}

// RecordConsumption records usage for the given provider
func (c *Client) RecordConsumption(provider groq.Provider, tokens int, requests int) error {
	providerKey := c.getProviderKey(provider)
	_, err := c.sendCommand(fmt.Sprintf("CONSUME %s %d %d", providerKey, tokens, requests))
	return err
}

// TimeUntilReset returns time until the next minute boundary
func (c *Client) TimeUntilReset() time.Duration {
	response, err := c.sendCommand("TIME_UNTIL_RESET")
	if err != nil {
		// Fallback to local calculation
		now := time.Now()
		nextMinute := now.Truncate(time.Minute).Add(time.Minute)
		return nextMinute.Sub(now)
	}

	ms, err := strconv.ParseInt(response, 10, 64)
	if err != nil {
		now := time.Now()
		nextMinute := now.Truncate(time.Minute).Add(time.Minute)
		return nextMinute.Sub(now)
	}

	return time.Duration(ms) * time.Millisecond
}

// SetBudgetForTests sets custom budgets for testing
func (c *Client) SetBudgetForTests(provider groq.Provider, tokens, requests int) error {
	providerKey := c.getProviderKey(provider)
	_, err := c.sendCommand(fmt.Sprintf("SET_BUDGET %s %d %d", providerKey, tokens, requests))
	return err
}

// Close closes the connection to the manager
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// getProviderKey returns a string key for the provider
func (c *Client) getProviderKey(provider groq.Provider) string {
	switch provider {
	case groq.ProviderGroq:
		return "groq"
	case groq.ProviderOpenAI:
		return "openai"
	default:
		return "unknown"
	}
}

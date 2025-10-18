package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FrenchMajesty/turbo-run/clients/groq"
	"github.com/FrenchMajesty/turbo-run/server"
	"github.com/FrenchMajesty/turbo-run/turbo_run"
	"github.com/google/uuid"
)

// MockGroqClient implements a mock Groq client for demo purposes
type MockGroqClient struct{}

func (m *MockGroqClient) ChatCompletion(ctx context.Context, req groq.ChatCompletionRequest) (*groq.ChatCompletionResponse, error) {
	// Simulate processing time (100ms to 2000ms)
	processingTime := time.Duration(100+rand.Intn(1900)) * time.Millisecond
	time.Sleep(processingTime)

	// Simulate occasional errors (10% failure rate)
	if rand.Float32() < 0.1 {
		return nil, fmt.Errorf("generation error: mock API failure")
	}

	// Return a mock response
	content := "Mock response from Groq"
	return &groq.ChatCompletionResponse{
		Choices: []groq.ChatCompletionChoice{
			{
				Message: groq.ChatMessage{
					Role:    groq.MessageRoleAssistant,
					Content: &content,
				},
			},
		},
		Usage: groq.ChatCompletionUsage{
			TotalTokens: rand.Intn(500) + 100,
		},
	}, nil
}

func (m *MockGroqClient) ChatCompletionStream(ctx context.Context, req groq.ChatCompletionRequest, callback func(token string)) (*groq.StreamingResult, error) {
	// Not implemented for demo
	return nil, fmt.Errorf("streaming not implemented in mock")
}

func main() {
	// Set environment to dev for verbose logging
	os.Setenv("ENV", "dev")

	fmt.Println("🚀 TurboRun WebSocket Visualization Demo")
	fmt.Println("=========================================")

	// Initialize mock clients
	mockGroq := &MockGroqClient{}

	// Initialize TurboRun
	turboRun := turbo_run.NewTurboRun(mockGroq, nil)
	defer turboRun.Stop()

	// Override budgets for demo (allow unlimited requests)
	turboRun.OverrideBudgetsForTests(1000000, 1000000, 10000, 10000)

	// Initialize WebSocket server with workload generator
	wsServer := server.NewWebSocketServer(turboRun, generateDemoWorkload)
	wsServer.Start()

	// Setup HTTP server
	http.Handle("/socket.io/", wsServer.HttpHandler())
	http.Handle("/", http.FileServer(http.Dir("./static")))

	// Start HTTP server in goroutine
	go func() {
		fmt.Println("📡 WebSocket server running on http://localhost:8080")
		fmt.Println("🎨 Open http://localhost:8080 in your browser to see the visualization")
		fmt.Println("⏸️  Processing will start when you click 'Start Processing' in the UI")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n👋 Shutting down gracefully...")
	wsServer.Shutdown()
}

// generateDemoWorkload creates a complex dependency graph of WorkNodes
func generateDemoWorkload(tr *turbo_run.TurboRun) {
	fmt.Println("\n📊 Generating demo workload...")

	// Create base request template
	createRequest := func(content string, tokens int) groq.ChatCompletionRequest {
		contentPtr := content
		return groq.ChatCompletionRequest{
			Model: "llama-3.1-70b-versatile",
			Messages: []groq.ChatMessage{
				{Role: groq.MessageRoleUser, Content: &contentPtr},
			},
		}
	}

	// Pattern 1: Single root node
	fmt.Println("  • Creating root analysis node...")
	rootNode := turbo_run.NewRetryableWorkNodeForGroq(
		createRequest("Analyze the dataset", 500),
	)
	tr.Push(rootNode)

	// Pattern 2: Fan-out - multiple nodes depend on root
	fmt.Println("  • Creating fan-out pattern (5 parallel processing nodes)...")
	var parallelNodes []*turbo_run.WorkNode
	for i := 0; i < 5; i++ {
		node := turbo_run.NewRetryableWorkNodeForGroq(
			createRequest(fmt.Sprintf("Process segment %d", i+1), 300),
		)
		tr.PushWithDependencies(node, []uuid.UUID{rootNode.ID})
		parallelNodes = append(parallelNodes, node)
	}

	// Pattern 3: Fan-in - aggregation node depends on all parallel nodes
	fmt.Println("  • Creating fan-in aggregation node...")
	parallelIDs := make([]uuid.UUID, len(parallelNodes))
	for i, node := range parallelNodes {
		parallelIDs[i] = node.ID
	}
	aggregateNode := turbo_run.NewRetryableWorkNodeForGroq(
		createRequest("Aggregate all segment results", 600),
	)
	tr.PushWithDependencies(aggregateNode, parallelIDs)

	// Pattern 4: Chain - sequential dependencies
	fmt.Println("  • Creating sequential chain (3 nodes)...")
	chainNode1 := turbo_run.NewRetryableWorkNodeForGroq(
		createRequest("Chain step 1: Extract features", 250),
	)
	tr.Push(chainNode1)

	chainNode2 := turbo_run.NewRetryableWorkNodeForGroq(
		createRequest("Chain step 2: Transform features", 250),
	)
	tr.PushWithDependencies(chainNode2, []uuid.UUID{chainNode1.ID})

	chainNode3 := turbo_run.NewRetryableWorkNodeForGroq(
		createRequest("Chain step 3: Validate results", 250),
	)
	tr.PushWithDependencies(chainNode3, []uuid.UUID{chainNode2.ID})

	// Pattern 5: Diamond - two paths converging
	fmt.Println("  • Creating diamond pattern...")
	diamondRoot := turbo_run.NewRetryableWorkNodeForGroq(
		createRequest("Diamond root: Initialize", 200),
	)
	tr.Push(diamondRoot)

	diamondLeft := turbo_run.NewRetryableWorkNodeForGroq(
		createRequest("Diamond left path: Process A", 300),
	)
	tr.PushWithDependencies(diamondLeft, []uuid.UUID{diamondRoot.ID})

	diamondRight := turbo_run.NewRetryableWorkNodeForGroq(
		createRequest("Diamond right path: Process B", 300),
	)
	tr.PushWithDependencies(diamondRight, []uuid.UUID{diamondRoot.ID})

	diamondMerge := turbo_run.NewRetryableWorkNodeForGroq(
		createRequest("Diamond merge: Combine A and B", 400),
	)
	tr.PushWithDependencies(diamondMerge, []uuid.UUID{diamondLeft.ID, diamondRight.ID})

	// Pattern 6: Independent nodes (no dependencies)
	fmt.Println("  • Creating independent nodes (3 nodes)...")
	for i := 0; i < 3; i++ {
		node := turbo_run.NewRetryableWorkNodeForGroq(
			createRequest(fmt.Sprintf("Independent task %d", i+1), 200),
		)
		tr.Push(node)
	}

	// Final summary node depending on main aggregate and diamond
	fmt.Println("  • Creating final summary node...")
	finalNode := turbo_run.NewRetryableWorkNodeForGroq(
		createRequest("Final summary: Combine all results", 800),
	)
	tr.PushWithDependencies(finalNode, []uuid.UUID{aggregateNode.ID, diamondMerge.ID, chainNode3.ID})

	fmt.Printf("\n✅ Generated 20 WorkNodes with complex dependencies\n")
	fmt.Println("   - 1 root node")
	fmt.Println("   - 5 parallel processing nodes (fan-out)")
	fmt.Println("   - 1 aggregation node (fan-in)")
	fmt.Println("   - 3 sequential chain nodes")
	fmt.Println("   - 4 diamond pattern nodes")
	fmt.Println("   - 3 independent nodes")
	fmt.Println("   - 1 final summary node")
	fmt.Println("\n🎬 Processing started! Watch the visualization in your browser.\n")
}

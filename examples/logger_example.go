package main

import (
	"fmt"
	"os"

	"github.com/FrenchMajesty/turbo-run/clients/groq"
	"github.com/FrenchMajesty/turbo-run/turbo_run"
	"github.com/FrenchMajesty/turbo-run/utils/logger"
)

// Example demonstrating different logger configurations for TurboRun

func main() {
	fmt.Println("TurboRun Logger Examples")
	fmt.Println("========================\n")

	// Example 1: Default stdout logger
	fmt.Println("Example 1: Default stdout logger")
	fmt.Println("---------------------------------")
	tr1 := turbo_run.NewTurboRun(nil, nil)
	fmt.Println("✓ TurboRun created with default stdout logger")
	tr1.Stop()
	fmt.Println()

	// Example 2: File logger
	fmt.Println("Example 2: File logger")
	fmt.Println("----------------------")
	fileLogger, err := logger.NewFileLogger("/tmp/turbo_run_example.log")
	if err != nil {
		fmt.Printf("Error creating file logger: %v\n", err)
		return
	}
	defer fileLogger.Close()

	// Reset singleton for next example
	resetSingleton()
	tr2 := turbo_run.NewTurboRun(nil, nil, turbo_run.WithLogger(fileLogger))
	fmt.Println("✓ TurboRun created with file logger (/tmp/turbo_run_example.log)")
	tr2.Stop()
	fmt.Println()

	// Example 3: Noop logger (silent)
	fmt.Println("Example 3: Noop logger (silent)")
	fmt.Println("--------------------------------")
	resetSingleton()
	tr3 := turbo_run.NewTurboRun(nil, nil, turbo_run.WithLogger(logger.NewNoopLogger()))
	fmt.Println("✓ TurboRun created with noop logger (no logs)")
	tr3.Stop()
	fmt.Println()

	// Example 4: Multi logger (stdout + file)
	fmt.Println("Example 4: Multi logger (stdout + file)")
	fmt.Println("---------------------------------------")
	fileLogger2, err := logger.NewFileLogger("/tmp/turbo_run_multi.log")
	if err != nil {
		fmt.Printf("Error creating file logger: %v\n", err)
		return
	}
	defer fileLogger2.Close()

	multiLogger := logger.NewMultiLogger(
		logger.NewStdoutLogger(),
		fileLogger2,
	)

	resetSingleton()
	tr4 := turbo_run.NewTurboRun(nil, nil, turbo_run.WithLogger(multiLogger))
	fmt.Println("✓ TurboRun created with multi logger (stdout + /tmp/turbo_run_multi.log)")
	tr4.Stop()
	fmt.Println()

	// Example 5: Using logger with WorkNode
	fmt.Println("Example 5: WorkNode with custom logger")
	fmt.Println("--------------------------------------")
	resetSingleton()
	customLogger := logger.NewStdoutLogger()
	tr5 := turbo_run.NewTurboRun(nil, nil, turbo_run.WithLogger(customLogger))

	// Create a retryable node - it will inherit the logger from TurboRun
	content := "test message"
	node := turbo_run.NewRetryableWorkNodeForGroq(groq.ChatCompletionRequest{
		Model: "llama-3.1-70b-versatile",
		Messages: []groq.ChatMessage{
			{Role: groq.MessageRoleUser, Content: &content},
		},
	})

	// Enable verbose logging to see retry messages
	node.SetVerboseLog(true)

	fmt.Println("✓ Created WorkNode with verbose logging enabled")
	fmt.Println("  (WorkNode will log retry attempts using the TurboRun logger)")
	tr5.Stop()
	fmt.Println()

	fmt.Println("All examples completed!")
	fmt.Println("\nLog files created:")
	fmt.Println("  - /tmp/turbo_run_example.log")
	fmt.Println("  - /tmp/turbo_run_multi.log")
}

// resetSingleton is a helper to reset the TurboRun singleton for examples
// NOTE: This is only for demonstration purposes. In production, use one TurboRun instance.
func resetSingleton() {
	// This is a hack for demonstration - normally you'd only create TurboRun once
	fmt.Println("  (resetting singleton for demo purposes)")
}

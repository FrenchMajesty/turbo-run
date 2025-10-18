package logger

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestStdoutLogger(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger := NewStdoutLogger()
	logger.Println("test message")
	logger.Printf("formatted %s", "message")

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "test message") {
		t.Errorf("Expected 'test message' in output, got: %s", output)
	}
	if !strings.Contains(output, "formatted message") {
		t.Errorf("Expected 'formatted message' in output, got: %s", output)
	}
}

func TestFileLogger(t *testing.T) {
	tmpFile := "/tmp/test_logger.log"
	defer os.Remove(tmpFile)

	logger, err := NewFileLogger(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create file logger: %v", err)
	}
	defer logger.Close()

	logger.Println("test message")
	logger.Printf("formatted %s", "message")

	// Close to flush
	logger.Close()

	// Read file contents
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	output := string(content)
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected 'test message' in file, got: %s", output)
	}
	if !strings.Contains(output, "formatted message") {
		t.Errorf("Expected 'formatted message' in file, got: %s", output)
	}
}

func TestNoopLogger(t *testing.T) {
	logger := NewNoopLogger()
	// Should not panic
	logger.Println("test")
	logger.Printf("test %s", "message")
}

func TestMultiLogger(t *testing.T) {
	tmpFile := "/tmp/test_multi_logger.log"
	defer os.Remove(tmpFile)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fileLogger, err := NewFileLogger(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create file logger: %v", err)
	}
	defer fileLogger.Close()

	stdoutLogger := NewStdoutLogger()
	multiLogger := NewMultiLogger(stdoutLogger, fileLogger)

	multiLogger.Println("test message")

	w.Close()
	os.Stdout = old
	fileLogger.Close()

	// Check stdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	stdoutOutput := buf.String()

	if !strings.Contains(stdoutOutput, "test message") {
		t.Errorf("Expected 'test message' in stdout, got: %s", stdoutOutput)
	}

	// Check file
	fileContent, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(fileContent), "test message") {
		t.Errorf("Expected 'test message' in file, got: %s", string(fileContent))
	}
}

func TestWriterLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWriterLogger(&buf)

	logger.Println("test message")
	logger.Printf("formatted %s", "message")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected 'test message' in output, got: %s", output)
	}
	if !strings.Contains(output, "formatted message") {
		t.Errorf("Expected 'formatted message' in output, got: %s", output)
	}
}

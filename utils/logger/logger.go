package logger

import (
	"io"
	"log"
	"os"
)

// Logger provides a simple logging interface for TurboRun and other components.
// All implementations must be safe for concurrent use across multiple goroutines.
type Logger interface {
	// Printf logs a formatted message
	Printf(format string, args ...interface{})
	// Println logs a message with a newline
	Println(message string)
}

// StdoutLogger writes logs to stdout using the standard log package.
// Safe for concurrent use across goroutines.
type StdoutLogger struct {
	logger *log.Logger
}

// NewStdoutLogger creates a new logger that writes to stdout
func NewStdoutLogger() *StdoutLogger {
	return &StdoutLogger{
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (s *StdoutLogger) Printf(format string, args ...interface{}) {
	s.logger.Printf(format, args...)
}

func (s *StdoutLogger) Println(message string) {
	s.logger.Println(message)
}

// FileLogger writes logs to a file using O_APPEND for atomic writes.
// Safe for concurrent use across both goroutines and processes.
// Uses Go's log.Logger which has internal mutex for thread safety,
// and O_APPEND for process-safe atomic appends.
type FileLogger struct {
	logger *log.Logger
	file   *os.File
}

// NewFileLogger creates a new logger that writes to the specified file path.
// The file is opened with O_APPEND to ensure atomic writes across processes.
// Returns an error if the file cannot be opened.
func NewFileLogger(filepath string) (*FileLogger, error) {
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	return &FileLogger{
		logger: log.New(file, "", log.LstdFlags),
		file:   file,
	}, nil
}

func (f *FileLogger) Printf(format string, args ...interface{}) {
	f.logger.Printf(format, args...)
}

func (f *FileLogger) Println(message string) {
	f.logger.Println(message)
}

// Close closes the underlying file. Should be called when done with the logger.
func (f *FileLogger) Close() error {
	if f.file != nil {
		return f.file.Close()
	}
	return nil
}

// NoopLogger discards all log messages. Useful for testing or when logging is disabled.
// Safe for concurrent use across goroutines.
type NoopLogger struct{}

// NewNoopLogger creates a new logger that discards all output
func NewNoopLogger() *NoopLogger {
	return &NoopLogger{}
}

func (n *NoopLogger) Printf(format string, args ...interface{}) {
	// Discard
}

func (n *NoopLogger) Println(message string) {
	// Discard
}

// MultiLogger writes to multiple loggers simultaneously.
// Safe for concurrent use if all underlying loggers are safe.
type MultiLogger struct {
	loggers []Logger
}

// NewMultiLogger creates a logger that writes to multiple destinations
func NewMultiLogger(loggers ...Logger) *MultiLogger {
	return &MultiLogger{
		loggers: loggers,
	}
}

func (m *MultiLogger) Printf(format string, args ...interface{}) {
	for _, logger := range m.loggers {
		logger.Printf(format, args...)
	}
}

func (m *MultiLogger) Println(message string) {
	for _, logger := range m.loggers {
		logger.Println(message)
	}
}

// WriterLogger adapts any io.Writer to the Logger interface.
// Thread safety depends on the underlying writer.
type WriterLogger struct {
	logger *log.Logger
}

// NewWriterLogger creates a logger from any io.Writer
func NewWriterLogger(w io.Writer) *WriterLogger {
	return &WriterLogger{
		logger: log.New(w, "", log.LstdFlags),
	}
}

func (w *WriterLogger) Printf(format string, args ...interface{}) {
	w.logger.Printf(format, args...)
}

func (w *WriterLogger) Println(message string) {
	w.logger.Println(message)
}

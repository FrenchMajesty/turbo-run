package logger

// Logger provides a simple logging interface for TurboRun and other components.
// All implementations must be safe for concurrent use across multiple goroutines.
type Logger interface {
	// Type returns the type of the logger
	Type() LoggerType
	// Printf logs a formatted message
	Printf(format string, args ...any)
	// Println logs a message with a newline
	Println(message string)
	// Close closes the logger
	Close() error
}

type LoggerType string

const (
	LoggerTypeStdout LoggerType = "stdout"
	LoggerTypeFile   LoggerType = "file"
	LoggerTypeNoop   LoggerType = "noop"
	LoggerTypeWriter LoggerType = "writer"
)

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

func (m *MultiLogger) Printf(format string, args ...any) {
	for _, logger := range m.loggers {
		logger.Printf(format, args...)
	}
}

func (m *MultiLogger) Println(message string) {
	for _, logger := range m.loggers {
		logger.Println(message)
	}
}

func (m *MultiLogger) Close() error {
	for _, logger := range m.loggers {
		logger.Close()
	}
	return nil
}

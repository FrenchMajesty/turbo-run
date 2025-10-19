package logger

// NoopLogger discards all log messages. Useful for testing or when logging is disabled.
// Safe for concurrent use across goroutines.
type NoopLogger struct{}

var _ Logger = (*NoopLogger)(nil)

// NewNoopLogger creates a new logger that discards all output
func NewNoopLogger() *NoopLogger {
	return &NoopLogger{}
}

func (n *NoopLogger) Type() LoggerType {
	return LoggerTypeNoop
}

func (n *NoopLogger) Printf(format string, args ...any) {
	// Discard
}

func (n *NoopLogger) Println(message string) {
	// Discard
}

func (n *NoopLogger) Close() error {
	return nil
}

package logger

import (
	"log"
	"os"
)

// StdoutLogger writes logs to stdout using the standard log package.
// Safe for concurrent use across goroutines.
type StdoutLogger struct {
	logger *log.Logger
}

var _ Logger = (*StdoutLogger)(nil)

// NewStdoutLogger creates a new logger that writes to stdout
func NewStdoutLogger() *StdoutLogger {
	return &StdoutLogger{
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (s *StdoutLogger) Type() LoggerType {
	return LoggerTypeStdout
}

func (s *StdoutLogger) Printf(format string, args ...any) {
	s.logger.Printf(format, args...)
}

func (s *StdoutLogger) Println(message string) {
	s.logger.Println(message)
}

func (s *StdoutLogger) Close() error {
	return nil
}

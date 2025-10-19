package logger

import (
	"io"
	"log"
)

// WriterLogger adapts any io.Writer to the Logger interface.
// Thread safety depends on the underlying writer.
type WriterLogger struct {
	logger *log.Logger
}

var _ Logger = (*WriterLogger)(nil)

// NewWriterLogger creates a logger from any io.Writer
func NewWriterLogger(w io.Writer) *WriterLogger {
	return &WriterLogger{
		logger: log.New(w, "", log.LstdFlags),
	}
}

func (w *WriterLogger) Type() LoggerType {
	return LoggerTypeWriter
}

func (w *WriterLogger) Printf(format string, args ...any) {
	w.logger.Printf(format, args...)
}

func (w *WriterLogger) Println(message string) {
	w.logger.Println(message)
}

func (w *WriterLogger) Close() error {
	return nil
}

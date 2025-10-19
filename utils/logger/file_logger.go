package logger

import (
	"log"
	"os"
)

// FileLogger writes logs to a file using O_APPEND for atomic writes.
// Safe for concurrent use across both goroutines and processes.
// Uses Go's log.Logger which has internal mutex for thread safety,
// and O_APPEND for process-safe atomic appends.
type FileLogger struct {
	logger *log.Logger
	file   *os.File
}

var _ Logger = (*FileLogger)(nil)

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

func (f *FileLogger) Type() LoggerType {
	return LoggerTypeFile
}

func (f *FileLogger) Printf(format string, args ...any) {
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

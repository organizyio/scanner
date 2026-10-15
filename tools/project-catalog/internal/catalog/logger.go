package catalog

import (
	"fmt"
	"os"
)

// Logger handles verbose logging for the catalog CLI.
type Logger struct {
	verbose bool
}

// NewLogger creates a new logger with the specified verbosity.
func NewLogger(verbose bool) *Logger {
	return &Logger{verbose: verbose}
}

// Debugf logs a formatted debug message if verbose mode is enabled.
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

// Debug logs a debug message if verbose mode is enabled.
func (l *Logger) Debug(msg string) {
	if l.verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] %s\n", msg)
	}
}

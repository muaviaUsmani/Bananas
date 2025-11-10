package errors

import (
	"fmt"
	"runtime/debug"
)

// PanicError represents an error recovered from a panic
type PanicError struct {
	Value      interface{} // The panic value
	Stacktrace string      // Full stack trace
}

// Error implements the error interface
func (p *PanicError) Error() string {
	return fmt.Sprintf("panic recovered: %v", p.Value)
}

// RecoverPanic recovers from a panic and returns it as an error with stack trace
// Returns nil if no panic occurred
func RecoverPanic() error {
	if r := recover(); r != nil {
		return &PanicError{
			Value:      r,
			Stacktrace: string(debug.Stack()),
		}
	}
	return nil
}

// FormatPanicForLog returns a formatted string suitable for logging
func FormatPanicForLog(panicErr *PanicError) string {
	return fmt.Sprintf("PANIC: %v\n\nStack Trace:\n%s", panicErr.Value, panicErr.Stacktrace)
}

package server

import (
	"log"
	"sync/atomic"
)

// DebugMode controls whether debug logging is enabled.
// Set via -debug flag or MUSH_DEBUG=true environment variable.
// Can also be toggled at runtime via @admin/debug.
var debugMode atomic.Bool

// SetDebug enables or disables debug logging.
func SetDebug(on bool) {
	debugMode.Store(on)
	if on {
		log.Printf("[DEBUG] Debug logging enabled")
	}
}

// IsDebug returns whether debug logging is currently enabled.
func IsDebug() bool {
	return debugMode.Load()
}

// DebugLog prints a debug message if debug mode is enabled.
// Arguments are formatted like log.Printf.
func DebugLog(format string, args ...any) {
	if debugMode.Load() {
		log.Printf("[DEBUG] "+format, args...)
	}
}

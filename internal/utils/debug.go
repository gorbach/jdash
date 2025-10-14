package utils

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

var debugEnabled atomic.Bool

func init() {
	debugEnabled.Store(parseDebugEnv(os.Getenv("JENKINS_TUI_DEBUG")))
}

func parseDebugEnv(value string) bool {
	if value == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

// DebugEnabled reports whether debug logging is enabled.
func DebugEnabled() bool {
	return debugEnabled.Load()
}

// Debugf prints a debug log line when the JENKINS_TUI_DEBUG env var is enabled.
func Debugf(format string, args ...interface{}) {
	if !DebugEnabled() {
		return
	}
	fmt.Fprintf(os.Stderr, "[jenkins-gotui] "+format+"\n", args...)
}

package shared

import (
	"fmt"
	"io"
	"time"
)

// LogMessage writes a timestamped message to the log file
func LogMessage(logWriter io.Writer, message string) {
	timestamp := time.Now().UTC().Format("2006-01-02 15:04:05")
	logLine := fmt.Sprintf("%s: %s\n", timestamp, message)
	logWriter.Write([]byte(logLine))
}

package shared

import (
	"os"
	"path/filepath"
)

const (
	logDirName  = ".toy-servicerunner"
	logFileName = "toy-service.log"
)

// GetLogDir returns the path to the log directory
func GetLogDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, logDirName), nil
}

// GetLogPath returns the full path to the log file
func GetLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, logDirName, logFileName)
}

// EnsureLogDir creates the log directory if it doesn't exist
func EnsureLogDir() error {
	logDir, err := GetLogDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(logDir, 0755)
}

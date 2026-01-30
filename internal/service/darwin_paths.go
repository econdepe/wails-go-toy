package service

import (
	"fmt"
	"os"
	"path/filepath"
)

func getDarwinUserLaunchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", fmt.Sprintf("%s.plist", serviceName)), nil
}

func getDarwinUserDomain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}

func getDarwinUserServiceTarget() string {
	// launchctl service target format: "gui/<uid>/<label>"
	return fmt.Sprintf("%s/%s", getDarwinUserDomain(), serviceName)
}

func getDarwinLaunchdStdoutPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, serviceRunnerDirName, "launchd.out.log"), nil
}

func getDarwinLaunchdStderrPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, serviceRunnerDirName, "launchd.err.log"), nil
}

func getDarwinInstalledBinaryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	// Install to ~/.local/bin/go-toy - a common user-local bin directory
	return filepath.Join(home, ".local", "bin", "go-toy"), nil
}

// System-wide (LaunchDaemon) paths

func getDarwinSystemLaunchDaemonPath() string {
	return fmt.Sprintf("/Library/LaunchDaemons/%s.plist", serviceName)
}

func getDarwinSystemBinaryPath() string {
	return "/usr/local/bin/go-toy-taskrunner"
}

func getDarwinSystemServiceTarget() string {
	return fmt.Sprintf("system/%s", serviceName)
}

func getDarwinSystemLogDir() string {
	return "/var/log/go-toy"
}

func getDarwinSystemStdoutPath() string {
	return filepath.Join(getDarwinSystemLogDir(), "launchd.out.log")
}

func getDarwinSystemStderrPath() string {
	return filepath.Join(getDarwinSystemLogDir(), "launchd.err.log")
}

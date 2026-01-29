package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type linuxService struct{}

const serviceName = "gotoy-taskrunner"

func (l *linuxService) getServicePath() string {
	return fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
}

func (l *linuxService) Install() error {
	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	// Get current user (prefer SUDO_USER when running under sudo)
	currentUser := os.Getenv("SUDO_USER")
	if currentUser == "" {
		currentUser = os.Getenv("USER")
	}
	if currentUser == "" {
		currentUser = os.Getenv("LOGNAME")
	}
	if currentUser == "" {
		return fmt.Errorf("failed to determine current user")
	}

	// Get the actual user's home directory
	var homeDir string
	if currentUser != "" {
		homeDir = filepath.Join("/home", currentUser)
	} else {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
	}

	// Create systemd service file content
	serviceContent := fmt.Sprintf(`[Unit]
Description=Task Runner Service
After=network.target

[Service]
Type=simple
User=%s
Environment="HOME=%s"
ExecStart=%s run
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
`, currentUser, homeDir, execPath)

	// Write service file (requires sudo)
	serviceFile := l.getServicePath()
	cmd := exec.Command("sudo", "tee", serviceFile)
	cmd.Stdin = strings.NewReader(serviceContent)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload systemd
	if err := exec.Command("sudo", "systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable service
	if err := exec.Command("sudo", "systemctl", "enable", serviceName).Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	return nil
}

func (l *linuxService) Uninstall() error {
	// Stop service first
	exec.Command("sudo", "systemctl", "stop", serviceName).Run()

	// Disable service
	exec.Command("sudo", "systemctl", "disable", serviceName).Run()

	// Remove service file
	if err := exec.Command("sudo", "rm", l.getServicePath()).Run(); err != nil {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	// Reload systemd
	if err := exec.Command("sudo", "systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	return nil
}

func (l *linuxService) Start() error {
	if err := exec.Command("sudo", "systemctl", "start", serviceName).Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}
	return nil
}

func (l *linuxService) Stop() error {
	if err := exec.Command("sudo", "systemctl", "stop", serviceName).Run(); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}
	return nil
}

func (l *linuxService) Status() (string, error) {
	cmd := exec.Command("systemctl", "is-active", serviceName)
	output, err := cmd.Output()
	if err != nil {
		// Check if service exists
		if _, statErr := os.Stat(l.getServicePath()); os.IsNotExist(statErr) {
			return "Not installed", nil
		}
		return "Stopped", nil
	}

	status := strings.TrimSpace(string(output))
	if status == "active" {
		return "Running", nil
	}
	return "Stopped", nil
}

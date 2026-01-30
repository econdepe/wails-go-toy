package service

import (
	"fmt"
	"os/exec"
	"strings"
)

type windowsService struct{}

func (w *windowsService) Install() error {
	execPath, err := currentExecutablePath()
	if err != nil {
		return err
	}

	// Use sc.exe to create the service
	return runServiceControl("create", serviceName,
		"binPath=", fmt.Sprintf("\"%s\" run", execPath),
		"start=", "auto",
		"DisplayName=", "Task Runner Service")
}

func (w *windowsService) Uninstall() error {
	// Stop service first (ignore errors)
	_ = w.Stop()

	// Delete the service
	return runServiceControl("delete", serviceName)
}

func (w *windowsService) Start() error {
	return runServiceControl("start", serviceName)
}

func (w *windowsService) Stop() error {
	output, err := runServiceControlOutput("stop", serviceName)
	if err != nil {
		// Service might already be stopped (error code 1062)
		if !strings.Contains(output, "1062") {
			return fmt.Errorf("failed to stop service: %w: %s", err, strings.TrimSpace(output))
		}
	}
	return nil
}

func (w *windowsService) Status() (string, error) {
	output, err := runServiceControlOutput("query", serviceName)

	if err != nil {
		// Check if service doesn't exist (error code 1060)
		if strings.Contains(output, "1060") {
			return "Not installed", nil
		}
		return "Error", fmt.Errorf("failed to query service: %w: %s", err, strings.TrimSpace(output))
	}

	if strings.Contains(output, "RUNNING") {
		return "Running", nil
	} else if strings.Contains(output, "STOPPED") {
		return "Stopped", nil
	}

	return "Unknown", nil
}

// Helper functions following the same pattern as Linux and macOS

func runServiceControl(args ...string) error {
	output, err := runServiceControlOutput(args...)
	if err != nil {
		return fmt.Errorf("sc.exe %s failed: %w: %s", args[0], err, strings.TrimSpace(output))
	}
	return nil
}

func runServiceControlOutput(args ...string) (string, error) {
	cmd := exec.Command("sc.exe", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

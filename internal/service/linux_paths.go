package service

import (
	"fmt"
	"os"
	"path/filepath"
)

func getSystemServicePath() string {
	return fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
}

func getUserServicePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user", fmt.Sprintf("%s.service", serviceName)), nil
}

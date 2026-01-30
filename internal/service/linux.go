package service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type linuxService struct{}

type serviceScope int

const (
	scopeNone serviceScope = iota
	scopeUser
	scopeSystem
)

func (l *linuxService) Install() error {
	return l.installUser()
}

// InstallSystem installs a system-wide unit under /etc/systemd/system.
// This typically requires admin privileges.
func (l *linuxService) InstallSystem() error {
	return l.installSystem()
}

func (l *linuxService) Uninstall() error {
	var errs []error

	if l.userUnitExists() {
		if err := l.uninstallUser(); err != nil {
			errs = append(errs, fmt.Errorf("user service uninstall: %w", err))
		}
	}
	if l.systemUnitExists() {
		if err := l.uninstallSystem(); err != nil {
			errs = append(errs, fmt.Errorf("system service uninstall: %w", err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (l *linuxService) Start() error {
	switch l.preferredScope() {
	case scopeUser:
		return runWithOutput("systemctl", "--user", "start", serviceName)
	case scopeSystem:
		return runSystemctlSystem("start", serviceName)
	default:
		return fmt.Errorf("service not installed")
	}
}

func (l *linuxService) Stop() error {
	switch l.preferredScope() {
	case scopeUser:
		return runWithOutput("systemctl", "--user", "stop", serviceName)
	case scopeSystem:
		return runSystemctlSystem("stop", serviceName)
	default:
		return fmt.Errorf("service not installed")
	}
}

func (l *linuxService) Status() (string, error) {
	switch l.preferredScope() {
	case scopeUser:
		return l.statusUser()
	case scopeSystem:
		return l.statusSystem()
	default:
		return "Not installed", nil
	}
}

func (l *linuxService) installUser() error {
	execPath, err := currentExecutablePath()
	if err != nil {
		return err
	}

	serviceFile, err := getUserServicePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(serviceFile), 0755); err != nil {
		return fmt.Errorf("failed to create user systemd dir: %w", err)
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=Task Runner Service
After=network.target

[Service]
Type=simple
ExecStart=%s run
Restart=on-failure
RestartSec=10

[Install]
WantedBy=default.target
`, execPath)

	if err := os.WriteFile(serviceFile, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write user service file: %w", err)
	}

	// Reload user systemd
	if err := runWithOutput("systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}

	// Enable service (do not start automatically; Start is separate)
	if err := runWithOutput("systemctl", "--user", "enable", serviceName); err != nil {
		return err
	}

	return nil
}

func (l *linuxService) installSystem() error {
	execPath, err := currentExecutablePath()
	if err != nil {
		return err
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

	// Best-effort home directory (works for typical Linux /home/<user>)
	homeDir := filepath.Join("/home", currentUser)

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
	serviceFile := getSystemServicePath()
	if user_is_root() {
		if err := os.WriteFile(serviceFile, []byte(serviceContent), 0644); err != nil {
			return fmt.Errorf("failed to write system service file: %w", err)
		}
	} else {
		cmd := exec.Command("sudo", "tee", serviceFile)
		cmd.Stdin = strings.NewReader(serviceContent)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to write system service file: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}

	// Reload systemd
	if err := runSystemctlSystem("daemon-reload"); err != nil {
		return err
	}

	// Enable service
	if err := runSystemctlSystem("enable", serviceName); err != nil {
		return err
	}

	return nil
}

func (l *linuxService) uninstallUser() error {
	// Stop/disable (ignore errors)
	exec.Command("systemctl", "--user", "stop", serviceName).Run()
	exec.Command("systemctl", "--user", "disable", serviceName).Run()

	serviceFile, err := getUserServicePath()
	if err != nil {
		return err
	}
	_ = os.Remove(serviceFile)

	// Reload user systemd
	if err := runWithOutput("systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}

	return nil
}

func (l *linuxService) uninstallSystem() error {
	// Stop service first (ignore errors)
	runSystemctlSystemIgnoreErrors("stop", serviceName)
	runSystemctlSystemIgnoreErrors("disable", serviceName)

	// Remove service file
	if err := runPrivileged("rm", getSystemServicePath()); err != nil {
		return err
	}

	// Reload systemd
	if err := runSystemctlSystem("daemon-reload"); err != nil {
		return err
	}

	return nil
}

func (l *linuxService) statusUser() (string, error) {
	output, err := exec.Command("systemctl", "--user", "is-active", serviceName).CombinedOutput()
	status := strings.TrimSpace(string(output))
	if err != nil {
		if status == "" {
			return "", fmt.Errorf("failed to get user service status: %w", err)
		}
		// inactive/failed usually return non-zero; treat as stopped
		return "Stopped (user)", nil
	}
	if status == "active" {
		return "Running (user)", nil
	}
	return "Stopped (user)", nil
}

func (l *linuxService) statusSystem() (string, error) {
	output, err := exec.Command("systemctl", "is-active", serviceName).CombinedOutput()
	status := strings.TrimSpace(string(output))
	if err != nil {
		if status == "" {
			return "", fmt.Errorf("failed to get system service status: %w", err)
		}
		return "Stopped (system)", nil
	}
	if status == "active" {
		return "Running (system)", nil
	}
	return "Stopped (system)", nil
}

func (l *linuxService) userUnitExists() bool {
	p, err := getUserServicePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

func (l *linuxService) systemUnitExists() bool {
	_, err := os.Stat(getSystemServicePath())
	return err == nil
}

func (l *linuxService) preferredScope() serviceScope {
	if l.userUnitExists() {
		return scopeUser
	}
	if l.systemUnitExists() {
		return scopeSystem
	}
	return scopeNone
}

func currentExecutablePath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlinks: %w", err)
	}
	return execPath, nil
}

func runWithOutput(name string, args ...string) error {
	output, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func user_is_root() bool {
	return os.Geteuid() == 0
}

func runPrivileged(name string, args ...string) error {
	if user_is_root() {
		return runWithOutput(name, args...)
	}
	sudoArgs := append([]string{name}, args...)
	return runWithOutput("sudo", sudoArgs...)
}

func runSystemctlSystem(args ...string) error {
	if user_is_root() {
		return runWithOutput("systemctl", args...)
	}
	return runWithOutput("sudo", append([]string{"systemctl"}, args...)...)
}

func runSystemctlSystemIgnoreErrors(args ...string) {
	if user_is_root() {
		exec.Command("systemctl", args...).Run()
		return
	}
	exec.Command("sudo", append([]string{"systemctl"}, args...)...).Run()
}

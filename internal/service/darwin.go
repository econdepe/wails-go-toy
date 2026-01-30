package service

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type darwinService struct{}

func (d *darwinService) Install() error {
	srcExecPath, err := currentExecutablePath()
	if err != nil {
		return err
	}

	// Copy the binary to a stable location so launchd can reliably access it.
	// This is necessary because when running via `wails dev`, the executable
	// is in a temporary build directory that launchd cannot access properly.
	installedBinPath, err := getDarwinInstalledBinaryPath()
	if err != nil {
		return err
	}

	// Ensure the bin directory exists
	if err := os.MkdirAll(filepath.Dir(installedBinPath), 0755); err != nil {
		return fmt.Errorf("failed to create bin dir: %w", err)
	}

	// Copy the binary to the stable location
	if err := copyExecutable(srcExecPath, installedBinPath); err != nil {
		return fmt.Errorf("failed to copy executable: %w", err)
	}

	plistPath, err := getDarwinUserLaunchAgentPath()
	if err != nil {
		return err
	}

	// Ensure LaunchAgents directory exists
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents dir: %w", err)
	}

	// Ensure our log directory exists (for StandardOut/ErrPath)
	_ = os.MkdirAll(filepath.Join(mustUserHomeDir(), serviceRunnerDirName), 0755)

	stdoutPath, err := getDarwinLaunchdStdoutPath()
	if err != nil {
		return err
	}
	stderrPath, err := getDarwinLaunchdStderrPath()
	if err != nil {
		return err
	}

	// Keep semantics similar to Linux: install/register, but don't start (no RunAtLoad/KeepAlive).
	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>

	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>run</string>
	</array>

	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, serviceName, xmlEscape(installedBinPath), xmlEscape(stdoutPath), xmlEscape(stderrPath))

	// Write plist and sync to disk before launchctl tries to read it
	plistFile, err := os.OpenFile(plistPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create plist file: %w", err)
	}
	if _, err := plistFile.WriteString(plistContent); err != nil {
		plistFile.Close()
		return fmt.Errorf("failed to write plist: %w", err)
	}
	if err := plistFile.Sync(); err != nil {
		plistFile.Close()
		return fmt.Errorf("failed to sync plist: %w", err)
	}
	plistFile.Close()

	// Register with launchd for the current GUI session (idempotent-ish).
	// If it was previously loaded, boot it out first so updates apply.
	_ = d.bootoutIgnoreErrors(plistPath)
	if err := d.bootstrap(plistPath); err != nil {
		return err
	}

	// Best-effort: ensure it's enabled (Start will still be needed to run).
	_ = d.enableIgnoreErrors()

	return nil
}

// InstallSystem installs a system-wide LaunchDaemon under /Library/LaunchDaemons.
// This requires admin privileges.
func (d *darwinService) InstallSystem() error {
	srcExecPath, err := currentExecutablePath()
	if err != nil {
		return err
	}

	// Get current user info for running the service as that user
	currentUser := os.Getenv("SUDO_USER")
	if currentUser == "" {
		currentUser = os.Getenv("USER")
	}
	if currentUser == "" {
		return fmt.Errorf("failed to determine current user")
	}

	plistPath := getDarwinSystemLaunchDaemonPath()
	binPath := getDarwinSystemBinaryPath()

	// Create log directory
	logDir := getDarwinSystemLogDir()
	if err := runPrivilegedDarwin("mkdir", "-p", logDir); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Copy binary to system location
	if err := copyExecutablePrivileged(srcExecPath, binPath); err != nil {
		return fmt.Errorf("failed to copy executable: %w", err)
	}

	// Create plist content - run as the current user
	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>

	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>run</string>
	</array>

	<key>UserName</key>
	<string>%s</string>

	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, serviceName, xmlEscape(binPath), currentUser, getDarwinSystemStdoutPath(), getDarwinSystemStderrPath())

	// Write plist via sudo
	if err := writeFilePrivileged(plistPath, plistContent, "644"); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	// Unload if previously loaded (ignore errors)
	_ = runLaunchctlSystem("bootout", "system", plistPath)

	// Load the daemon
	if err := runLaunchctlSystem("bootstrap", "system", plistPath); err != nil {
		// Try legacy load as fallback
		if err2 := runLaunchctlSystem("load", "-w", plistPath); err2 != nil {
			return fmt.Errorf("launchctl bootstrap failed: %w (legacy load also failed: %v)", err, err2)
		}
	}

	// Enable the service
	_ = runLaunchctlSystem("enable", getDarwinSystemServiceTarget())

	return nil
}

func (d *darwinService) Uninstall() error {
	var errs []error

	// Uninstall user service if exists
	if d.userPlistExists() {
		if err := d.uninstallUser(); err != nil {
			errs = append(errs, fmt.Errorf("user service uninstall: %w", err))
		}
	}

	// Uninstall system service if exists
	if d.systemPlistExists() {
		if err := d.uninstallSystem(); err != nil {
			errs = append(errs, fmt.Errorf("system service uninstall: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%v", errs)
	}
	return nil
}

func (d *darwinService) uninstallUser() error {
	plistPath, err := getDarwinUserLaunchAgentPath()
	if err != nil {
		return err
	}

	// Best-effort stop/unregister; ignore errors so file removal still happens.
	_ = d.Stop()
	_ = d.bootoutIgnoreErrors(plistPath)

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist: %w", err)
	}

	// Also remove the installed binary
	if binPath, err := getDarwinInstalledBinaryPath(); err == nil {
		_ = os.Remove(binPath)
	}

	return nil
}

func (d *darwinService) uninstallSystem() error {
	plistPath := getDarwinSystemLaunchDaemonPath()

	// Stop and unload
	_ = runLaunchctlSystem("bootout", "system", plistPath)
	_ = runLaunchctlSystem("unload", "-w", plistPath)

	// Remove plist
	if err := runPrivilegedDarwin("rm", "-f", plistPath); err != nil {
		return fmt.Errorf("failed to remove plist: %w", err)
	}

	// Remove binary
	if err := runPrivilegedDarwin("rm", "-f", getDarwinSystemBinaryPath()); err != nil {
		return fmt.Errorf("failed to remove binary: %w", err)
	}

	return nil
}

func (d *darwinService) userPlistExists() bool {
	p, err := getDarwinUserLaunchAgentPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

func (d *darwinService) systemPlistExists() bool {
	_, err := os.Stat(getDarwinSystemLaunchDaemonPath())
	return err == nil
}

type darwinScope int

const (
	darwinScopeNone darwinScope = iota
	darwinScopeUser
	darwinScopeSystem
)

func (d *darwinService) preferredScope() darwinScope {
	if d.userPlistExists() {
		return darwinScopeUser
	}
	if d.systemPlistExists() {
		return darwinScopeSystem
	}
	return darwinScopeNone
}

func (d *darwinService) Start() error {
	switch d.preferredScope() {
	case darwinScopeUser:
		return d.startUser()
	case darwinScopeSystem:
		return d.startSystem()
	default:
		return fmt.Errorf("service not installed")
	}
}

func (d *darwinService) startUser() error {
	plistPath, err := getDarwinUserLaunchAgentPath()
	if err != nil {
		return err
	}

	// Ensure registered with launchd, then enable and kickstart.
	if loaded, _, err := d.isLoaded(); err != nil {
		return err
	} else if !loaded {
		if err := d.bootstrap(plistPath); err != nil {
			return err
		}
	}

	if err := d.enable(); err != nil {
		return err
	}
	if err := d.kickstart(); err != nil {
		return err
	}

	return nil
}

func (d *darwinService) startSystem() error {
	plistPath := getDarwinSystemLaunchDaemonPath()

	// Enable and start the system service
	_ = runLaunchctlSystem("enable", getDarwinSystemServiceTarget())
	if err := runLaunchctlSystem("kickstart", "-k", getDarwinSystemServiceTarget()); err != nil {
		// Try legacy start as fallback
		if err2 := runLaunchctlSystem("start", serviceName); err2 != nil {
			// Try loading the plist
			if err3 := runLaunchctlSystem("load", "-w", plistPath); err3 != nil {
				return fmt.Errorf("failed to start system service: %w", err)
			}
		}
	}
	return nil
}

func (d *darwinService) Stop() error {
	switch d.preferredScope() {
	case darwinScopeUser:
		return d.stopUser()
	case darwinScopeSystem:
		return d.stopSystem()
	default:
		return fmt.Errorf("service not installed")
	}
}

func (d *darwinService) stopUser() error {
	// Disable first to avoid immediate relaunch in common configs.
	_ = d.disableIgnoreErrors()

	// Stop job (best-effort if not running).
	_ = d.stopIgnoreErrors()
	return nil
}

func (d *darwinService) stopSystem() error {
	_ = runLaunchctlSystem("disable", getDarwinSystemServiceTarget())
	_ = runLaunchctlSystem("stop", serviceName)
	return nil
}

func (d *darwinService) Status() (string, error) {
	switch d.preferredScope() {
	case darwinScopeUser:
		return d.statusUser()
	case darwinScopeSystem:
		return d.statusSystem()
	default:
		return "Not installed", nil
	}
}

func (d *darwinService) statusUser() (string, error) {
	loaded, out, err := d.isLoaded()
	if err != nil {
		return "", err
	}
	if !loaded {
		return "Installed (not loaded)", nil
	}

	// Heuristics: launchctl print contains "state = running" for a running job.
	if strings.Contains(out, "state = running") || strings.Contains(out, "pid =") {
		return "Running (user)", nil
	}
	return "Stopped (user)", nil
}

func (d *darwinService) statusSystem() (string, error) {
	cmd := exec.Command("launchctl", "print", getDarwinSystemServiceTarget())
	out, err := cmd.CombinedOutput()
	outStr := string(out)

	if err != nil {
		lower := strings.ToLower(outStr)
		if strings.Contains(lower, "could not find service") ||
			strings.Contains(lower, "not found") {
			return "Installed (not loaded)", nil
		}
		return "", fmt.Errorf("failed to get system service status: %w: %s", err, strings.TrimSpace(outStr))
	}

	if strings.Contains(outStr, "state = running") || strings.Contains(outStr, "pid =") {
		return "Running (system)", nil
	}
	return "Stopped (system)", nil
}

func (d *darwinService) isLoaded() (bool, string, error) {
	out, err := launchctlCombinedOutput("print", getDarwinUserServiceTarget())
	if err != nil {
		// Not loaded / unknown service is not an error for our status semantics.
		lower := strings.ToLower(out)
		if strings.Contains(lower, "could not find service") ||
			strings.Contains(lower, "unknown service") ||
			strings.Contains(lower, "not found") {
			return false, out, nil
		}
		return false, out, fmt.Errorf("launchctl print failed: %w: %s", err, strings.TrimSpace(out))
	}
	return true, out, nil
}

func (d *darwinService) bootstrap(plistPath string) error {
	// Try modern bootstrap first
	out, err := launchctlCombinedOutput("bootstrap", getDarwinUserDomain(), plistPath)
	if err != nil {
		// If bootstrap fails, try legacy load command which may work better
		// in certain contexts (e.g., when called from GUI apps)
		outLegacy, errLegacy := launchctlCombinedOutput("load", "-w", plistPath)
		if errLegacy != nil {
			// Return the original bootstrap error with additional context
			return fmt.Errorf("launchctl bootstrap failed: %w: %s (legacy load also failed: %s)",
				err, strings.TrimSpace(out), strings.TrimSpace(outLegacy))
		}
		// Legacy load succeeded
		return nil
	}
	return nil
}

func (d *darwinService) bootoutIgnoreErrors(plistPath string) error {
	// Try multiple unload methods for compatibility; errors are expected if not loaded.
	_, _ = launchctlCombinedOutput("bootout", getDarwinUserServiceTarget())
	_, _ = launchctlCombinedOutput("unload", "-w", plistPath) // legacy fallback
	return nil
}

func (d *darwinService) enable() error {
	out, err := launchctlCombinedOutput("enable", getDarwinUserServiceTarget())
	if err != nil {
		return fmt.Errorf("launchctl enable failed: %w: %s", err, strings.TrimSpace(out))
	}
	return nil
}

func (d *darwinService) enableIgnoreErrors() error {
	_, _ = launchctlCombinedOutput("enable", getDarwinUserServiceTarget())
	return nil
}

func (d *darwinService) disableIgnoreErrors() error {
	_, _ = launchctlCombinedOutput("disable", getDarwinUserServiceTarget())
	return nil
}

func (d *darwinService) kickstart() error {
	out, err := launchctlCombinedOutput("kickstart", "-k", getDarwinUserServiceTarget())
	if err != nil {
		return fmt.Errorf("launchctl kickstart failed: %w: %s", err, strings.TrimSpace(out))
	}
	return nil
}

func (d *darwinService) stopIgnoreErrors() error {
	_, _ = launchctlCombinedOutput("stop", getDarwinUserServiceTarget())
	return nil
}

func launchctlCombinedOutput(args ...string) (string, error) {
	cmd := exec.Command("launchctl", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runPrivilegedDarwin(name string, args ...string) error {
	if os.Geteuid() == 0 {
		cmd := exec.Command(name, args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s failed: %w: %s", name, err, strings.TrimSpace(string(output)))
		}
		return nil
	}
	sudoArgs := append([]string{name}, args...)
	cmd := exec.Command("sudo", sudoArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sudo %s failed: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func runLaunchctlSystem(args ...string) error {
	if os.Geteuid() == 0 {
		cmd := exec.Command("launchctl", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("launchctl %s failed: %w: %s", args[0], err, strings.TrimSpace(string(output)))
		}
		return nil
	}
	sudoArgs := append([]string{"launchctl"}, args...)
	cmd := exec.Command("sudo", sudoArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sudo launchctl %s failed: %w: %s", args[0], err, strings.TrimSpace(string(output)))
	}
	return nil
}

func copyExecutablePrivileged(src, dst string) error {
	// Use cp via sudo to copy to privileged location
	if os.Geteuid() == 0 {
		if err := copyExecutable(src, dst); err != nil {
			return err
		}
		return nil
	}
	// Remove existing first
	_ = exec.Command("sudo", "rm", "-f", dst).Run()
	cmd := exec.Command("sudo", "cp", src, dst)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy: %w: %s", err, strings.TrimSpace(string(output)))
	}
	// Make executable
	cmd = exec.Command("sudo", "chmod", "755", dst)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to chmod: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func writeFilePrivileged(path, content, mode string) error {
	if os.Geteuid() == 0 {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
		return nil
	}
	// Use tee via sudo
	cmd := exec.Command("sudo", "tee", path)
	cmd.Stdin = strings.NewReader(content)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write: %w: %s", err, strings.TrimSpace(string(output)))
	}
	// Set permissions
	cmd = exec.Command("sudo", "chmod", mode, path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to chmod: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func xmlEscape(s string) string {
	// Minimal XML escaping for attribute/text nodes.
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}

func mustUserHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fall back; callers only use this for best-effort directory creation.
		return ""
	}
	return home
}

func copyExecutable(src, dst string) error {
	_ = os.Remove(dst) // Remove existing file if present

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		return fmt.Errorf("failed to copy: %w", err)
	}

	// Sync to ensure data is flushed to disk before launchctl reads it
	if err := dstFile.Sync(); err != nil {
		dstFile.Close()
		return fmt.Errorf("failed to sync: %w", err)
	}

	return dstFile.Close()
}

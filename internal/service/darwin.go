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

func (d *darwinService) Uninstall() error {
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

func (d *darwinService) Start() error {
	plistPath, err := getDarwinUserLaunchAgentPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(plistPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("service not installed")
		}
		return fmt.Errorf("failed to stat plist: %w", err)
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

func (d *darwinService) Stop() error {
	// Disable first to avoid immediate relaunch in common configs.
	_ = d.disableIgnoreErrors()

	// Stop job (best-effort if not running).
	if err := d.stopIgnoreErrors(); err != nil {
		return err
	}
	return nil
}

func (d *darwinService) Status() (string, error) {
	plistPath, err := getDarwinUserLaunchAgentPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(plistPath); err != nil {
		if os.IsNotExist(err) {
			return "Not installed", nil
		}
		return "", fmt.Errorf("failed to stat plist: %w", err)
	}

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

//go:build linux

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const systemdServiceName = "claude-monitor"

func getSystemdUserDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "systemd", "user")
}

func getSystemdServicePath() string {
	return filepath.Join(getSystemdUserDir(), systemdServiceName+".service")
}

func getExecutablePath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(execPath)
}

func copyBinaryToConfigDir() (string, error) {
	srcPath, err := getExecutablePath()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	if err := os.MkdirAll(getConfigDir(), 0700); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	dstPath := getInstalledBinaryPath()

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source binary: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dstPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create destination binary: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return "", fmt.Errorf("failed to copy binary: %w", err)
	}

	return dstPath, nil
}

func installService(_ *Config) error {
	// Stop existing service if running
	if isServiceInstalled() {
		fmt.Println("Stopping existing service...")
		exec.Command("systemctl", "--user", "stop", systemdServiceName).Run()
		exec.Command("systemctl", "--user", "disable", systemdServiceName).Run()
	}

	// Copy binary to config directory
	installedPath, err := copyBinaryToConfigDir()
	if err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}
	fmt.Printf("Binary copied to: %s\n", installedPath)

	// Create systemd user directory
	if err := os.MkdirAll(getSystemdUserDir(), 0755); err != nil {
		return fmt.Errorf("failed to create systemd user directory: %w", err)
	}

	// Create systemd user service unit
	serviceContent := fmt.Sprintf(`[Unit]
Description=Claude Code Usage Monitor
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s run
Restart=on-failure
RestartSec=10
StandardOutput=append:%s
StandardError=append:%s

[Install]
WantedBy=default.target
`, installedPath, getLogPath(), getLogPath())

	if err := os.WriteFile(getSystemdServicePath(), []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write systemd service: %w", err)
	}

	// Reload systemd and enable/start
	fmt.Println("Starting service...")
	exec.Command("systemctl", "--user", "daemon-reload").Run()

	cmd := exec.Command("systemctl", "--user", "enable", "--now", systemdServiceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable service: %s - %w", string(output), err)
	}

	return nil
}

func uninstallService() error {
	if _, err := os.Stat(getSystemdServicePath()); os.IsNotExist(err) {
		return fmt.Errorf("service is not installed")
	}

	exec.Command("systemctl", "--user", "stop", systemdServiceName).Run()
	exec.Command("systemctl", "--user", "disable", systemdServiceName).Run()

	if err := os.Remove(getSystemdServicePath()); err != nil {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	exec.Command("systemctl", "--user", "daemon-reload").Run()

	return nil
}

func isServiceRunning() bool {
	cmd := exec.Command("systemctl", "--user", "is-active", systemdServiceName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "active"
}

func getServiceStatus() string {
	cmd := exec.Command("systemctl", "--user", "is-active", systemdServiceName)
	output, err := cmd.Output()
	if err != nil {
		return "Not installed or not running"
	}

	state := strings.TrimSpace(string(output))
	switch state {
	case "active":
		return "Running"
	case "inactive":
		return "Installed but not running"
	case "failed":
		return "Failed"
	default:
		return fmt.Sprintf("Installed (%s)", state)
	}
}

func isServiceInstalled() bool {
	_, err := os.Stat(getSystemdServicePath())
	return err == nil
}

// startDetachedInstaller runs "<path> install" outside of this service process.
// systemctl stop kills every process in the service cgroup, so the installer runs as a transient unit.
func startDetachedInstaller(path string) error {
	logPath := getLogPath()
	cmd := exec.Command("systemd-run", "--user", "--collect", "--quiet",
		"-p", "StandardOutput=append:"+logPath,
		"-p", "StandardError=append:"+logPath,
		path, "install")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemd-run failed: %s - %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

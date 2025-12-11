//go:build darwin

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const launchAgentLabel = "com.claude.monitor"

func getLaunchAgentPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "Library", "LaunchAgents", launchAgentLabel+".plist")
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

	// Ensure config directory exists
	if err := os.MkdirAll(getConfigDir(), 0700); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	dstPath := getInstalledBinaryPath()

	// Open source file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source binary: %w", err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.OpenFile(dstPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create destination binary: %w", err)
	}
	defer dstFile.Close()

	// Copy content
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return "", fmt.Errorf("failed to copy binary: %w", err)
	}

	return dstPath, nil
}

func installService(_ *Config) error {
	plistPath := getLaunchAgentPath()

	// Stop existing service if running
	if isServiceInstalled() {
		fmt.Println("Stopping existing service...")
		exec.Command("launchctl", "unload", plistPath).Run()
	}

	// Copy binary to config directory
	installedPath, err := copyBinaryToConfigDir()
	if err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}
	fmt.Printf("Binary copied to: %s\n", installedPath)

	// Create LaunchAgents directory if not exists
	launchAgentsDir := filepath.Dir(plistPath)
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	// Create plist content with installed binary path
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
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
</dict>
</plist>`, launchAgentLabel, installedPath, getLogPath(), getLogPath())

	// Write plist file
	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	// Load and start the service
	fmt.Println("Starting service...")
	cmd := exec.Command("launchctl", "load", plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to load LaunchAgent: %s - %w", string(output), err)
	}

	return nil
}

func uninstallService() error {
	plistPath := getLaunchAgentPath()

	// Check if plist exists
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return fmt.Errorf("service is not installed")
	}

	// Unload
	cmd := exec.Command("launchctl", "unload", plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Continue even if unload fails
		fmt.Printf("Warning: unload returned: %s\n", string(output))
	}

	// Remove plist file
	if err := os.Remove(plistPath); err != nil {
		return fmt.Errorf("failed to remove plist: %w", err)
	}

	return nil
}

func isServiceRunning() bool {
	cmd := exec.Command("launchctl", "list", launchAgentLabel)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(output) > 0
}

func getServiceStatus() string {
	cmd := exec.Command("launchctl", "list", launchAgentLabel)
	output, err := cmd.Output()
	if err != nil {
		return "Not installed or not running"
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		parts := strings.Fields(lines[0])
		if len(parts) >= 2 {
			pid := parts[0]
			if pid != "-" {
				return fmt.Sprintf("Running (PID: %s)", pid)
			}
		}
	}
	return "Installed but not running"
}

func isServiceInstalled() bool {
	plistPath := getLaunchAgentPath()
	_, err := os.Stat(plistPath)
	return err == nil
}

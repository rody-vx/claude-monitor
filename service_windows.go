//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const taskName = "ClaudeMonitor"

func getExecutablePath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(execPath)
}

func getInstalledBinaryPathWindows() string {
	return getInstalledBinaryPath() + ".exe"
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

	dstPath := getInstalledBinaryPathWindows()

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
	// Stop existing service if running
	if isServiceInstalled() {
		fmt.Println("Stopping existing service...")
		exec.Command("schtasks", "/End", "/TN", taskName).Run()
		exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").Run()
	}

	// Copy binary to config directory
	installedPath, err := copyBinaryToConfigDir()
	if err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}
	fmt.Printf("Binary copied to: %s\n", installedPath)

	// Create scheduled task that runs at logon
	cmd := exec.Command("schtasks", "/Create",
		"/TN", taskName,
		"/TR", fmt.Sprintf(`"%s" run`, installedPath),
		"/SC", "ONLOGON",
		"/RL", "LIMITED",
		"/F",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create scheduled task: %s - %w", string(output), err)
	}

	// Start the service
	fmt.Println("Starting service...")
	startCmd := exec.Command("schtasks", "/Run", "/TN", taskName)
	startCmd.Run()

	return nil
}

func uninstallService() error {
	// Stop the task first
	exec.Command("schtasks", "/End", "/TN", taskName).Run()

	// Delete the task
	cmd := exec.Command("schtasks", "/Delete", "/TN", taskName, "/F")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete scheduled task: %s - %w", string(output), err)
	}

	return nil
}

func isServiceRunning() bool {
	// Check if the process is running
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq claude-monitor.exe", "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "claude-monitor.exe")
}

func getServiceStatus() string {
	// Check task status
	cmd := exec.Command("schtasks", "/Query", "/TN", taskName, "/FO", "LIST")
	output, err := cmd.Output()
	if err != nil {
		return "Not installed"
	}

	outputStr := string(output)
	if strings.Contains(outputStr, "Running") {
		return "Running"
	} else if strings.Contains(outputStr, "Ready") {
		if isServiceRunning() {
			return "Running"
		}
		return "Installed (ready to run at next logon)"
	}

	return "Installed"
}

func isServiceInstalled() bool {
	cmd := exec.Command("schtasks", "/Query", "/TN", taskName)
	err := cmd.Run()
	return err == nil
}

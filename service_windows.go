//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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

func getBatchFilePath() string {
	return filepath.Join(getConfigDir(), "claude-monitor.bat")
}

func getVbsFilePath() string {
	return filepath.Join(getConfigDir(), "claude-monitor.vbs")
}

func createLauncherFiles(exePath string) error {
	// Create batch file that runs the exe with "run" argument
	batContent := fmt.Sprintf("@echo off\r\n\"%s\" run\r\n", exePath)
	if err := os.WriteFile(getBatchFilePath(), []byte(batContent), 0755); err != nil {
		return fmt.Errorf("failed to create batch file: %w", err)
	}

	// Create VBScript that launches the batch file with hidden window
	vbsContent := fmt.Sprintf("CreateObject(\"Wscript.Shell\").Run \"\"\"%s\"\"\", 0, False\r\n", getBatchFilePath())
	if err := os.WriteFile(getVbsFilePath(), []byte(vbsContent), 0755); err != nil {
		return fmt.Errorf("failed to create vbs file: %w", err)
	}

	return nil
}

func installService(_ *Config) error {
	// Stop existing service and kill all related processes
	if isServiceInstalled() {
		fmt.Println("Stopping existing service...")
		exec.Command("schtasks", "/End", "/TN", taskName).Run()
		exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").Run()
	}
	// Kill the monitor process and any wscript/cmd hosting it
	exec.Command("taskkill", "/IM", "claude-monitor.exe", "/F").Run()
	// Kill wscript running our vbs launcher
	exec.Command("cmd", "/C", fmt.Sprintf(
		`wmic process where "name='wscript.exe' and commandline like '%%%s%%'" call terminate`,
		"claude-monitor.vbs",
	)).Run()
	// Wait for file to be released
	installedBin := getInstalledBinaryPathWindows()
	for i := 0; i < 20; i++ {
		f, err := os.OpenFile(installedBin, os.O_RDWR, 0755)
		if err == nil {
			f.Close()
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Copy binary to config directory
	installedPath, err := copyBinaryToConfigDir()
	if err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}
	fmt.Printf("Binary copied to: %s\n", installedPath)

	// Create launcher files (bat + vbs for hidden window)
	if err := createLauncherFiles(installedPath); err != nil {
		return fmt.Errorf("failed to create launcher files: %w", err)
	}
	fmt.Printf("Launcher created: %s\n", getVbsFilePath())

	// Create scheduled task that runs the VBScript (hidden window)
	cmd := exec.Command("schtasks", "/Create",
		"/TN", taskName,
		"/TR", fmt.Sprintf(`wscript.exe "%s"`, getVbsFilePath()),
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

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func handleInstall() {
	args := os.Args[2:]

	// Get or create config (loads existing, or prompts user)
	config, err := getOrCreateConfig(args)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Installing Claude Monitor service...")
	fmt.Printf("  Email: %s\n", config.Email)
	fmt.Printf("  Server: %s\n", config.ServerURL)
	fmt.Printf("  Interval: %d seconds (%d minutes)\n", config.IntervalSeconds, config.IntervalSeconds/60)

	// Install service
	if err := installService(config); err != nil {
		fmt.Printf("Error installing service: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nService installed successfully!")
	fmt.Println("The monitor will start automatically on login.")
	fmt.Printf("Log file: %s\n", getLogPath())
}

func handleUninstall() {
	fmt.Println("Uninstalling Claude Monitor service...")

	if err := uninstallService(); err != nil {
		fmt.Printf("Error uninstalling service: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Service uninstalled successfully!")
}

func handleStatus() {
	fmt.Println("Claude Monitor Status")
	fmt.Println("=====================")

	// Check if installed
	if !isServiceInstalled() {
		fmt.Println("Status: Not installed")
		fmt.Println("\nRun 'claude-monitor install --email your@email.com' to install")
		return
	}

	// Show service status
	fmt.Printf("Service: %s\n", getServiceStatus())

	// Load and show config
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Config: Error loading (%v)\n", err)
	} else {
		fmt.Printf("\nConfiguration:\n")
		fmt.Printf("  Email: %s\n", config.Email)
		fmt.Printf("  Server: %s\n", config.ServerURL)
		fmt.Printf("  Interval: %d seconds (%d minutes)\n", config.IntervalSeconds, config.IntervalSeconds/60)
	}

	// Show log file path
	fmt.Printf("\nLog file: %s\n", getLogPath())

	// Show last few lines of log if exists
	logPath := getLogPath()
	if info, err := os.Stat(logPath); err == nil {
		fmt.Printf("Log size: %d bytes\n", info.Size())
		fmt.Printf("Last modified: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))
	}
}

func handleRun() {
	args := os.Args[2:]

	// Get or create config (loads existing, or prompts user)
	config, err := getOrCreateConfig(args)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Setup logging
	// Write to the log file directly (rotated daily) and to stdout.
	// The service definitions no longer redirect stdout to the log file, so lines are not duplicated.
	var logger *log.Logger
	logWriter, err := openDailyLog(getLogPath())
	if err != nil {
		fmt.Printf("Warning: Could not open log file: %v\n", err)
		logger = log.New(os.Stdout, "", log.LstdFlags)
	} else {
		defer logWriter.Close()
		// Log file first: MultiWriter stops at the first failing writer
		multiWriter := io.MultiWriter(logWriter, os.Stdout)
		logger = log.New(multiWriter, "", log.LstdFlags)
	}

	logger.Printf("Claude Monitor started")
	logger.Printf("  Email: %s", config.Email)
	logger.Printf("  Server: %s", config.ServerURL)
	logger.Printf("  Interval: %d seconds", config.IntervalSeconds)
	logger.Printf("  Projects dir: %s", getClaudeProjectsDir())

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initial upload
	logger.Println("Performing initial upload...")
	result, err := uploadUsageData(config)
	if err != nil {
		logger.Printf("Initial upload error: %v", err)
	} else {
		logger.Printf("Initial upload: %s", result.Message)
	}

	// Auto update only for the service binary, not for a manual run of a downloaded binary
	var updateTick <-chan time.Time
	if !config.DisableAutoUpdate && isRunningInstalledBinary() {
		if _, ok := parseVersion(version); ok {
			cleanupUpdateBinary()
			tryAutoUpdate(logger)

			updateTicker := time.NewTicker(updateCheckInterval)
			defer updateTicker.Stop()
			updateTick = updateTicker.C
		}
	}

	// Start periodic upload loop
	ticker := time.NewTicker(time.Duration(config.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	uploadCount := 1

	for {
		select {
		case <-ticker.C:
			uploadCount++
			logger.Printf("Upload #%d starting...", uploadCount)
			result, err := uploadUsageData(config)
			if err != nil {
				logger.Printf("Upload #%d error: %v", uploadCount, err)
			} else {
				logger.Printf("Upload #%d: %s", uploadCount, result.Message)
			}

		case <-updateTick:
			tryAutoUpdate(logger)

		case sig := <-sigChan:
			logger.Printf("Received signal: %v", sig)
			logger.Println("Performing final upload...")
			result, err := uploadUsageData(config)
			if err != nil {
				logger.Printf("Final upload error: %v", err)
			} else {
				logger.Printf("Final upload: %s", result.Message)
			}
			logger.Printf("Claude Monitor stopped (total uploads: %d)", uploadCount)
			return
		}
	}
}

// handleUpdate downloads the latest release and installs it
func handleUpdate() {
	if _, ok := parseVersion(version); !ok {
		fmt.Printf("Update is not supported for %s build\n", version)
		os.Exit(1)
	}

	fmt.Printf("Current version: %s\n", version)
	tag, newer, err := checkForUpdate()
	if err != nil {
		fmt.Printf("Error checking for update: %v\n", err)
		os.Exit(1)
	}
	if !newer {
		fmt.Printf("Already up to date (latest: %s)\n", tag)
		return
	}

	fmt.Printf("Downloading %s...\n", tag)
	updatePath, err := downloadUpdate(tag)
	if err != nil {
		fmt.Printf("Error downloading update: %v\n", err)
		os.Exit(1)
	}

	// Install with the new binary, which also replaces the installed binary and restarts the service
	cmd := exec.Command(updatePath, "install")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error installing update: %v\n", err)
		os.Exit(1)
	}
}

// handleTest collects usage data and saves to file for comparison (no upload)
func handleTest() {
	fmt.Println("Test mode: Collecting usage data without uploading...")
	fmt.Printf("Projects dir: %s\n", getClaudeProjectsDir())

	usageData, err := collectUsageData()
	if err != nil {
		fmt.Printf("Error collecting data: %v\n", err)
		os.Exit(1)
	}

	// Save to file
	outputFile := filepath.Join(os.TempDir(), "claude-usage-go.json")
	jsonData, err := json.MarshalIndent(usageData, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nOutput saved to: %s\n", outputFile)
	fmt.Printf("Total days: %d\n", len(usageData.Daily))

	// Print summary
	var totalTokens int64
	var totalRequests int
	for _, day := range usageData.Daily {
		totalTokens += day.TotalTokens
		totalRequests += day.RequestCount
	}
	fmt.Printf("Total tokens: %d\n", totalTokens)
	fmt.Printf("Total requests: %d\n", totalRequests)
}

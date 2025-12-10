package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Email          string `json:"email"`
	ServerURL      string `json:"serverUrl"`
	IntervalSeconds int    `json:"intervalSeconds"`
}

func getConfigDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".claude-monitor")
}

func getConfigPath() string {
	return filepath.Join(getConfigDir(), "config.json")
}

func getLogPath() string {
	return filepath.Join(getConfigDir(), "monitor.log")
}

func getInstalledBinaryPath() string {
	return filepath.Join(getConfigDir(), "claude-monitor")
}

func getClaudeProjectsDir() string {
	if dir := os.Getenv("CLAUDE_PROJECTS_DIR"); dir != "" {
		return dir
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".claude", "projects")
}

func loadConfig() (*Config, error) {
	configPath := getConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Set defaults
	if config.ServerURL == "" {
		config.ServerURL = "http://10.12.200.99:3498"
	}
	if config.IntervalSeconds == 0 {
		config.IntervalSeconds = 600
	}

	return &config, nil
}

func saveConfig(config *Config) error {
	configDir := getConfigDir()
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(getConfigPath(), data, 0600)
}

func parseInstallArgs(args []string) *Config {
	config := &Config{
		ServerURL:       "http://10.12.200.99:3498",
		IntervalSeconds: 600,
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--email":
			if i+1 < len(args) {
				config.Email = args[i+1]
				i++
			}
		case "--server":
			if i+1 < len(args) {
				config.ServerURL = args[i+1]
				i++
			}
		case "--interval":
			if i+1 < len(args) {
				var interval int
				fmt.Sscanf(args[i+1], "%d", &interval)
				if interval > 0 {
					config.IntervalSeconds = interval
				}
				i++
			}
		}
	}

	return config
}

// promptInput prompts user for input with optional default value
func promptInput(prompt string, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)

	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" && defaultValue != "" {
		return defaultValue
	}
	return input
}

// promptConfig interactively prompts user for configuration
func promptConfig() *Config {
	fmt.Println("Claude Monitor Configuration")
	fmt.Println("============================")
	fmt.Println()

	config := &Config{}

	// Email (required)
	for config.Email == "" {
		config.Email = promptInput("Email (required)", "")
		if config.Email == "" {
			fmt.Println("Error: Email is required")
		}
	}

	// Server URL (optional, has default)
	config.ServerURL = promptInput("Server URL", "http://10.12.200.99:3498")

	// Interval (optional, has default)
	intervalStr := promptInput("Upload interval in seconds", "600")
	var interval int
	fmt.Sscanf(intervalStr, "%d", &interval)
	if interval > 0 {
		config.IntervalSeconds = interval
	} else {
		config.IntervalSeconds = 600
	}

	fmt.Println()
	return config
}

// getOrCreateConfig loads existing config or prompts user to create one
func getOrCreateConfig(args []string) (*Config, error) {
	// Try to load existing config first
	existingConfig, err := loadConfig()
	if err == nil {
		return existingConfig, nil
	}

	// No config exists, check CLI args
	config := parseInstallArgs(args)

	// If email not provided via args, prompt interactively
	if config.Email == "" {
		config = promptConfig()
	}

	// Save the new config
	if err := saveConfig(config); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Configuration saved to %s\n", getConfigPath())
	fmt.Println()

	return config, nil
}

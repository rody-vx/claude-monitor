package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "install":
		handleInstall()
	case "uninstall":
		handleUninstall()
	case "status":
		handleStatus()
	case "run":
		handleRun()
	case "test":
		handleTest()
	case "version":
		fmt.Println("claude-monitor v1.0.0")
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Claude Code Usage Monitor

Usage:
  claude-monitor <command> [options]

Commands:
  install     Install as background service (auto-start on login)
  uninstall   Remove background service
  status      Show service status and last upload info
  run         Run in foreground (manual mode)
  version     Show version
  help        Show this help

Install Options:
  --email <email>       User email (required for first install)
  --server <url>        Server URL (default: http://10.12.200.99:3498)
  --interval <seconds>  Upload interval in seconds (default: 600)

Examples:
  claude-monitor install --email your@email.com
  claude-monitor install --email your@email.com --interval 300
  claude-monitor status
  claude-monitor uninstall
  claude-monitor run`)
}

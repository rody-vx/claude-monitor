package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// runLockFile is kept open for the life of the process. The OS releases the lock when the process exits.
var runLockFile *os.File

func getRunLockPath() string {
	return filepath.Join(getConfigDir(), "run.lock")
}

// acquireRunLock allows only one "run" process at a time, retrying until the timeout
func acquireRunLock(timeout time.Duration) error {
	if err := os.MkdirAll(getConfigDir(), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(getRunLockPath(), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(timeout)
	for attempt := 0; ; attempt++ {
		if err := lockFile(file); err == nil {
			runLockFile = file
			return nil
		}
		if time.Now().After(deadline) {
			file.Close()
			return fmt.Errorf("claude-monitor is already running (service or another run)")
		}
		if attempt == 0 {
			fmt.Println("Waiting for the running claude-monitor to exit...")
		}
		time.Sleep(time.Second)
	}
}

//go:build darwin || linux

package main

import (
	"os"
	"syscall"
)

// lockFile takes an exclusive lock on the file without blocking
func lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

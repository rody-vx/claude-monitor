//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx = kernel32.NewProc("LockFileEx")
)

const (
	lockfileFailImmediately = 0x00000001
	lockfileExclusiveLock   = 0x00000002
)

// lockFile takes an exclusive lock on the file without blocking
func lockFile(f *os.File) error {
	var overlapped syscall.Overlapped
	r, _, err := procLockFileEx.Call(
		f.Fd(),
		lockfileExclusiveLock|lockfileFailImmediately,
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if r == 0 {
		return err
	}
	return nil
}

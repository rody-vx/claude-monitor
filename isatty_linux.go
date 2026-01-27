//go:build linux

package main

import (
	"syscall"
	"unsafe"
)

func isatty(fd uintptr) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, 0x5401, uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	return err == 0
}

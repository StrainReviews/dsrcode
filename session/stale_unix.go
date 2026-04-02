//go:build !windows

package session

import (
	"os"
	"syscall"
)

// IsPidAlive checks if a process with the given PID is running on Unix.
// Uses signal 0 which checks existence without sending a real signal.
func IsPidAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

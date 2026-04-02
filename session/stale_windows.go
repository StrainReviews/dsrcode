//go:build windows

package session

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// IsPidAlive checks if a process with the given PID is running on Windows.
// Uses the tasklist command because os.FindProcess on Windows always succeeds
// even for non-existent PIDs (it only opens a handle, does not verify liveness).
func IsPidAlive(pid int) bool {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), strconv.Itoa(pid))
}

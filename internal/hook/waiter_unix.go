//go:build !windows

package hook

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// stopWaiter signals the waiter. A stale pid may now belong to another process,
// so only one whose command line is ours is signalled.
func stopWaiter(pid int, _ string) {
	out, _ := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if strings.Contains(string(out), "claude-notify") {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
}

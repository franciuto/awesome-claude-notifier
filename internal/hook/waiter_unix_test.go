//go:build !windows

package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
)

func TestCancelIgnoresForeignPid(t *testing.T) {
	h, _, _ := newHandler(t, nil)
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })

	pidFile := filepath.Join(h.WaitersDir, "t.pid")
	os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644)
	h.CancelReplyWait("t")

	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		t.Errorf("foreign process was signalled: %v", err)
	}
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Errorf("pid file left behind: %v", err)
	}
}

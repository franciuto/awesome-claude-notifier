package hook

import (
	"os"
	"time"

	"github.com/franciuto/awesome-claude-notifier/internal/winproc"
)

// stopWaiter ends the waiter. A stale pid may now belong to another process,
// which started after the pid file was written.
func stopWaiter(pid int, pidFile string) {
	st, err := os.Stat(pidFile)
	if err != nil {
		return
	}
	if started, ok := winproc.StartedAt(uint32(pid)); ok && !started.After(st.ModTime().Add(time.Second)) {
		winproc.Terminate(uint32(pid))
	}
}

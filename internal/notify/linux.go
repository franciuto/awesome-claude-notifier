//go:build !darwin && !windows

package notify

import (
	"os/exec"
	"time"
)

type linux struct{}

// New returns the notifier for the current OS.
func New() Notifier { return linux{} }

func (linux) Send(r Request) bool {
	return exec.Command("notify-send", notifySendArgs(r)...).Run() == nil
}

// Ask: notify-send has no buttons.
func (linux) Ask(Request, string, time.Duration) *Answer { return nil }

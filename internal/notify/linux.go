//go:build !darwin && !windows

package notify

import (
	"os/exec"
	"path/filepath"
	"time"
)

type linux struct{}

// New returns the notifier for the current OS.
func New() Notifier { return linux{} }

// iconPath is the plugin's app icon, shown by the notification daemon.
func iconPath() string {
	return filepath.Join(PluginRoot(), "assets", "icon", "app-icon.png")
}

func (linux) Send(r Request) bool {
	args := append([]string{"--icon=" + iconPath()}, notifySendArgs(r)...)
	return exec.Command("notify-send", args...).Run() == nil
}

// Ask: notify-send has no buttons.
func (linux) Ask(Request, string, time.Duration) *Answer { return nil }

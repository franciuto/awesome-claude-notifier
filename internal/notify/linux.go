//go:build !darwin && !windows

package notify

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
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

// Ask shows Allow / Deny buttons for "permission" and returns the one clicked.
// nil leaves the decision to the terminal dialog: no buttons for "reply"
// (notify-send has no text field), a daemon without actions, or no click in time.
func (linux) Ask(r Request, actions string, wait time.Duration) *Answer {
	if actions != "permission" || !daemonHasActions() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	// Options go before the title and body, the last two arguments; the later
	// --expire-time wins.
	base := notifySendArgs(r)
	args := slices.Insert(base, len(base)-2, "--icon="+iconPath(), "-A", "allow=Allow", "-A", "deny=Deny",
		"--expire-time="+strconv.Itoa(int(wait.Milliseconds())))
	out, err := exec.CommandContext(ctx, "notify-send", args...).Output()
	if err != nil {
		return nil
	}
	switch action := strings.TrimSpace(string(out)); action {
	case "allow", "deny":
		return &Answer{Action: action}
	}
	return nil
}

// daemonHasActions reports whether the notification daemon advertises the
// "actions" capability. Without it notify-send would wait for a click that
// can never come, and delay the terminal dialog.
func daemonHasActions() bool {
	for _, cmd := range [][]string{
		{"busctl", "--user", "call", "org.freedesktop.Notifications", "/org/freedesktop/Notifications", "org.freedesktop.Notifications", "GetCapabilities"},
		{"gdbus", "call", "--session", "--dest", "org.freedesktop.Notifications", "--object-path", "/org/freedesktop/Notifications", "--method", "org.freedesktop.Notifications.GetCapabilities"},
	} {
		if out, err := exec.Command(cmd[0], cmd[1:]...).Output(); err == nil {
			return bytes.Contains(out, []byte("actions"))
		}
	}
	return false
}

//go:build !darwin && !windows

package notify

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
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

// Ask shows Allow / Deny buttons ("permission") or the daemon's inline reply
// field ("reply") and waits for the answer. nil means the interactive
// notification is unavailable (the daemon lacks the capability, notify-send
// failed to start), so the caller falls back to the terminal. A reply that
// ends unanswered (dismissed, timed out, hook cancelled) is a "timeout".
func (linux) Ask(r Request, actions string, wait time.Duration) *Answer {
	var capability string
	var buttons []string
	switch actions {
	case "permission":
		capability, buttons = "actions", []string{"-A", "allow=Allow", "-A", "deny=Deny"}
	case "reply":
		// "inline-reply" is the action key the daemon turns into a text field.
		capability, buttons = "inline-reply", []string{"-A", "inline-reply=Reply"}
	default:
		return nil
	}
	if !daemonHas(capability) {
		return nil
	}

	// CancelReplyWait ends a pending Reply with SIGTERM: stop the children too.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()

	var replies <-chan reply
	if actions == "reply" {
		var ok bool
		if replies, ok = watchReplies(ctx); !ok {
			return nil
		}
	}

	// Options go before the title and body, the last two arguments; the later
	// --expire-time wins.
	base := notifySendArgs(r)
	args := slices.Insert(base, len(base)-2, append(buttons, "-p", "--icon="+iconPath(),
		"--expire-time="+strconv.Itoa(int(wait.Milliseconds())))...)
	cmd := exec.CommandContext(ctx, "notify-send", args...)
	out, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return nil
	}
	lines := bufio.NewScanner(out)
	id := uint32(0)
	if lines.Scan() {
		n, _ := strconv.ParseUint(strings.TrimSpace(lines.Text()), 10, 32)
		id = uint32(n)
	}
	action := make(chan string, 1)
	go func() {
		last := ""
		for lines.Scan() {
			last = strings.TrimSpace(lines.Text())
		}
		cmd.Wait()
		action <- last
	}()
	defer closeNotification(id)

	if actions == "permission" {
		if a := <-action; a == "allow" || a == "deny" {
			return &Answer{Action: a}
		}
		return nil
	}
	// A reply or a close ends the wait. The reply signal comes before the
	// close, but the two reach us on different pipes: give it a moment to land.
	grace := make(<-chan time.Time)
	for {
		select {
		case rp, ok := <-replies:
			if !ok {
				return &Answer{Action: "timeout"}
			}
			if rp.id == id {
				return &Answer{Action: "reply", Text: rp.text}
			}
		case <-action:
			action, grace = nil, time.After(300*time.Millisecond)
		case <-grace:
			return &Answer{Action: "timeout"}
		}
	}
}

type reply struct {
	id   uint32
	text string
}

// watchReplies streams the NotificationReplied signals until ctx ends. busctl
// reports each signal as one JSON line, which keeps the reply text intact.
// ok is false when busctl is missing or fails to start.
func watchReplies(ctx context.Context) (<-chan reply, bool) {
	cmd := exec.CommandContext(ctx, "busctl", "--user", "monitor", "--json=short",
		"--match", "type='signal',interface='org.freedesktop.Notifications',member='NotificationReplied'")
	out, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return nil, false
	}
	ch := make(chan reply, 8)
	go func() {
		defer close(ch)
		defer cmd.Wait()
		lines := bufio.NewScanner(out)
		for lines.Scan() {
			var msg struct {
				Member  string
				Payload struct{ Data []json.RawMessage }
			}
			if json.Unmarshal(lines.Bytes(), &msg) != nil || msg.Member != "NotificationReplied" || len(msg.Payload.Data) != 2 {
				continue
			}
			var rp reply
			if json.Unmarshal(msg.Payload.Data[0], &rp.id) == nil && json.Unmarshal(msg.Payload.Data[1], &rp.text) == nil {
				select {
				case ch <- rp:
				default:
				}
			}
		}
	}()
	return ch, true
}

// closeNotification withdraws a notification that is still on screen.
func closeNotification(id uint32) {
	if id != 0 {
		_ = busctlCall("CloseNotification", "u", strconv.FormatUint(uint64(id), 10))
	}
}

// daemonHas reports whether the notification daemon advertises a capability.
// Without it notify-send would wait for a click that can never come, and
// delay the terminal dialog.
func daemonHas(capability string) bool {
	out, err := busctlOutput("GetCapabilities")
	return err == nil && bytes.Contains(out, []byte(`"`+capability+`"`))
}

// busctlOutput calls a method of the notification daemon with busctl. gdbus
// quotes strings with ' instead of ", so GetCapabilities falls back to it.
func busctlOutput(method string, args ...string) ([]byte, error) {
	cmd := append([]string{"--user", "call", "org.freedesktop.Notifications", "/org/freedesktop/Notifications",
		"org.freedesktop.Notifications", method}, args...)
	out, err := exec.Command("busctl", cmd...).Output()
	if err == nil || method != "GetCapabilities" {
		return out, err
	}
	out, err = exec.Command("gdbus", "call", "--session", "--dest", "org.freedesktop.Notifications",
		"--object-path", "/org/freedesktop/Notifications", "--method", "org.freedesktop.Notifications.GetCapabilities").Output()
	return bytes.ReplaceAll(out, []byte("'"), []byte(`"`)), err
}

func busctlCall(method string, args ...string) error {
	_, err := busctlOutput(method, args...)
	return err
}

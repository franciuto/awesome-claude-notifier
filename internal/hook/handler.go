package hook

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/franciuto/awesome-claude-notifier/internal/notify"
)

// PermissionWait is how long a permission notification waits before the
// terminal dialog takes over.
const PermissionWait = 20 * time.Second

// NewHandler returns a Handler wired to the real OS.
func NewHandler() *Handler {
	return &Handler{
		Notifier:             notify.New(),
		Getenv:               os.Getenv,
		UserAtTerminal:       userAtTerminal,
		BringTerminalForward: bringTerminalForward,
		WaitersDir:           filepath.Join(notify.CacheDir(), "waiters"),
		DesktopSessionsDirs:  desktopSessionsDirs(),
	}
}

// InDesktopApp reports whether the Claude desktop app runs this session: it has
// its own UI for permissions and replies.
func (h *Handler) InDesktopApp() bool {
	return h.Getenv("CLAUDE_CODE_ENTRYPOINT") == "claude-desktop"
}

func (h *Handler) title(data map[string]any, group string) string {
	return h.SessionName(data) + " · " + group
}

// Process sends the plain notification for an event.
func (h *Handler) Process(event string, data map[string]any) bool {
	group, summary := Describe(event, data)
	return h.Notifier.Send(notify.Request{
		Title: h.title(data, group), Body: summary, Sound: SoundSlug(group), Urgent: group == Error,
	})
}

// Permission asks Allow / Deny from a notification. It returns the hook output
// to print, or nil to leave the decision to the terminal dialog.
func (h *Handler) Permission(data map[string]any) map[string]any {
	if h.InDesktopApp() || h.UserAtTerminal() {
		return nil
	}

	tool := str(data, "tool_name", "tool")
	input, _ := data["tool_input"].(map[string]any)
	detail := ""
	for _, key := range []string{"command", "file_path", "url", "description"} {
		if detail = str(input, key, ""); detail != "" {
			break
		}
	}
	body := "Asking to use " + tool
	if detail != "" {
		body = tool + ": " + detail
	}
	answer := h.Notifier.Ask(notify.Request{
		Title: h.title(data, NeedsYou),
		Body:  Summarize(body, SummaryMaxChars),
		Sound: SoundSlug(NeedsYou),
	}, "permission", PermissionWait)

	if answer == nil {
		return nil
	}
	switch answer.Action {
	case "open":
		h.BringTerminalForward()
	case "allow", "deny":
		decision := map[string]any{"behavior": answer.Action}
		if answer.Action == "deny" {
			decision["message"] = "The user denied permission from the notification."
		}
		return map[string]any{"hookSpecificOutput": map[string]any{
			"hookEventName": "PermissionRequest", "decision": decision,
		}}
	}
	return nil
}

// NotifyAndWaitReply shows the notification with a Reply field and returns the
// text the user sent, "" when there is none.
func (h *Handler) NotifyAndWaitReply(event string, data map[string]any, wait time.Duration) string {
	sessionID := str(data, "session_id", "unknown")
	h.CancelReplyWait(sessionID)
	pidFile := filepath.Join(h.WaitersDir, sessionID+".pid")
	pid := strconv.Itoa(os.Getpid())
	if os.MkdirAll(h.WaitersDir, 0o755) == nil {
		_ = os.WriteFile(pidFile, []byte(pid), 0o644)
	}

	group, summary := Describe(event, data)
	answer := h.askReply(notify.Request{
		Title: h.title(data, group), Body: summary, Sound: SoundSlug(group),
	}, wait, pidFile, pid)

	if answer == nil {
		// Helper unavailable: fall back to a plain notification
		h.Process(event, data)
		return ""
	}
	switch answer.Action {
	case "open":
		h.BringTerminalForward()
	case "reply":
		return strings.TrimSpace(answer.Text)
	}
	return ""
}

// askReply runs the interactive notification and drops our pid file afterwards,
// unless a newer waiter has replaced it.
func (h *Handler) askReply(r notify.Request, wait time.Duration, pidFile, pid string) *notify.Answer {
	defer func() {
		if b, err := os.ReadFile(pidFile); err == nil && strings.TrimSpace(string(b)) == pid {
			os.Remove(pidFile)
		}
	}()
	return h.Notifier.Ask(r, "reply", wait)
}

// CancelReplyWait withdraws a pending Reply notification for this session.
func (h *Handler) CancelReplyWait(sessionID string) {
	pidFile := filepath.Join(h.WaitersDir, sessionID+".pid")
	if b, err := os.ReadFile(pidFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
			stopWaiter(pid, pidFile)
		}
	}
	os.Remove(pidFile)
}

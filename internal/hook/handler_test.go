package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/franciuto/awesome-claude-notifier/internal/notify"
)

type fakeNotifier struct {
	answer  *notify.Answer
	onAsk   func()
	sent    []notify.Request
	asked   []notify.Request
	actions []string
	waits   []time.Duration
}

func (f *fakeNotifier) Send(r notify.Request) bool { f.sent = append(f.sent, r); return true }

func (f *fakeNotifier) Ask(r notify.Request, actions string, wait time.Duration) *notify.Answer {
	f.asked = append(f.asked, r)
	f.actions = append(f.actions, actions)
	f.waits = append(f.waits, wait)
	if f.onAsk != nil {
		f.onAsk()
	}
	return f.answer
}

func newHandler(t *testing.T, answer *notify.Answer) (*Handler, *fakeNotifier, *int) {
	t.Helper()
	n := &fakeNotifier{answer: answer}
	forward := 0
	return &Handler{
		Notifier:             n,
		Getenv:               func(string) string { return "" },
		UserAtTerminal:       func() bool { return false },
		BringTerminalForward: func() { forward++ },
		WaitersDir:           t.TempDir(),
	}, n, &forward
}

var permData = map[string]any{
	"session_id": "t", "cwd": "/tmp/demo", "tool_name": "Bash",
	"tool_input": map[string]any{"command": "ls -la"},
}

func TestPermissionAnswered(t *testing.T) {
	for action, want := range map[string]string{
		"allow": `{"hookSpecificOutput":{"hookEventName":"PermissionRequest","decision":{"behavior":"allow"}}}`,
		"deny":  `{"hookSpecificOutput":{"hookEventName":"PermissionRequest","decision":{"behavior":"deny","message":"The user denied permission from the notification."}}}`,
	} {
		h, _, _ := newHandler(t, &notify.Answer{Action: action})
		got, _ := json.Marshal(h.Permission(permData))
		var g, w any // key order is irrelevant to Claude Code
		json.Unmarshal(got, &g)
		json.Unmarshal([]byte(want), &w)
		if !reflect.DeepEqual(g, w) {
			t.Errorf("%s: got %s, want %s", action, got, want)
		}
	}
}

func TestPermissionUnanswered(t *testing.T) {
	for _, a := range []*notify.Answer{{Action: "timeout"}, {Action: "open"}, nil} {
		h, _, forward := newHandler(t, a)
		if out := h.Permission(permData); out != nil {
			t.Errorf("%v: got %v, want nil", a, out)
		}
		if wantForward := a != nil && a.Action == "open"; (*forward == 1) != wantForward {
			t.Errorf("%v: BringTerminalForward calls = %d", a, *forward)
		}
	}
}

func TestPermissionSkippedAtTerminalOrDesktop(t *testing.T) {
	h, n, _ := newHandler(t, &notify.Answer{Action: "allow"})
	h.UserAtTerminal = func() bool { return true }
	if out := h.Permission(permData); out != nil || len(n.asked) != 0 {
		t.Errorf("at terminal: out %v, asks %d", out, len(n.asked))
	}

	h, n, _ = newHandler(t, &notify.Answer{Action: "allow"})
	h.Getenv = func(k string) string {
		if k == "CLAUDE_CODE_ENTRYPOINT" {
			return "claude-desktop"
		}
		return ""
	}
	if out := h.Permission(permData); out != nil || len(n.asked) != 0 {
		t.Errorf("desktop: out %v, asks %d", out, len(n.asked))
	}
}

func TestPermissionRequestText(t *testing.T) {
	h, n, _ := newHandler(t, nil)
	h.Permission(permData)
	if len(n.asked) != 1 {
		t.Fatalf("asks = %d", len(n.asked))
	}
	r := n.asked[0]
	if r.Body != "Bash: ls -la" || r.Title != "demo · Needs you" || r.Sound != "needs-you" ||
		n.actions[0] != "permission" || n.waits[0] != 20*time.Second {
		t.Errorf("got %+v actions %q wait %v", r, n.actions[0], n.waits[0])
	}
}

func TestPermissionWithoutDetail(t *testing.T) {
	h, n, _ := newHandler(t, nil)
	h.Permission(map[string]any{"session_id": "t", "cwd": "/tmp/demo", "tool_name": "Read"})
	if n.asked[0].Body != "Asking to use Read" {
		t.Errorf("body %q", n.asked[0].Body)
	}
}

var stopData = map[string]any{"session_id": "t", "cwd": "/tmp/demo", "last_assistant_message": "Done."}

func TestReplyReturnsTrimmedText(t *testing.T) {
	for _, c := range []struct {
		a    notify.Answer
		want string
	}{
		{notify.Answer{Action: "reply", Text: " go on "}, "go on"},
		{notify.Answer{Action: "reply", Text: "  "}, ""},
		{notify.Answer{Action: "timeout"}, ""},
	} {
		h, _, _ := newHandler(t, &c.a)
		if got := h.NotifyAndWaitReply("Stop", stopData, time.Second); got != c.want {
			t.Errorf("%+v: got %q, want %q", c.a, got, c.want)
		}
	}
}

func TestReplyUnavailableFallsBackToPlain(t *testing.T) {
	h, n, _ := newHandler(t, nil)
	if got := h.NotifyAndWaitReply("Stop", stopData, time.Second); got != "" {
		t.Errorf("got %q", got)
	}
	if len(n.sent) != 1 {
		t.Errorf("sends = %d", len(n.sent))
	}
}

func TestReplyWritesAndRemovesPidFile(t *testing.T) {
	h, n, _ := newHandler(t, &notify.Answer{Action: "timeout"})
	pidFile := filepath.Join(h.WaitersDir, "t.pid")
	n.onAsk = func() {
		b, err := os.ReadFile(pidFile)
		if err != nil || string(b) != strconv.Itoa(os.Getpid()) {
			t.Errorf("during Ask: %q, %v", b, err)
		}
	}
	h.NotifyAndWaitReply("Stop", stopData, time.Second)
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Errorf("pid file left behind: %v", err)
	}
}

func TestProcessSendsGroupTitleAndSound(t *testing.T) {
	h, n, _ := newHandler(t, nil)
	h.Process("Stop", stopData)
	h.Process("StopFailure", map[string]any{"session_id": "t", "cwd": "/tmp/demo", "error": "rate_limit"})
	if len(n.sent) != 2 {
		t.Fatalf("sends = %d", len(n.sent))
	}
	if r := n.sent[0]; r.Title != "demo · Done" || r.Sound != "done" || r.Urgent {
		t.Errorf("Stop: %+v", r)
	}
	if r := n.sent[1]; r.Sound != "error" || !r.Urgent {
		t.Errorf("StopFailure: %+v", r)
	}
}

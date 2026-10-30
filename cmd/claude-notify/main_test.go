package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

type fakeHook struct {
	permission    map[string]any
	reply         string
	desktop       bool
	processOK     bool
	processPanics bool

	cancelled []string
	processed []string
	asked     []string
}

func (f *fakeHook) Permission(map[string]any) map[string]any { return f.permission }
func (f *fakeHook) CancelReplyWait(id string)                { f.cancelled = append(f.cancelled, id) }
func (f *fakeHook) InDesktopApp() bool                       { return f.desktop }
func (f *fakeHook) NotifyAndWaitReply(event string, _ map[string]any, _ time.Duration) string {
	f.asked = append(f.asked, event)
	return f.reply
}
func (f *fakeHook) Process(event string, _ map[string]any) bool {
	if f.processPanics {
		panic("boom")
	}
	f.processed = append(f.processed, event)
	return f.processOK
}

func do(h Hook, stdin string, args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errb, h)
	return code, out.String(), errb.String()
}

func TestUsageErrorIsExit1(t *testing.T) {
	if code, _, _ := do(&fakeHook{}, `{"hook_event_name":"Stop"}`, "hook", "--bogus"); code != 1 {
		t.Fatalf("got %d, want 1", code)
	}
}

func TestUnknownCommandIsExit1(t *testing.T) {
	if code, _, _ := do(&fakeHook{}, "{}", "send"); code != 1 {
		t.Fatalf("send: got %d, want 1", code)
	}
	if code, _, _ := do(&fakeHook{}, "{}"); code != 1 {
		t.Fatalf("no args: got %d, want 1", code)
	}
}

func TestBadStdinIsExit1(t *testing.T) {
	for _, in := range []string{"", "null", "[]", `"x"`, "{", "{}"} {
		code, _, stderr := do(&fakeHook{}, in, "hook")
		if code != 1 || stderr == "" {
			t.Errorf("stdin %q: code %d, stderr %q; want 1 and a message", in, code, stderr)
		}
	}
}

func TestMissingEventIsExit1(t *testing.T) {
	code, _, stderr := do(&fakeHook{}, `{"session_id":"t"}`, "hook")
	if code != 1 || stderr == "" {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestPanicIsExit1(t *testing.T) {
	code, _, stderr := do(&fakeHook{processPanics: true}, `{"hook_event_name":"Stop"}`, "hook")
	if code != 1 || stderr == "" {
		t.Fatalf("code %d, stderr %q; want 1 and a message", code, stderr)
	}
}

func TestPermissionPrintsDecisionOnlyWhenAnswered(t *testing.T) {
	in := `{"hook_event_name":"PermissionRequest"}`
	h := &fakeHook{permission: map[string]any{"hookSpecificOutput": map[string]any{"decision": "allow"}}}
	code, stdout, _ := do(h, in, "hook")
	if code != 0 || strings.TrimSpace(stdout) != `{"hookSpecificOutput":{"decision":"allow"}}` {
		t.Fatalf("answered: code %d, stdout %q", code, stdout)
	}
	code, stdout, _ = do(&fakeHook{}, in, "hook")
	if code != 0 || stdout != "" {
		t.Fatalf("unanswered: code %d, stdout %q", code, stdout)
	}
}

func TestUserPromptSubmitCancels(t *testing.T) {
	h := &fakeHook{}
	code, _, _ := do(h, `{"hook_event_name":"UserPromptSubmit","session_id":"t"}`, "hook")
	if code != 0 || len(h.cancelled) != 1 || h.cancelled[0] != "t" {
		t.Fatalf("code %d, cancelled %v", code, h.cancelled)
	}
}

func TestReplyExit2WithText(t *testing.T) {
	h := &fakeHook{reply: "go on"}
	code, _, stderr := do(h, `{"hook_event_name":"Stop","session_id":"t"}`, "hook", "--reply-wait", "60")
	prefix := "Message from the user, sent by replying to a desktop notification."
	if code != 2 || !strings.Contains(stderr, "go on") || !strings.Contains(stderr, prefix) {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestNoReplyExit0(t *testing.T) {
	h := &fakeHook{}
	code, _, _ := do(h, `{"hook_event_name":"Stop","session_id":"t"}`, "hook", "--reply-wait", "60")
	if code != 0 || len(h.asked) != 1 {
		t.Fatalf("code %d, asked %v", code, h.asked)
	}
}

func TestDesktopAppSkipsReply(t *testing.T) {
	h := &fakeHook{desktop: true, processOK: true}
	code, _, _ := do(h, `{"hook_event_name":"Stop","session_id":"t"}`, "hook", "--reply-wait", "60")
	if code != 0 || len(h.asked) != 0 || len(h.processed) != 1 {
		t.Fatalf("code %d, asked %v, processed %v", code, h.asked, h.processed)
	}
}

func TestProcessFailureStillExit0(t *testing.T) {
	code, _, stderr := do(&fakeHook{}, `{"hook_event_name":"Stop"}`, "hook")
	if code != 0 || !strings.Contains(stderr, "Warning") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

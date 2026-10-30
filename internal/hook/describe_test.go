package hook

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSummarizeKeepsUnderscoresStripsMarkdown(t *testing.T) {
	got := Summarize("**Fixed** `hook_handler.py`", 120)
	if got != "Fixed hook_handler.py" {
		t.Fatalf("got %q", got)
	}
}

func TestSummarizeCutsLongText(t *testing.T) {
	got := Summarize(strings.Repeat("word ", 100), 120)
	if n := utf8.RuneCountInString(got); n > 120 {
		t.Fatalf("%d runes: %q", n, got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("no ellipsis: %q", got)
	}
}

func TestSummarizeKeepsWholeSentences(t *testing.T) {
	got := Summarize("One. Two two. "+strings.Repeat("x", 130), 120)
	if got != "One. Two two." {
		t.Fatalf("got %q", got)
	}
}

func TestSummarizeUnicode(t *testing.T) {
	got := Summarize(strings.Repeat("perché è così 🎉 ", 20), 120)
	if !utf8.ValidString(got) {
		t.Fatalf("invalid UTF-8: %q", got)
	}
	if n := utf8.RuneCountInString(got); n > 120 {
		t.Fatalf("%d runes", n)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("no ellipsis: %q", got)
	}
}

func TestSummarizeStripsCodeBlocksLinksBullets(t *testing.T) {
	got := Summarize("```go\nx\n```\n- [doc](http://x) here", 120)
	if got != "doc here" {
		t.Fatalf("got %q", got)
	}
}

func TestSoundSlug(t *testing.T) {
	cases := map[string]string{NeedsYou: "needs-you", Done: "done", Paused: "paused", Error: "error", Limit: "limit", Info: "info", "nope": ""}
	for group, want := range cases {
		if got := SoundSlug(group); got != want {
			t.Errorf("SoundSlug(%q) = %q, want %q", group, got, want)
		}
	}
}

func TestDescribe(t *testing.T) {
	type m = map[string]any
	tests := []struct {
		name        string
		event       string
		data        m
		wantGroup   string
		wantSummary string
	}{
		{"stop", "Stop", m{"last_assistant_message": "All **done**."}, Done, "All done."},
		{"stop empty", "Stop", m{}, Done, "Finished responding"},
		{"stop background tasks", "Stop", m{"background_tasks": []any{"x"}, "last_assistant_message": "Waiting."}, Paused, "Waiting."},
		{"stop empty background tasks", "Stop", m{"background_tasks": []any{}}, Done, "Finished responding"},
		{"notification permission", "Notification", m{"notification_type": "permission_prompt", "message": "Claude needs your permission to use Bash"}, NeedsYou, "Asking to use Bash"},
		{"notification fallback text", "Notification", m{"notification_type": "idle_prompt", "message": "ignored"}, NeedsYou, "Waiting for your reply"},
		{"notification done type", "Notification", m{"notification_type": "agent_completed"}, Done, "A background agent finished"},
		{"notification limit type", "Notification", m{"notification_type": "quota_auto_resume_fired"}, Limit, "Usage limit reset, work resumed"},
		{"notification unknown type with message", "Notification", m{"notification_type": "weird", "message": "Hello **there**"}, Info, "Hello there"},
		{"notification unknown type no message", "Notification", m{"notification_type": "weird"}, Info, "Notification"},
		{"notification no type", "Notification", m{}, Info, "Notification"},
		{"notification known type ignores message", "Notification", m{"notification_type": "auth_success", "message": "Logged in as x"}, Info, "Signed in"},
		{"stopfailure known with details", "StopFailure", m{"error": "rate_limit", "error_details": "Try later"}, Error, "Rate limit reached. Try later"},
		{"stopfailure falls back to last message", "StopFailure", m{"error": "overloaded", "last_assistant_message": "Oops"}, Error, "Server overloaded. Oops"},
		{"stopfailure unknown error", "StopFailure", m{"error": "boom"}, Error, "boom"},
		{"stopfailure no error", "StopFailure", m{}, Error, "unknown"},
		{"permission denied", "PermissionDenied", m{"tool_name": "Bash", "reason": "Too risky"}, Error, "Auto mode blocked Bash. Too risky"},
		{"permission denied defaults", "PermissionDenied", m{}, Error, "Auto mode blocked a tool."},
		{"permission request", "PermissionRequest", m{"tool_name": "Edit"}, NeedsYou, "Asking to use Edit"},
		{"permission request default", "PermissionRequest", m{}, NeedsYou, "Asking to use a tool"},
		{"subagent stop", "SubagentStop", m{"last_assistant_message": "Found it."}, Done, "Found it."},
		{"subagent stop empty", "SubagentStop", m{"last_assistant_message": ""}, Done, "A subagent finished"},
		{"task completed", "TaskCompleted", m{"task_subject": "Write tests"}, Done, "Write tests"},
		{"task completed default", "TaskCompleted", m{}, Done, "Task completed"},
		{"unknown event", "SomethingNew", m{}, Info, "SomethingNew"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group, summary := Describe(tt.event, tt.data, "en")
			if group != tt.wantGroup || summary != tt.wantSummary {
				t.Fatalf("got (%q, %q), want (%q, %q)", group, summary, tt.wantGroup, tt.wantSummary)
			}
		})
	}
}

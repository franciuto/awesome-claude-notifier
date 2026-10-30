package notify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadAnswer(t *testing.T) {
	write := func(content string) string {
		f := filepath.Join(t.TempDir(), "a.json")
		if err := os.WriteFile(f, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return f
	}

	f := write(`{"action":"allow"}`)
	if a := readAnswer(f, true); a == nil || a.Action != "allow" {
		t.Fatalf("allow: %v", a)
	}
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Fatal("answer file not deleted")
	}
	if a := readAnswer(write(`{"action":"reply","text":"hi"}`), true); a == nil || a.Text != "hi" {
		t.Fatalf("reply: %v", a)
	}
	if a := readAnswer(write(`{"action":"error"}`), true); a != nil {
		t.Fatalf("error: %v", a)
	}
	missing := filepath.Join(t.TempDir(), "none.json")
	if a := readAnswer(missing, true); a == nil || a.Action != "timeout" {
		t.Fatalf("missing+exitOK: %v", a)
	}
	if a := readAnswer(missing, false); a != nil {
		t.Fatalf("missing+!exitOK: %v", a)
	}
	if a := readAnswer(write(`not json`), true); a != nil {
		t.Fatalf("invalid: %v", a)
	}
}

func TestPluginRootFromEnv(t *testing.T) {
	t.Setenv("CLAUDE_PLUGIN_ROOT", "/x")
	if got := PluginRoot(); got != "/x" {
		t.Fatalf("got %q", got)
	}
}

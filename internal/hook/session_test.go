package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func transcript(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.jsonl")
	writeFile(t, path, strings.Join(lines, "\n")+"\n")
	return path
}

func TestSessionNamePrefersCustomTitle(t *testing.T) {
	h := &Handler{}
	both := transcript(t,
		`{"type":"ai-title","aiTitle":"Auto"}`,
		`{"type":"custom-title","customTitle":"Mine"}`,
		`{"type":"ai-title","aiTitle":"Auto 2"}`)
	if got := h.SessionName(map[string]any{"transcript_path": both}); got != "Mine" {
		t.Fatalf("got %q", got)
	}
	onlyAI := transcript(t, `{"type":"user"}`, `{"type":"ai-title","aiTitle":"Auto"}`)
	if got := h.SessionName(map[string]any{"transcript_path": onlyAI}); got != "Auto" {
		t.Fatalf("got %q", got)
	}
}

func TestSessionNameHugeLine(t *testing.T) {
	huge := `{"type":"assistant","text":"` + strings.Repeat("a", 1<<20) + `"}`
	path := transcript(t, huge, `{"type":"custom-title","customTitle":"After huge"}`)
	if got := (&Handler{}).SessionName(map[string]any{"transcript_path": path}); got != "After huge" {
		t.Fatalf("got %q", got)
	}
}

func TestSessionNameDesktopApp(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a", "b", "local_1.json"), `{"cliSessionId":"s1","title":"Desk"}`)
	writeFile(t, filepath.Join(dir, "a", "b", "local_2.json"), `{"cliSessionId":"other","title":"Wrong"}`)
	h := &Handler{DesktopSessionsDirs: []string{filepath.Join(dir, "missing"), dir}}
	if got := h.SessionName(map[string]any{"session_id": "s1"}); got != "Desk" {
		t.Fatalf("got %q", got)
	}
}

func TestSessionNameFallbacks(t *testing.T) {
	h := &Handler{}
	if got := h.SessionName(map[string]any{"cwd": "/tmp/demo", "transcript_path": "/nonexistent/t.jsonl"}); got != "demo" {
		t.Fatalf("got %q", got)
	}
	if got := h.SessionName(map[string]any{}); got != "Session" {
		t.Fatalf("got %q", got)
	}
	if got := h.SessionName(map[string]any{"cwd": "/"}); got != "Session" {
		t.Fatalf("root cwd: got %q", got)
	}
}

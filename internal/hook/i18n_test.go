package hook

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDescribeItalian(t *testing.T) {
	type m = map[string]any
	tests := []struct {
		event       string
		data        m
		wantSummary string
	}{
		{"Stop", m{}, "Ha finito di rispondere"},
		{"PermissionRequest", m{"tool_name": "Bash"}, "Chiede di usare Bash"},
		{"PermissionRequest", m{}, "Chiede di usare un tool"},
		{"Notification", m{"notification_type": "idle_prompt"}, "Aspetta una tua risposta"},
		{"Notification", m{"message": "Claude needs your permission to use Edit"}, "Chiede di usare Edit"},
		{"StopFailure", m{"error": "rate_limit"}, "Limite di richieste raggiunto"},
		{"PermissionDenied", m{"tool_name": "Bash", "reason": "Rischioso"}, "Auto mode ha bloccato Bash. Rischioso"},
	}
	for _, tt := range tests {
		if _, got := Describe(tt.event, tt.data, "it"); got != tt.wantSummary {
			t.Errorf("%s %v: got %q, want %q", tt.event, tt.data, got, tt.wantSummary)
		}
	}
	// Unknown language falls back to English
	if _, got := Describe("Stop", m{}, "xx"); got != "Finished responding" {
		t.Errorf("unknown language: got %q", got)
	}
}

func TestClassify(t *testing.T) {
	cases := map[string]string{
		"puoi sistemare il bug che non fa partire la build? voglio che sia veloce": "it",
		"can you fix the bug that is breaking the build? it should be fast":        "en",
		"ok":                 "", // too few hits
		"il the e and di of": "", // no clear winner
	}
	for sample, want := range cases {
		if got := classify(sample); got != want {
			t.Errorf("classify(%q) = %q, want %q", sample, got, want)
		}
	}
}

func TestDetectLanguageFromTranscript(t *testing.T) {
	path := transcript(t,
		`{"type":"user","message":{"role":"user","content":"<command-name>/clear</command-name> the the the the"}}`,
		`{"type":"user","message":{"role":"user","content":"adesso fammi vedere come è fatto il codice che hai scritto"}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Ecco."}]}}`,
	)
	if got := detectLanguage(map[string]any{"transcript_path": path}, ""); got != "it" {
		t.Fatalf("got %q, want it (harness messages skipped)", got)
	}
}

func TestDetectLanguageFallsBackToSettingsThenEnglish(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")
	writeFile(t, settings, `{"language": " Italiano "}`)
	if got := detectLanguage(map[string]any{}, settings); got != "it" {
		t.Fatalf("settings: got %q", got)
	}
	writeFile(t, settings, `{"language": "it"}`)
	if got := detectLanguage(map[string]any{}, settings); got != "it" {
		t.Fatalf("settings code: got %q", got)
	}
	writeFile(t, settings, `{"language": "klingon"}`)
	if got := detectLanguage(map[string]any{}, settings); got != "en" {
		t.Fatalf("unknown setting: got %q", got)
	}
	if got := detectLanguage(map[string]any{"transcript_path": filepath.Join(dir, "missing.jsonl")}, ""); got != "en" {
		t.Fatalf("nothing: got %q", got)
	}
}

func TestRecentChatTextReadsOnlyTheTail(t *testing.T) {
	old := `{"type":"user","message":{"content":"` + strings.Repeat("the ", transcriptTailBytes/4) + `"}}`
	path := transcript(t, old, `{"type":"user","message":{"content":"ciao"}}`)
	if got := recentChatText(path); len(got) != 1 || got[0] != "ciao" {
		t.Fatalf("got %d texts, want only the recent one", len(got))
	}
}

func TestTitleFallbackFollowsLanguage(t *testing.T) {
	h, n, _ := newHandler(t, nil)
	h.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, h.SettingsPath, `{"language": "italian"}`)
	h.Process("Stop", map[string]any{})
	if r := n.sent[0]; r.Title != "Sessione · Done" || r.Body != "Ha finito di rispondere" {
		t.Fatalf("got title %q body %q", r.Title, r.Body)
	}
}

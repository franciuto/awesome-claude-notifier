//go:build darwin

package notify

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// fakeApp builds <dir>/<name>.app with an Info.plist and a marker file.
func fakeApp(t *testing.T, dir, id, version string) string {
	t.Helper()
	app := filepath.Join(dir, "Claude Notify.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents"), 0o755); err != nil {
		t.Fatal(err)
	}
	plist := filepath.Join(app, "Contents", "Info.plist")
	for _, args := range [][]string{
		{"-create", "xml1", plist},
		{"-insert", "CFBundleIdentifier", "-string", id, plist},
		{"-insert", "CFBundleVersion", "-string", version, plist},
	} {
		if out, err := exec.Command("plutil", args...).CombinedOutput(); err != nil {
			t.Fatalf("plutil %v: %v: %s", args, err, out)
		}
	}
	return app
}

// putSounds writes name -> content files into <app>/Contents/Resources.
func putSounds(t *testing.T, app string, files map[string]string) {
	t.Helper()
	res := filepath.Join(app, "Contents", "Resources")
	if err := os.MkdirAll(res, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(res, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func plistKey(t *testing.T, app, key string) string {
	t.Helper()
	v, err := plistValue(app, key)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestEnsureAppCopiesWhenMissing(t *testing.T) {
	bundled := fakeApp(t, t.TempDir(), "com.example.notify", "1")
	installed := filepath.Join(t.TempDir(), "Claude Notify.app")
	if err := ensureApp(bundled, installed, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if got := plistKey(t, installed, "CFBundleVersion"); got != "1" {
		t.Fatalf("version %q", got)
	}
}

func TestEnsureAppUpdatesSameBundleID(t *testing.T) {
	bundled := fakeApp(t, t.TempDir(), "com.example.notify", "2")
	installed := fakeApp(t, t.TempDir(), "com.example.notify", "1")
	if err := ensureApp(bundled, installed, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if got := plistKey(t, installed, "CFBundleVersion"); got != "2" {
		t.Fatalf("version %q", got)
	}
}

func TestEnsureAppKeepsForeignBundleID(t *testing.T) {
	bundled := fakeApp(t, t.TempDir(), "com.example.notify", "2")
	installed := fakeApp(t, t.TempDir(), "com.example.other", "1")
	if err := ensureApp(bundled, installed, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if got := plistKey(t, installed, "CFBundleIdentifier"); got != "com.example.other" {
		t.Fatalf("overwritten: %q", got)
	}
}

func TestEnsureAppNoBundledIsNoop(t *testing.T) {
	installed := filepath.Join(t.TempDir(), "Claude Notify.app")
	if err := ensureApp(filepath.Join(t.TempDir(), "missing.app"), installed, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(installed); !os.IsNotExist(err) {
		t.Fatal("installed something from nothing")
	}
}

func TestEnsureAppInstallsSounds(t *testing.T) {
	bundled := fakeApp(t, t.TempDir(), "com.example.notify", "1")
	putSounds(t, bundled, map[string]string{
		"claude-notify-done.aiff": "done", "claude-notify-error.aiff": "error", "AppIcon.icns": "icon"})
	installed := filepath.Join(t.TempDir(), "Claude Notify.app")
	sounds := filepath.Join(t.TempDir(), "Library", "Sounds") // does not exist yet
	if err := ensureApp(bundled, installed, sounds); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"claude-notify-done.aiff": "done", "claude-notify-error.aiff": "error"} {
		if got := readFile(t, filepath.Join(sounds, name)); got != want {
			t.Fatalf("%s = %q", name, got)
		}
	}
	if entries, _ := os.ReadDir(sounds); len(entries) != 2 {
		t.Fatalf("expected only the 2 sounds (no temp files, no icon), got %d entries", len(entries))
	}
}

func TestEnsureAppRestoresMissingSound(t *testing.T) {
	bundled := fakeApp(t, t.TempDir(), "com.example.notify", "1")
	putSounds(t, bundled, map[string]string{"claude-notify-done.aiff": "new-done", "claude-notify-error.aiff": "new-error"})
	installed := fakeApp(t, t.TempDir(), "com.example.notify", "1") // up to date
	sounds := t.TempDir()
	kept := filepath.Join(sounds, "claude-notify-error.aiff")
	if err := os.WriteFile(kept, []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureApp(bundled, installed, sounds); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(sounds, "claude-notify-done.aiff")); got != "new-done" {
		t.Fatalf("missing sound not restored: %q", got)
	}
	if got := readFile(t, kept); got != "custom" {
		t.Fatalf("existing sound touched: %q", got)
	}
}

func TestEnsureAppUpdatesSoundsWithApp(t *testing.T) {
	bundled := fakeApp(t, t.TempDir(), "com.example.notify", "2")
	putSounds(t, bundled, map[string]string{"claude-notify-done.aiff": "v2"})
	installed := fakeApp(t, t.TempDir(), "com.example.notify", "1")
	sounds := t.TempDir()
	if err := os.WriteFile(filepath.Join(sounds, "claude-notify-done.aiff"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureApp(bundled, installed, sounds); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(sounds, "claude-notify-done.aiff")); got != "v2" {
		t.Fatalf("sound not replaced: %q", got)
	}
}

func TestEnsureAppForeignAppStillGetsMissingSounds(t *testing.T) {
	bundled := fakeApp(t, t.TempDir(), "com.example.notify", "2")
	putSounds(t, bundled, map[string]string{"claude-notify-done.aiff": "done"})
	installed := fakeApp(t, t.TempDir(), "com.example.other", "1")
	sounds := t.TempDir()
	if err := ensureApp(bundled, installed, sounds); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(sounds, "claude-notify-done.aiff")); got != "done" {
		t.Fatalf("sound = %q", got)
	}
}

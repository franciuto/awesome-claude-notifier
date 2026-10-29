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
	if err := ensureApp(bundled, installed); err != nil {
		t.Fatal(err)
	}
	if got := plistKey(t, installed, "CFBundleVersion"); got != "1" {
		t.Fatalf("version %q", got)
	}
}

func TestEnsureAppUpdatesSameBundleID(t *testing.T) {
	bundled := fakeApp(t, t.TempDir(), "com.example.notify", "2")
	installed := fakeApp(t, t.TempDir(), "com.example.notify", "1")
	if err := ensureApp(bundled, installed); err != nil {
		t.Fatal(err)
	}
	if got := plistKey(t, installed, "CFBundleVersion"); got != "2" {
		t.Fatalf("version %q", got)
	}
}

func TestEnsureAppKeepsForeignBundleID(t *testing.T) {
	bundled := fakeApp(t, t.TempDir(), "com.example.notify", "2")
	installed := fakeApp(t, t.TempDir(), "com.example.other", "1")
	if err := ensureApp(bundled, installed); err != nil {
		t.Fatal(err)
	}
	if got := plistKey(t, installed, "CFBundleIdentifier"); got != "com.example.other" {
		t.Fatalf("overwritten: %q", got)
	}
}

func TestEnsureAppNoBundledIsNoop(t *testing.T) {
	installed := filepath.Join(t.TempDir(), "Claude Notify.app")
	if err := ensureApp(filepath.Join(t.TempDir(), "missing.app"), installed); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(installed); !os.IsNotExist(err) {
		t.Fatal("installed something from nothing")
	}
}

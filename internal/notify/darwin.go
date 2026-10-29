//go:build darwin

package notify

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type darwin struct{}

// New returns the notifier for the current OS.
func New() Notifier { return darwin{} }

func installedApp() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Applications", "Claude Notify.app")
}

func bundledApp() string { return filepath.Join(PluginRoot(), "macos", "Claude Notify.app") }

func helperBinary(app string) string {
	return filepath.Join(app, "Contents", "MacOS", "claude-notify-helper")
}

func plistValue(app, key string) (string, error) {
	out, err := exec.Command("plutil", "-extract", key, "raw", "-o", "-",
		filepath.Join(app, "Contents", "Info.plist")).Output()
	return strings.TrimSpace(string(out)), err
}

// ensureApp copies the bundled helper app to installed when it is missing or an
// older build of the same app. A different bundle id is the user's own app: left alone.
func ensureApp(bundled, installed string) error {
	if _, err := os.Stat(bundled); err != nil {
		return nil // dev checkout without a build: use whatever is installed
	}
	if _, err := os.Stat(installed); err == nil {
		id, err := plistValue(installed, "CFBundleIdentifier")
		if err != nil {
			return err
		}
		bundledID, err := plistValue(bundled, "CFBundleIdentifier")
		if err != nil {
			return err
		}
		if id != bundledID {
			return nil
		}
		version, err := plistValue(installed, "CFBundleVersion")
		if err != nil {
			return err
		}
		bundledVersion, err := plistValue(bundled, "CFBundleVersion")
		if err != nil {
			return err
		}
		if version == bundledVersion {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(installed), 0o755); err != nil {
		return err
	}
	if out, err := exec.Command("ditto", bundled, installed).CombinedOutput(); err != nil {
		return fmt.Errorf("ditto: %v: %s", err, out)
	}
	return nil
}

func prepare() string {
	app := installedApp()
	if err := ensureApp(bundledApp(), app); err != nil {
		fmt.Fprintln(os.Stderr, "claude-notify: installing the helper app:", err)
	}
	return app
}

func soundArgs(r Request) []string {
	if r.Sound == "" {
		return nil
	}
	return []string{"--sound", "claude-notify-" + r.Sound + ".aiff"}
}

func (darwin) Send(r Request) bool {
	helper := helperBinary(prepare())
	if st, err := os.Stat(helper); err == nil && st.Mode()&0o111 != 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		args := append([]string{"--title", r.Title, "--body", r.Body}, soundArgs(r)...)
		if exec.CommandContext(ctx, helper, args...).Run() == nil {
			return true
		}
	}
	// Escape for AppleScript string literals so quotes in the text can't break out
	esc := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace
	script := fmt.Sprintf(`display notification "%s" with title "%s" subtitle "Claude" with sound`,
		esc(r.Body), esc(r.Title))
	return exec.Command("osascript", "-e", script).Run() == nil
}

func (darwin) Ask(r Request, actions string, wait time.Duration) *Answer {
	app := prepare()
	if st, err := os.Stat(helperBinary(app)); err != nil || st.Mode()&0o111 == 0 {
		return nil
	}
	secs := int(wait / time.Second)
	b := make([]byte, 16)
	rand.Read(b)
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	id := fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
	dir := filepath.Join(CacheDir(), "responses")
	answerFile, pidFile := filepath.Join(dir, id+".json"), filepath.Join(dir, id+".pid")

	// Launched through LaunchServices, not exec'd: only then does macOS route the
	// click (and the typed reply) back to the helper.
	args := append([]string{"-n", "-g", "-W", "-a", app, "--args",
		"--title", r.Title, "--body", r.Body, "--actions", actions,
		"--wait", strconv.Itoa(secs), "--id", id, "--parent", strconv.Itoa(os.Getpid())}, soundArgs(r)...)
	cmd := exec.Command("open", args...)
	if cmd.Start() != nil {
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	stopHelper := func() {
		if data, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
				syscall.Kill(pid, syscall.SIGTERM)
			}
		}
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer signal.Stop(sigs)
	timeout := time.NewTimer(wait + 15*time.Second)
	defer timeout.Stop()

	var waitErr error
	select {
	case waitErr = <-done:
	case <-sigs:
		// Answer no longer wanted: take the notification with us
		stopHelper()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
		os.Exit(0)
	case <-timeout.C:
		stopHelper()
		cmd.Process.Kill()
		waitErr = <-done
	}
	return readAnswer(answerFile, waitErr == nil)
}

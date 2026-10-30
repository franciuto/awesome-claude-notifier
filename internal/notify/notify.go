// Package notify posts desktop notifications and waits for the user's answer.
package notify

import (
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Request is one notification. Sound is a group slug, "" for the default sound.
type Request struct {
	Title, Body, Sound string
	Urgent             bool
}

// Answer is what the user did with an interactive notification:
// allow, deny, reply, open or timeout.
type Answer struct{ Action, Text string }

// Notifier posts notifications for the current OS.
type Notifier interface {
	Send(r Request) bool
	// Ask shows buttons ("permission") or a reply field ("reply") and waits.
	// nil means interactive notifications are unavailable.
	Ask(r Request, actions string, wait time.Duration) *Answer
}

// CacheDir is where waiter pid files and helper responses live.
func CacheDir() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "claude-notify", "cache")
	}
	return filepath.Join(home, "Library", "Caches", "claude-notify")
}

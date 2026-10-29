package hook

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Used when the terminal app can't be told from __CFBundleIdentifier.
var terminalBundleIDs = []string{
	"dev.warp.Warp-Stable", "com.apple.Terminal", "com.googlecode.iterm2",
	"com.mitchellh.ghostty", "net.kovidgoyal.kitty", "com.github.wez.wezterm",
	"io.alacritty", "com.microsoft.VSCode", "com.todesktop.230313mzl4w4u92",
}

var bundleIDRe = regexp.MustCompile(`"?(?:bundleID|CFBundleIdentifier)"?\s*=\s*"([^"]+)"`)

func lsappinfo(args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, "lsappinfo", args...).Output()
	return strings.TrimSpace(string(out))
}

// userAtTerminal is true when the terminal is the frontmost app, so the normal
// dialog is quicker.
func userAtTerminal() bool {
	front := lsappinfo("front")
	m := bundleIDRe.FindStringSubmatch(lsappinfo("info", "-only", "bundleid", front))
	if m == nil {
		return false
	}
	if own := os.Getenv("__CFBundleIdentifier"); own != "" {
		return m[1] == own
	}
	return slices.Contains(terminalBundleIDs, m[1])
}

// bringTerminalForward: clicking the notification itself takes you to the terminal.
func bringTerminalForward() {
	if own := os.Getenv("__CFBundleIdentifier"); own != "" {
		_ = exec.Command("open", "-b", own).Run()
	}
}

func desktopSessionsDirs() []string {
	home, _ := os.UserHomeDir()
	return []string{filepath.Join(home, "Library", "Application Support", "Claude", "claude-code-sessions")}
}

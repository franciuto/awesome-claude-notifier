//go:build !darwin && !windows

package hook

import (
	"os"
	"path/filepath"
)

func userAtTerminal() bool { return false }

func bringTerminalForward() {}

func desktopSessionsDirs() []string {
	home, _ := os.UserHomeDir()
	return []string{filepath.Join(home, "Library", "Application Support", "Claude", "claude-code-sessions")}
}

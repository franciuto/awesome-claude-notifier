package hook

import (
	"os"
	"path/filepath"
)

// userAtTerminal is false until the Windows process helpers arrive.
func userAtTerminal() bool { return false }

// bringTerminalForward is done by the toast script: only the process the click
// went to may change the foreground.
func bringTerminalForward() {}

// Installer build under %APPDATA%, Microsoft Store build in its package's redirected copy.
func desktopSessionsDirs() []string {
	var dirs []string
	if appData := os.Getenv("APPDATA"); appData != "" {
		dirs = append(dirs, filepath.Join(appData, "Claude", "claude-code-sessions"))
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		m, _ := filepath.Glob(filepath.Join(local, "Packages", "Claude_*", "LocalCache", "Roaming", "Claude", "claude-code-sessions"))
		dirs = append(dirs, m...)
	}
	return dirs
}

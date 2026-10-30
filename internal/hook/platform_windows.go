package hook

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/franciuto/awesome-claude-notifier/internal/winproc"
)

// Used when the foreground app isn't one of this session's parent processes.
var terminalExes = []string{
	"windowsterminal.exe", "conhost.exe", "openconsole.exe", "cmd.exe", "powershell.exe",
	"pwsh.exe", "mintty.exe", "wezterm-gui.exe", "alacritty.exe", "warp.exe", "tabby.exe",
	"code.exe", "cursor.exe", "windsurf.exe",
}

// userAtTerminal: the foreground window belongs to a parent of this process, or to a known terminal.
func userAtTerminal() bool {
	pid, exe, procs, err := winproc.ForegroundApp()
	if err != nil {
		return false
	}
	return slices.Contains(winproc.Ancestors(uint32(os.Getpid()), procs), pid) || slices.Contains(terminalExes, exe)
}

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

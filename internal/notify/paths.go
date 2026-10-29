package notify

import (
	"os"
	"path/filepath"
)

// PluginRoot is the plugin directory: CLAUDE_PLUGIN_ROOT, else three levels
// above the binary (<root>/bin/<platform>/claude-notify).
func PluginRoot() string {
	if root := os.Getenv("CLAUDE_PLUGIN_ROOT"); root != "" {
		return root
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(exe)))
}

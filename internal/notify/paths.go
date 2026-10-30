package notify

import (
	"crypto/rand"
	"fmt"
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

// newID is a random UUID v4, the name of one interactive notification's files.
func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}

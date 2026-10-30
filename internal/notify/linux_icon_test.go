//go:build !darwin && !windows

package notify

import "testing"

func TestIconPath(t *testing.T) {
	t.Setenv("CLAUDE_PLUGIN_ROOT", "/p")
	if got := iconPath(); got != "/p/assets/icon/app-icon.png" {
		t.Fatalf("got %s", got)
	}
}

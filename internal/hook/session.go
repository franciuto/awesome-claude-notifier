package hook

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/franciuto/awesome-claude-notifier/internal/notify"
)

// Handler turns hook events into notifications.
type Handler struct {
	Notifier             notify.Notifier
	Getenv               func(string) string
	UserAtTerminal       func() bool // true when the terminal is in front: the normal dialog is quicker
	BringTerminalForward func()
	WaitersDir           string // pid files, one pending Reply per session

	// DesktopSessionsDirs are where the Claude desktop app keeps its session list.
	DesktopSessionsDirs []string

	// SettingsPath is Claude Code's settings.json, read for the "language" setting.
	SettingsPath string
}

// SessionName is the name shown in Claude's session list: the custom title, then
// the auto-generated one, then the desktop app title, then the folder name, then "".
func (h *Handler) SessionName(data map[string]any) string {
	title := titleFromTranscript(str(data, "transcript_path", ""))
	if title == "" {
		title = h.titleFromDesktopApp(str(data, "session_id", ""))
	}
	if title == "" {
		if cwd := str(data, "cwd", ""); cwd != "" {
			if base := filepath.Base(cwd); base != "." && base != "/" && base != string(filepath.Separator) {
				title = base
			}
		}
	}
	return title
}

// titleFromTranscript reads the latest custom-title / ai-title record.
func titleFromTranscript(path string) string {
	if path == "" {
		return ""
	}
	f, err := os.Open(expandHome(path))
	if err != nil {
		return ""
	}
	defer f.Close()

	var custom, auto string
	r := bufio.NewReader(f) // no line limit: transcript lines can be several MB
	for {
		line, err := r.ReadString('\n')
		// Cheap filter before parsing: title records are rare.
		if strings.Contains(line, `-title"`) {
			var rec struct {
				Type        string `json:"type"`
				CustomTitle string `json:"customTitle"`
				AITitle     string `json:"aiTitle"`
			}
			if json.Unmarshal([]byte(line), &rec) == nil {
				if rec.Type == "custom-title" && rec.CustomTitle != "" {
					custom = rec.CustomTitle
				} else if rec.Type == "ai-title" && rec.AITitle != "" {
					auto = rec.AITitle
				}
			}
		}
		if err != nil { // io.EOF or a read error: either way, stop
			break
		}
	}
	if custom != "" {
		return custom
	}
	return auto
}

// titleFromDesktopApp looks up the title the desktop app keeps for a CLI session id.
func (h *Handler) titleFromDesktopApp(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	for _, dir := range h.DesktopSessionsDirs {
		metas, _ := filepath.Glob(filepath.Join(dir, "*", "*", "local_*.json"))
		for _, meta := range metas {
			content, err := os.ReadFile(meta)
			if err != nil || !bytes.Contains(content, []byte(sessionID)) {
				continue
			}
			var rec struct {
				CLISessionID string `json:"cliSessionId"`
				Title        string `json:"title"`
			}
			if json.Unmarshal(content, &rec) == nil && rec.CLISessionID == sessionID && rec.Title != "" {
				return rec.Title
			}
		}
	}
	return ""
}

// expandHome resolves a leading "~/" in a transcript path.
func expandHome(path string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	return path
}

package notify

import (
	"path/filepath"
	"time"
)

// toastRequest is the JSON for toast.ps1 (keys documented in its header).
// Untagged so it is tested on every OS.
func toastRequest(r Request, actions string, wait time.Duration, id, response string, parent int,
	root, cache string, soundExists func(string) bool) map[string]any {
	sound := "default"
	if r.Sound == "none" {
		sound = "none"
	} else if r.Sound != "" {
		if mp3 := filepath.Join(root, "sounds", "mp3", r.Sound+".mp3"); soundExists(mp3) {
			sound = mp3
		}
	}
	return map[string]any{
		"title": r.Title, "body": r.Body, "sound": sound,
		"icon":    filepath.Join(root, "windows", "icon.png"),
		"actions": actions, "wait": int(wait / time.Second), "id": id,
		"response": response, "parent": parent, "cache": cache,
	}
}

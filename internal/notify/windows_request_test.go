package notify

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func exists(string) bool { return true }

func TestToastRequestPlain(t *testing.T) {
	root, cache := filepath.FromSlash("/plug"), filepath.FromSlash("/cache")
	got := toastRequest(Request{Title: "T", Body: "B", Sound: "done"}, "", 0, "id1", "", 0, root, cache, exists)
	want := map[string]any{
		"title": "T", "body": "B",
		"sound":   filepath.Join(root, "sounds", "mp3", "done.mp3"),
		"icon":    filepath.Join(root, "windows", "icon.png"),
		"actions": "", "wait": 0, "id": "id1", "response": "", "parent": 0, "cache": cache,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestToastRequestMissingSoundIsDefault(t *testing.T) {
	r := Request{Sound: "done"}
	if got := toastRequest(r, "", 0, "i", "", 0, "/p", "/c", func(string) bool { return false })["sound"]; got != "default" {
		t.Fatalf("missing mp3: %v", got)
	}
	if got := toastRequest(Request{}, "", 0, "i", "", 0, "/p", "/c", exists)["sound"]; got != "default" {
		t.Fatalf("no slug: %v", got)
	}
	if got := toastRequest(Request{Sound: "none"}, "", 0, "i", "", 0, "/p", "/c", exists)["sound"]; got != "none" {
		t.Fatalf("none: %v", got)
	}
}

func TestToastRequestPermissionWait(t *testing.T) {
	got := toastRequest(Request{Sound: "needs-you"}, "permission", 20*time.Second, "id2", `C:\r\id2.json`, 42, "/p", "/c", exists)
	if got["actions"] != "permission" || got["wait"] != 20 || got["parent"] != 42 || got["response"] != `C:\r\id2.json` {
		t.Fatalf("got %v", got)
	}
}

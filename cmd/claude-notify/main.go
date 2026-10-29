// Command claude-notify posts desktop notifications for Claude Code hooks.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

// Hook is what the hook command dispatches to; *hook.Handler implements it.
type Hook interface {
	Permission(data map[string]any) map[string]any // nil = print nothing
	CancelReplyWait(sessionID string)
	InDesktopApp() bool
	NotifyAndWaitReply(event string, data map[string]any, wait time.Duration) string // "" = no reply
	Process(event string, data map[string]any) bool
}

const replyPrefix = "Message from the user, sent by replying to a desktop notification. " +
	"Treat it as a new message from them and answer normally:\n\n"

// run returns the process exit code. Exit code 2 is reserved for a Reply:
// every other failure, panics included, returns 0 or 1.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer, h Hook) (code int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "Error: %v\n", r)
			code = 1
		}
	}()

	if len(args) == 0 || args[0] != "hook" {
		fmt.Fprintln(stderr, "Usage: claude-notify hook [--event-type NAME] [--reply-wait SECONDS]")
		return 1
	}

	fs := flag.NewFlagSet("hook", flag.ContinueOnError)
	fs.SetOutput(stderr)
	eventType := fs.String("event-type", "", "override the event type")
	fs.StringVar(eventType, "e", "", "override the event type")
	replyWait := fs.Int("reply-wait", 0, "offer Reply and wait this many seconds for a reply (use with asyncRewake)")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() > 0 {
		return 1
	}

	var data map[string]any
	raw, _ := io.ReadAll(stdin)
	if err := json.Unmarshal(raw, &data); err != nil || len(data) == 0 {
		fmt.Fprintln(stderr, "Error: No JSON data received from stdin")
		return 1
	}

	event := *eventType
	if event == "" {
		event, _ = data["hook_event_name"].(string)
	}
	if event == "" {
		fmt.Fprintln(stderr, "Error: no hook_event_name in the JSON data; pass --event-type")
		return 1
	}

	// Allow / Deny from the notification; no output leaves it to the terminal dialog
	if event == "PermissionRequest" {
		if out := h.Permission(data); out != nil {
			b, _ := json.Marshal(out)
			fmt.Fprintln(stdout, string(b))
		}
		return 0
	}

	// A prompt typed in the terminal makes a pending Reply pointless
	if event == "UserPromptSubmit" {
		sessionID, ok := data["session_id"].(string)
		if !ok {
			sessionID = "unknown"
		}
		h.CancelReplyWait(sessionID)
		return 0
	}

	// Reply: exit code 2 makes an asyncRewake hook wake Claude with stderr
	if *replyWait > 0 && !h.InDesktopApp() {
		if reply := h.NotifyAndWaitReply(event, data, time.Duration(*replyWait)*time.Second); reply != "" {
			fmt.Fprintln(stderr, replyPrefix+reply)
			return 2
		}
		return 0
	}

	if !h.Process(event, data) {
		// Not an error code: a failed notification must not block Claude
		fmt.Fprintln(stderr, "Warning: Failed to send notification")
	}
	return 0
}

// noopHook sends nothing; Task 3 replaces it with hook.NewHandler().
type noopHook struct{}

func (noopHook) Permission(map[string]any) map[string]any { return nil }
func (noopHook) CancelReplyWait(string)                   {}
func (noopHook) InDesktopApp() bool                       { return false }
func (noopHook) NotifyAndWaitReply(string, map[string]any, time.Duration) string {
	return ""
}
func (noopHook) Process(string, map[string]any) bool { return false }

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, noopHook{}))
}

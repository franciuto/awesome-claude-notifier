// Package hook turns Claude Code hook events into notification texts.
package hook

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Groups shown in the notification title.
const (
	NeedsYou = "Needs you"
	Done     = "Done"
	Paused   = "Paused"
	Error    = "Error"
	Limit    = "Limit"
	Info     = "Info"
)

var groupSlugs = map[string]string{
	NeedsYou: "needs-you",
	Done:     "done",
	Paused:   "paused",
	Error:    "error",
	Limit:    "limit",
	Info:     "info",
}

// SoundSlug is the bare sound name of a group ("" if unknown). The per-OS
// notifiers add the claude-notify- prefix and the file extension.
func SoundSlug(group string) string { return groupSlugs[group] }

type notificationType struct{ group, text string }

// Per Notification "notification_type": group and fallback text.
var notificationTypes = map[string]notificationType{
	"permission_prompt":          {NeedsYou, "Asking for permission"},
	"idle_prompt":                {NeedsYou, "Waiting for your reply"},
	"elicitation_dialog":         {NeedsYou, "An MCP server is asking for input"},
	"elicitation_url_dialog":     {NeedsYou, "An MCP server wants you to open a link"},
	"agent_needs_input":          {NeedsYou, "A background agent is waiting for you"},
	"agent_completed":            {Done, "A background agent finished"},
	"auth_success":               {Info, "Signed in"},
	"quota_auto_resume_fired":    {Limit, "Usage limit reset, work resumed"},
	"quota_auto_resume_stale":    {NeedsYou, "Usage limit reset, press Enter to resume"},
	"quota_auto_resume_disabled": {Limit, "Stopped waiting for the usage limit reset"},
}

// Per StopFailure "error".
var stopFailureTexts = map[string]string{
	"rate_limit":            "Rate limit reached",
	"overloaded":            "Server overloaded",
	"authentication_failed": "Authentication failed",
	"billing_error":         "Billing problem",
	"server_error":          "Server error",
	"max_output_tokens":     "Response too long, cut off",
	"model_not_found":       "Model not found",
}

// SummaryMaxChars is roughly two lines of a macOS banner.
const SummaryMaxChars = 120

var (
	codeBlockRe = regexp.MustCompile("(?s)```.*?```")
	linkRe      = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	bulletRe    = regexp.MustCompile(`(?m)^\s{0,3}(#+|[-*>]|\d+\.)\s+`)
	markupRe    = regexp.MustCompile("[*`|]") // not "_": it is part of file and identifier names
	spacesRe    = regexp.MustCompile(`[\s\p{Z}]+`)
	sentenceEnd = regexp.MustCompile(`[.!?]\s+`)
	toolRe      = regexp.MustCompile(`permission to use (\S+)`)
)

// Summarize strips markdown and keeps the first sentences that fit in max runes.
func Summarize(text string, max int) string {
	text = codeBlockRe.ReplaceAllString(text, " ")
	text = linkRe.ReplaceAllString(text, "$1")
	text = bulletRe.ReplaceAllString(text, "")
	text = markupRe.ReplaceAllString(text, "")
	text = strings.TrimSpace(spacesRe.ReplaceAllString(text, " "))
	if utf8.RuneCountInString(text) <= max {
		return text
	}

	// RE2 has no lookbehind: a sentence ends after . ! or ? followed by whitespace.
	var sentences []string
	start := 0
	for _, loc := range sentenceEnd.FindAllStringIndex(text, -1) {
		sentences = append(sentences, text[start:loc[0]+1])
		start = loc[1]
	}
	sentences = append(sentences, text[start:])

	summary := ""
	for _, sentence := range sentences {
		if summary != "" && utf8.RuneCountInString(summary)+1+utf8.RuneCountInString(sentence) > max {
			break
		}
		summary = strings.TrimSpace(summary + " " + sentence)
	}
	if summary == "" || utf8.RuneCountInString(summary) > max {
		cut := string([]rune(text)[:max-1])
		if i := strings.LastIndex(cut, " "); i >= 0 {
			cut = cut[:i]
		}
		summary = cut + "…"
	}
	return summary
}

// str reads a string field, def when absent or not a string.
func str(data map[string]any, key, def string) string {
	if s, ok := data[key].(string); ok {
		return s
	}
	return def
}

// truthy mirrors Python truthiness for decoded JSON values.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case float64:
		return x != 0
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}

func orElse(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// Describe returns the group for the title and a short summary for the body.
func Describe(event string, data map[string]any) (group, summary string) {
	last := str(data, "last_assistant_message", "")

	switch event {
	case "Stop":
		group = Done
		if truthy(data["background_tasks"]) {
			group = Paused
		}
		return group, orElse(Summarize(last, SummaryMaxChars), "Finished responding")

	case "Notification":
		nt, ok := notificationTypes[str(data, "notification_type", "")]
		if !ok {
			nt = notificationType{Info, "Notification"}
		}
		message := str(data, "message", "")
		if m := toolRe.FindStringSubmatch(message); m != nil {
			return nt.group, "Asking to use " + m[1]
		}
		if message != "" && nt.group == Info {
			return nt.group, Summarize(message, SummaryMaxChars)
		}
		return nt.group, nt.text

	case "StopFailure":
		errName := str(data, "error", "unknown")
		detail := orElse(str(data, "error_details", ""), last)
		text, ok := stopFailureTexts[errName]
		if !ok {
			text = errName
		}
		if detail != "" {
			text += ". " + detail
		}
		return Error, Summarize(text, SummaryMaxChars)

	case "PermissionDenied":
		return Error, Summarize("Auto mode blocked "+str(data, "tool_name", "a tool")+". "+str(data, "reason", ""), SummaryMaxChars)

	case "PermissionRequest":
		return NeedsYou, "Asking to use " + str(data, "tool_name", "a tool")

	case "SubagentStop":
		return Done, orElse(Summarize(last, SummaryMaxChars), "A subagent finished")

	case "TaskCompleted":
		return Done, Summarize(orElse(str(data, "task_subject", ""), "Task completed"), SummaryMaxChars)
	}
	return Info, event
}

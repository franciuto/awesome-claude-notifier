// Package hook turns Claude Code hook events into notification texts.
package hook

import (
	"fmt"
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

// Group per Notification "notification_type"; the texts are in i18n.go.
var notificationGroups = map[string]string{
	"permission_prompt":          NeedsYou,
	"idle_prompt":                NeedsYou,
	"elicitation_dialog":         NeedsYou,
	"elicitation_url_dialog":     NeedsYou,
	"agent_needs_input":          NeedsYou,
	"agent_completed":            Done,
	"auth_success":               Info,
	"quota_auto_resume_fired":    Limit,
	"quota_auto_resume_stale":    NeedsYou,
	"quota_auto_resume_disabled": Limit,
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

// Describe returns the group for the title and a short summary for the body,
// with its fixed sentences in lang.
func Describe(event string, data map[string]any, lang string) (group, summary string) {
	t := textsFor(lang)
	last := str(data, "last_assistant_message", "")
	tool := str(data, "tool_name", t.aTool)

	switch event {
	case "Stop":
		group = Done
		if truthy(data["background_tasks"]) {
			group = Paused
		}
		return group, orElse(Summarize(last, SummaryMaxChars), t.finished)

	case "Notification":
		kind := str(data, "notification_type", "")
		group = orElse(notificationGroups[kind], Info)
		message := str(data, "message", "")
		if m := toolRe.FindStringSubmatch(message); m != nil {
			return group, fmt.Sprintf(t.askingToUse, m[1])
		}
		if text, ok := t.notificationTypes[kind]; ok {
			return group, text
		}
		return group, orElse(Summarize(message, SummaryMaxChars), t.notification)

	case "StopFailure":
		errName := str(data, "error", "unknown")
		detail := orElse(str(data, "error_details", ""), last)
		text := orElse(t.stopFailures[errName], errName)
		if detail != "" {
			text += ". " + detail
		}
		return Error, Summarize(text, SummaryMaxChars)

	case "PermissionDenied":
		return Error, Summarize(fmt.Sprintf(t.autoModeBlocked, tool, str(data, "reason", "")), SummaryMaxChars)

	case "PermissionRequest":
		return NeedsYou, fmt.Sprintf(t.askingToUse, tool)

	case "SubagentStop":
		return Done, orElse(Summarize(last, SummaryMaxChars), t.subagentFinished)

	case "TaskCompleted":
		return Done, Summarize(orElse(str(data, "task_subject", ""), t.taskCompleted), SummaryMaxChars)
	}
	return Info, event
}

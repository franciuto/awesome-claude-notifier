package hook

import (
	"encoding/json"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
)

// texts are the fixed sentences of a notification body in one language. Group
// names in the title and the Allow / Deny / Reply buttons stay in English.
type texts struct {
	notificationTypes map[string]string // per Notification "notification_type"
	stopFailures      map[string]string // per StopFailure "error"
	askingToUse       string            // %s is the tool
	aTool             string
	finished          string
	autoModeBlocked   string // %s is the tool, then the reason
	subagentFinished  string
	taskCompleted     string
	notification      string
	session           string
}

const defaultLanguage = "en"

// To add a language: add its texts here and its common words to stopwords.
var languages = map[string]texts{
	"en": {
		notificationTypes: map[string]string{
			"permission_prompt":          "Asking for permission",
			"idle_prompt":                "Waiting for your reply",
			"elicitation_dialog":         "An MCP server is asking for input",
			"elicitation_url_dialog":     "An MCP server wants you to open a link",
			"agent_needs_input":          "A background agent is waiting for you",
			"agent_completed":            "A background agent finished",
			"auth_success":               "Signed in",
			"quota_auto_resume_fired":    "Usage limit reset, work resumed",
			"quota_auto_resume_stale":    "Usage limit reset, press Enter to resume",
			"quota_auto_resume_disabled": "Stopped waiting for the usage limit reset",
		},
		stopFailures: map[string]string{
			"rate_limit":            "Rate limit reached",
			"overloaded":            "Server overloaded",
			"authentication_failed": "Authentication failed",
			"billing_error":         "Billing problem",
			"server_error":          "Server error",
			"max_output_tokens":     "Response too long, cut off",
			"model_not_found":       "Model not found",
		},
		askingToUse:      "Asking to use %s",
		aTool:            "a tool",
		finished:         "Finished responding",
		autoModeBlocked:  "Auto mode blocked %s. %s",
		subagentFinished: "A subagent finished",
		taskCompleted:    "Task completed",
		notification:     "Notification",
		session:          "Session",
	},
	"it": {
		notificationTypes: map[string]string{
			"permission_prompt":          "Chiede un permesso",
			"idle_prompt":                "Aspetta una tua risposta",
			"elicitation_dialog":         "Un server MCP ti chiede dei dati",
			"elicitation_url_dialog":     "Un server MCP ti chiede di aprire un link",
			"agent_needs_input":          "Un agente in background aspetta una risposta",
			"agent_completed":            "Un agente in background ha finito",
			"auth_success":               "Login completato",
			"quota_auto_resume_fired":    "Limite d'uso resettato, lavoro ripreso",
			"quota_auto_resume_stale":    "Limite d'uso resettato, premi Invio per riprendere",
			"quota_auto_resume_disabled": "Ha smesso di aspettare il reset del limite d'uso",
		},
		stopFailures: map[string]string{
			"rate_limit":            "Limite di richieste raggiunto",
			"overloaded":            "Server sovraccarico",
			"authentication_failed": "Autenticazione fallita",
			"billing_error":         "Problema di fatturazione",
			"server_error":          "Errore del server",
			"max_output_tokens":     "Risposta troppo lunga, interrotta",
			"model_not_found":       "Modello non trovato",
		},
		askingToUse:      "Chiede di usare %s",
		aTool:            "un tool",
		finished:         "Ha finito di rispondere",
		autoModeBlocked:  "Auto mode ha bloccato %s. %s",
		subagentFinished: "Un subagent ha finito",
		taskCompleted:    "Task completato",
		notification:     "Notifica",
		session:          "Sessione",
	},
}

// Frequent short words that give a language away even in a few sentences.
// Scored in this order, so a tie goes to the first.
var stopwords = []struct {
	lang  string
	words []string
}{
	{"en", strings.Fields("the and is are to of in that it for with you this was have not be on as but what " +
		"i we do can if or my your there they will would should")},
	{"it", strings.Fields("il lo la gli le un una di che è e per non con sono questo questa della del nel " +
		"ho hai come anche più ma se mi ti ci si fammi voglio deve devono sia")},
}

// Claude Code's "language" setting, the fallback when the chat gives no clear signal.
var settingLanguages = map[string]string{"english": "en", "italian": "it", "italiano": "it"}

const (
	transcriptTailBytes = 300_000 // enough recent text to classify a long transcript
	sampleChars         = 1500
)

var wordRe = regexp.MustCompile(`[a-zàèéìòù']+`)

func textsFor(lang string) texts {
	if t, ok := languages[lang]; ok {
		return t
	}
	return languages[defaultLanguage]
}

// detectLanguage is the language of the chat: the user's recent messages, then
// Claude's, then the "language" setting in settingsPath, then English.
func detectLanguage(data map[string]any, settingsPath string) string {
	sample := strings.Join(recentChatText(str(data, "transcript_path", "")), " ") +
		" " + str(data, "last_assistant_message", "")
	if lang := classify(sample); lang != "" {
		return lang
	}
	if lang := languageFromSettings(settingsPath); lang != "" {
		return lang
	}
	return defaultLanguage
}

// classify returns "" without a few hits and a clear winner.
func classify(sample string) string {
	words := wordRe.FindAllString(strings.ToLower(sample), -1)
	best, first, second := "", 0, 0
	for _, sw := range stopwords {
		score := 0
		for _, w := range words {
			if slices.Contains(sw.words, w) {
				score++
			}
		}
		if score > first {
			best, first, second = sw.lang, score, first
		} else if score > second {
			second = score
		}
	}
	if first < 3 || float64(first) < float64(second)*1.5 {
		return ""
	}
	return best
}

// recentChatText returns the newest user messages first (they set the chat
// language), then Claude's text.
func recentChatText(path string) []string {
	if path == "" {
		return nil
	}
	f, err := os.Open(expandHome(path))
	if err != nil {
		return nil
	}
	defer f.Close()
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return nil
	}
	if _, err := f.Seek(max(0, size-transcriptTailBytes), io.SeekStart); err != nil {
		return nil
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return nil
	}

	var user, assistant []string
	userChars := 0
	lines := strings.Split(string(b), "\n")
	for _, line := range slices.Backward(lines) {
		if !strings.Contains(line, `"type":"user"`) && !strings.Contains(line, `"type":"assistant"`) {
			continue
		}
		var rec struct {
			Type    string `json:"type"`
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		switch rec.Type {
		case "user":
			var content string
			// Skip harness-injected messages such as <command-name> or <task-notification>
			if json.Unmarshal(rec.Message.Content, &content) == nil && !strings.HasPrefix(strings.TrimSpace(content), "<") {
				user = append(user, content)
				userChars += len(content)
			}
		case "assistant":
			var parts []struct{ Type, Text string }
			if json.Unmarshal(rec.Message.Content, &parts) == nil {
				for _, p := range parts {
					if p.Type == "text" {
						assistant = append(assistant, p.Text)
					}
				}
			}
		}
		if userChars > sampleChars {
			break
		}
	}
	out := append(user, assistant...)
	return out[:min(len(out), 20)]
}

func languageFromSettings(path string) string {
	b, err := os.ReadFile(path)
	if path == "" || err != nil {
		return ""
	}
	var settings struct {
		Language any `json:"language"`
	}
	if json.Unmarshal(b, &settings) != nil {
		return ""
	}
	value, _ := settings.Language.(string)
	value = strings.ToLower(strings.TrimSpace(value))
	if lang, ok := settingLanguages[value]; ok {
		return lang
	}
	if _, ok := languages[value]; ok {
		return value
	}
	return ""
}

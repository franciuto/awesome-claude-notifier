"""Notification texts per language, and detection of the language a session is chatting in

To add a language: add its texts to TEXTS and its common words to STOPWORDS.
Group names in the title (Done, Needs you, ...) and the buttons (Allow / Deny / Reply)
are not translated; they stay in English.
"""

import json
import re
from pathlib import Path
from typing import Any, Dict, List, Optional

DEFAULT_LANGUAGE = "en"

TEXTS: Dict[str, Dict[str, Any]] = {
    "en": {
        "notification_types": {
            "permission_prompt": "Asking for permission",
            "idle_prompt": "Waiting for your reply",
            "elicitation_dialog": "An MCP server is asking for input",
            "elicitation_url_dialog": "An MCP server wants you to open a link",
            "agent_needs_input": "A background agent is waiting for you",
            "agent_completed": "A background agent finished",
            "auth_success": "Signed in",
            "quota_auto_resume_fired": "Usage limit reset, work resumed",
            "quota_auto_resume_stale": "Usage limit reset, press Enter to resume",
            "quota_auto_resume_disabled": "Stopped waiting for the usage limit reset",
        },
        "stop_failures": {
            "rate_limit": "Rate limit reached",
            "overloaded": "Server overloaded",
            "authentication_failed": "Authentication failed",
            "billing_error": "Billing problem",
            "server_error": "Server error",
            "max_output_tokens": "Response too long, cut off",
            "model_not_found": "Model not found",
        },
        "asking_to_use": "Asking to use {tool}",
        "a_tool": "a tool",
        "finished": "Finished responding",
        "auto_mode_blocked": "Auto mode blocked {tool}. {reason}",
        "subagent_finished": "A subagent finished",
        "task_completed": "Task completed",
        "notification": "Notification",
        "session": "Session",
    },
    "it": {
        "notification_types": {
            "permission_prompt": "Chiede un permesso",
            "idle_prompt": "Aspetta una tua risposta",
            "elicitation_dialog": "Un server MCP ti chiede dei dati",
            "elicitation_url_dialog": "Un server MCP ti chiede di aprire un link",
            "agent_needs_input": "Un agente in background aspetta una risposta",
            "agent_completed": "Un agente in background ha finito",
            "auth_success": "Login completato",
            "quota_auto_resume_fired": "Limite d'uso resettato, lavoro ripreso",
            "quota_auto_resume_stale": "Limite d'uso resettato, premi Invio per riprendere",
            "quota_auto_resume_disabled": "Ha smesso di aspettare il reset del limite d'uso",
        },
        "stop_failures": {
            "rate_limit": "Limite di richieste raggiunto",
            "overloaded": "Server sovraccarico",
            "authentication_failed": "Autenticazione fallita",
            "billing_error": "Problema di fatturazione",
            "server_error": "Errore del server",
            "max_output_tokens": "Risposta troppo lunga, interrotta",
            "model_not_found": "Modello non trovato",
        },
        "asking_to_use": "Chiede di usare {tool}",
        "a_tool": "un tool",
        "finished": "Ha finito di rispondere",
        "auto_mode_blocked": "Auto mode ha bloccato {tool}. {reason}",
        "subagent_finished": "Un subagent ha finito",
        "task_completed": "Task completato",
        "notification": "Notifica",
        "session": "Sessione",
    },
}

# Frequent short words that give a language away even in a few sentences
STOPWORDS = {
    "en": set("the and is are to of in that it for with you this was have not be on as but what "
              "i we do can if or my your there they will would should".split()),
    "it": set("il lo la gli le un una di che è e per non con sono questo questa della del nel "
              "ho hai come anche più ma se mi ti ci si fammi voglio deve devono sia".split()),
}

# Claude Code's "language" setting, as a fallback when the chat gives no clear signal
SETTING_LANGUAGES = {"english": "en", "italian": "it", "italiano": "it"}

# Enough recent text to classify, without reading a long transcript in full
TRANSCRIPT_TAIL_BYTES = 300_000
SAMPLE_CHARS = 1500


def detect_language(data: Dict[str, Any]) -> str:
    """Language of the chat: the user's recent messages, then Claude's, then the settings"""
    sample = " ".join(_recent_chat_text(data.get("transcript_path", "")))
    sample += " " + (data.get("last_assistant_message") or "")
    language = _classify(sample)
    return language or _language_from_settings() or DEFAULT_LANGUAGE


def texts(language: str) -> Dict[str, Any]:
    return TEXTS.get(language, TEXTS[DEFAULT_LANGUAGE])


def _classify(sample: str) -> Optional[str]:
    words = re.findall(r"[a-zàèéìòù']+", sample.lower())
    scores = {lang: sum(w in stop for w in words) for lang, stop in STOPWORDS.items()}
    best = max(scores, key=scores.get)
    ranked = sorted(scores.values(), reverse=True)
    # Need a few hits and a clear winner, else let the caller fall back
    if ranked[0] < 3 or (len(ranked) > 1 and ranked[0] < ranked[1] * 1.5):
        return None
    return best


def _recent_chat_text(transcript_path: str) -> List[str]:
    """Newest user messages first (they set the chat language), then Claude's text"""
    if not transcript_path:
        return []
    try:
        with open(Path(transcript_path).expanduser(), "rb") as f:
            f.seek(0, 2)
            f.seek(max(0, f.tell() - TRANSCRIPT_TAIL_BYTES))
            lines = f.read().decode("utf-8", errors="ignore").splitlines()
    except OSError:
        return []

    user, assistant = [], []
    for line in reversed(lines):
        if '"type":"user"' not in line and '"type":"assistant"' not in line:
            continue
        try:
            record = json.loads(line)
        except json.JSONDecodeError:
            continue
        content = (record.get("message") or {}).get("content")
        if record.get("type") == "user" and isinstance(content, str):
            # Skip harness-injected messages such as <command-name> or <task-notification>
            if not content.lstrip().startswith("<"):
                user.append(content)
        elif record.get("type") == "assistant" and isinstance(content, list):
            assistant.extend(part.get("text", "") for part in content if part.get("type") == "text")
        if sum(map(len, user)) > SAMPLE_CHARS:
            break
    return (user + assistant)[:20]


def _language_from_settings() -> Optional[str]:
    try:
        settings = json.loads((Path.home() / ".claude/settings.json").read_text())
    except (OSError, ValueError):
        return None
    value = str(settings.get("language", "")).strip().lower()
    return SETTING_LANGUAGES.get(value, value if value in TEXTS else None)

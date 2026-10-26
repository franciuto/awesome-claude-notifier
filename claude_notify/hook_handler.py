"""Hook handler for Claude Code integration

Notification layout:
    title: "<session name> · <group>"   e.g. "Claude Notify configuration · Done"
    body:  what happened, at most ~2 lines
Fixed texts follow the language the session is chatting in (see i18n.py).
"""

import json
import os
import re
import signal
import subprocess
import sys
from pathlib import Path
from typing import Dict, Any, Optional, Tuple
from .i18n import detect_language, texts
from .notifier import ClaudeNotifier


# Groups; the title always shows them in English, whatever the chat language
NEEDS_YOU = "needs-you"
DONE = "done"
PAUSED = "paused"
ERROR = "error"
LIMIT = "limit"
INFO = "info"

GROUP_LABELS = {
    NEEDS_YOU: "Needs you", DONE: "Done", PAUSED: "Paused",
    ERROR: "Error", LIMIT: "Limit", INFO: "Info",
}


def sound_for(group: str) -> Optional[str]:
    """The group's sound if installed, else None (macOS default sound).

    One sound per group, generated with ElevenLabs into ~/Library/Sounds (see sounds/generate.sh).
    """
    name = f"claude-notify-{group}.aiff"
    return name if (Path.home() / "Library/Sounds" / name).exists() else None

# Group per Notification type (hook input "notification_type"); texts are in i18n.py
NOTIFICATION_GROUPS = {
    "permission_prompt": NEEDS_YOU,
    "idle_prompt": NEEDS_YOU,
    "elicitation_dialog": NEEDS_YOU,
    "elicitation_url_dialog": NEEDS_YOU,
    "agent_needs_input": NEEDS_YOU,
    "agent_completed": DONE,
    "auth_success": INFO,
    "quota_auto_resume_fired": LIMIT,
    "quota_auto_resume_stale": NEEDS_YOU,
    "quota_auto_resume_disabled": LIMIT,
}

# Roughly two lines of a macOS banner
SUMMARY_MAX_CHARS = 120

DESKTOP_SESSIONS_DIR = Path.home() / "Library/Application Support/Claude/claude-code-sessions"

# One pending Reply per session: a newer Stop or a prompt typed in the terminal replaces it
WAITERS_DIR = Path.home() / "Library/Caches/claude-notify/waiters"

# How long a permission notification waits before the terminal dialog takes over
PERMISSION_WAIT = 20

# Used when the terminal app can't be told from __CFBundleIdentifier
TERMINAL_BUNDLE_IDS = {
    "dev.warp.Warp-Stable", "com.apple.Terminal", "com.googlecode.iterm2",
    "com.mitchellh.ghostty", "net.kovidgoyal.kitty", "com.github.wez.wezterm",
    "io.alacritty", "com.microsoft.VSCode", "com.todesktop.230313mzl4w4u92",
}


def summarize(text: str, max_chars: int = SUMMARY_MAX_CHARS) -> str:
    """Strip markdown and keep the first sentences that fit in about two lines"""
    text = re.sub(r"```.*?```", " ", text, flags=re.S)           # code blocks
    text = re.sub(r"\[([^\]]+)\]\([^)]*\)", r"\1", text)          # links -> label
    text = re.sub(r"^\s{0,3}(#+|[-*>]|\d+\.)\s+", "", text, flags=re.M)  # headings, bullets
    text = re.sub(r"[*_`|]", "", text)
    text = re.sub(r"\s+", " ", text).strip()
    if len(text) <= max_chars:
        return text

    summary = ""
    for sentence in re.split(r"(?<=[.!?])\s+", text):
        if summary and len(summary) + 1 + len(sentence) > max_chars:
            break
        summary = f"{summary} {sentence}".strip()
    if len(summary) > max_chars or not summary:
        summary = text[: max_chars - 1].rsplit(" ", 1)[0] + "…"
    return summary


class HookHandler:
    """Process Claude Code hook events and send appropriate notifications"""

    def __init__(self, notifier: Optional[ClaudeNotifier] = None):
        self.notifier = notifier or ClaudeNotifier()

    def get_session_name(self, data: Dict[str, Any]) -> str:
        """
        Name of the session as shown in Claude's sidebar / session list.

        Order: title set by hand or by the desktop app (custom-title), then the
        auto-generated one (ai-title), then the desktop app metadata, then the folder name.
        """
        title = self._title_from_transcript(data.get("transcript_path", ""))
        if not title:
            title = self._title_from_desktop_app(data.get("session_id", ""))
        if not title:
            cwd = data.get("cwd") or ""
            title = Path(cwd).name if cwd else ""
        return title or ""

    def _title_from_transcript(self, transcript_path: str) -> Optional[str]:
        """Read the latest custom-title / ai-title record from the session transcript"""
        if not transcript_path:
            return None
        custom, auto = None, None
        try:
            with open(Path(transcript_path).expanduser(), "r", errors="ignore") as f:
                for line in f:
                    # Cheap filter before parsing: title records are rare
                    if '-title"' not in line:
                        continue
                    try:
                        record = json.loads(line)
                    except json.JSONDecodeError:
                        continue
                    if record.get("type") == "custom-title" and record.get("customTitle"):
                        custom = record["customTitle"]
                    elif record.get("type") == "ai-title" and record.get("aiTitle"):
                        auto = record["aiTitle"]
        except OSError:
            return None
        return custom or auto

    def _title_from_desktop_app(self, session_id: str) -> Optional[str]:
        """Look up the title the Claude desktop app keeps for this CLI session id"""
        if not session_id or not DESKTOP_SESSIONS_DIR.is_dir():
            return None
        for meta in DESKTOP_SESSIONS_DIR.glob("*/*/local_*.json"):
            try:
                with open(meta, "r") as f:
                    content = f.read()
                if session_id not in content:
                    continue
                record = json.loads(content)
                if record.get("cliSessionId") == session_id and record.get("title"):
                    return record["title"]
            except (OSError, json.JSONDecodeError):
                continue
        return None

    def describe_event(self, event_type: str, data: Dict[str, Any], language: str) -> Tuple[str, str]:
        """Group for the title and a short summary for the body, in the given language"""
        t = texts(language)
        last_message = data.get("last_assistant_message") or ""
        tool_name = data.get("tool_name") or t["a_tool"]

        if event_type == "Stop":
            group = PAUSED if data.get("background_tasks") else DONE
            return group, summarize(last_message) or t["finished"]

        if event_type == "Notification":
            kind = data.get("notification_type", "")
            group = NOTIFICATION_GROUPS.get(kind, INFO)
            message = data.get("message") or ""
            tool = re.search(r"permission to use (\S+)", message)
            if tool:
                return group, t["asking_to_use"].format(tool=tool.group(1))
            if kind in t["notification_types"]:
                return group, t["notification_types"][kind]
            return group, summarize(message) or t["notification"]

        if event_type == "StopFailure":
            error = data.get("error", "unknown")
            detail = data.get("error_details") or last_message
            text = t["stop_failures"].get(error, error)
            return ERROR, summarize(f"{text}. {detail}" if detail else text)

        if event_type == "PermissionDenied":
            return ERROR, summarize(t["auto_mode_blocked"].format(tool=tool_name, reason=data.get("reason", "")))

        if event_type == "PermissionRequest":
            return NEEDS_YOU, t["asking_to_use"].format(tool=tool_name)

        if event_type == "SubagentStop":
            return DONE, summarize(last_message) or t["subagent_finished"]

        if event_type == "TaskCompleted":
            return DONE, summarize(data.get("task_subject") or t["task_completed"])

        return INFO, event_type

    def title(self, data: Dict[str, Any], group: str, language: str) -> str:
        """'<session name> · <group>'; only the fallback session word follows the chat language"""
        return f"{self.get_session_name(data) or texts(language)['session']} · {GROUP_LABELS[group]}"

    def process_hook_event(self, event_type: str, data: Dict[str, Any]) -> bool:
        """
        Process a hook event and send appropriate notification

        Args:
            event_type: Type of hook event (Stop, Notification, StopFailure, ...)
            data: JSON data from the hook

        Returns:
            bool: True if notification was sent successfully
        """
        language = detect_language(data)
        group, summary = self.describe_event(event_type, data, language)
        return self.notifier.send_notification(
            title=self.title(data, group, language),
            message=summary,
            urgency="critical" if group == ERROR else "normal",
            sound_name=sound_for(group),
        )

    # --- Interactive notifications (CLI only) ---

    @staticmethod
    def in_desktop_app() -> bool:
        """The Claude desktop app has its own UI for permissions and replies"""
        return os.environ.get("CLAUDE_CODE_ENTRYPOINT") == "claude-desktop"

    @staticmethod
    def terminal_bundle_id() -> Optional[str]:
        """Bundle id of the terminal app hosting this session, when macOS tells us"""
        return os.environ.get("__CFBundleIdentifier")

    def user_at_terminal(self) -> bool:
        """True when the terminal is the frontmost app, so the normal dialog is quicker"""
        try:
            front = subprocess.run(["lsappinfo", "front"], capture_output=True, text=True, timeout=2).stdout.strip()
            info = subprocess.run(["lsappinfo", "info", "-only", "bundleid", front],
                                  capture_output=True, text=True, timeout=2).stdout
        except (OSError, subprocess.TimeoutExpired):
            return False
        match = re.search(r'"?(?:bundleID|CFBundleIdentifier)"?\s*=\s*"([^"]+)"', info)
        if not match:
            return False
        own = self.terminal_bundle_id()
        return match.group(1) == own if own else match.group(1) in TERMINAL_BUNDLE_IDS

    def bring_terminal_forward(self) -> None:
        """Clicking the notification itself takes you to the terminal"""
        bundle = self.terminal_bundle_id()
        if bundle:
            subprocess.run(["open", "-b", bundle], capture_output=True)

    def handle_permission_request(self, data: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        """
        Ask Allow / Deny from a notification.

        Returns the hook output to print, or None to leave the decision to the terminal dialog.
        """
        if self.in_desktop_app() or self.user_at_terminal():
            return None

        language = detect_language(data)
        tool = data.get("tool_name", "tool")
        tool_input = data.get("tool_input") or {}
        detail = (tool_input.get("command") or tool_input.get("file_path")
                  or tool_input.get("url") or tool_input.get("description") or "")
        answer = self.notifier.ask(
            title=self.title(data, NEEDS_YOU, language),
            message=summarize(f"{tool}: {detail}" if detail else texts(language)["asking_to_use"].format(tool=tool)),
            actions="permission",
            wait=PERMISSION_WAIT,
            sound_name=sound_for(NEEDS_YOU),
        )
        action = (answer or {}).get("action")
        if action == "open":
            self.bring_terminal_forward()
        if action not in ("allow", "deny"):
            return None

        decision: Dict[str, Any] = {"behavior": action}
        if action == "deny":
            decision["message"] = "The user denied permission from the notification."
        return {"hookSpecificOutput": {"hookEventName": "PermissionRequest", "decision": decision}}

    def notify_and_wait_reply(self, event_type: str, data: Dict[str, Any], wait: int) -> Optional[str]:
        """Show the notification with a Reply field; return the text the user sent, if any"""
        session_id = data.get("session_id", "unknown")
        self.cancel_reply_wait(session_id)
        WAITERS_DIR.mkdir(parents=True, exist_ok=True)
        pid_file = WAITERS_DIR / f"{session_id}.pid"
        pid_file.write_text(str(os.getpid()))

        try:
            language = detect_language(data)
            group, summary = self.describe_event(event_type, data, language)
            answer = self.notifier.ask(
                title=self.title(data, group, language),
                message=summary,
                actions="reply",
                wait=wait,
                sound_name=sound_for(group),
            )
        finally:
            if pid_file.exists() and pid_file.read_text().strip() == str(os.getpid()):
                pid_file.unlink()

        if answer is None:
            # Helper unavailable: fall back to a plain notification
            self.process_hook_event(event_type, data)
            return None
        if answer.get("action") == "open":
            self.bring_terminal_forward()
        if answer.get("action") == "reply" and answer.get("text", "").strip():
            return answer["text"].strip()
        return None

    @staticmethod
    def cancel_reply_wait(session_id: str) -> None:
        """Withdraw a pending Reply notification for this session, if any"""
        pid_file = WAITERS_DIR / f"{session_id}.pid"
        try:
            pid = int(pid_file.read_text().strip())
            # A stale pid may now belong to another process: only signal our own waiter
            command = subprocess.run(["ps", "-p", str(pid), "-o", "command="],
                                     capture_output=True, text=True).stdout
            if "claude-notify" in command:
                os.kill(pid, signal.SIGTERM)
        except (OSError, ValueError):
            pass
        try:
            pid_file.unlink()
        except OSError:
            pass

    def read_stdin_json(self) -> Optional[Dict[str, Any]]:
        """Read JSON data from stdin"""
        try:
            # Read all input from stdin
            input_data = sys.stdin.read()
            if not input_data:
                return None

            # Parse JSON
            return json.loads(input_data)
        except json.JSONDecodeError as e:
            print(f"Error parsing JSON: {e}", file=sys.stderr)
            return None
        except Exception as e:
            print(f"Error reading stdin: {e}", file=sys.stderr)
            return None

    def determine_event_type(self, data: Dict[str, Any]) -> Optional[str]:
        """Event type from the hook data; Claude Code sends it as hook_event_name"""
        if data.get("hook_event_name"):
            return data["hook_event_name"]
        if "event_type" in data:
            return data["event_type"]

        # Older payloads: infer from the data structure
        if "tool_name" in data:
            return "PostToolUse" if "tool_response" in data else "PreToolUse"
        if "notification_type" in data:
            return "Notification"
        if "session_id" in data:
            return "Stop"
        return None

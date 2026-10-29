"""Contracts of `claude-notify hook` (see CLAUDE.md), with a fake notifier: nothing is posted"""

import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from claude_notify import hook_handler
from claude_notify.cli import cli
from claude_notify.hook_handler import summarize
from claude_notify.notifier import ClaudeNotifier


class FakeNotifier:
    def __init__(self, answer=None):
        self.answer = answer
        self.sent = []
        self.asked = []

    def send_notification(self, **kwargs):
        self.sent.append(kwargs)
        return True

    def ask(self, **kwargs):
        self.asked.append(kwargs)
        return self.answer


class HookTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        for target, value in [
            ("claude_notify.hook_handler.WAITERS_DIR", Path(self.tmp.name)),
            ("claude_notify.hook_handler.HookHandler.user_at_terminal", lambda self: False),
            ("claude_notify.hook_handler.HookHandler.get_session_name", lambda self, data: "S"),
            ("claude_notify.hook_handler.HookHandler.bring_terminal_forward", lambda self: None),
        ]:
            patcher = mock.patch(target, value)
            patcher.start()
            self.addCleanup(patcher.stop)

        environ = mock.patch.dict(os.environ)
        environ.start()
        self.addCleanup(environ.stop)
        os.environ.pop("CLAUDE_CODE_ENTRYPOINT", None)  # the desktop app gets no buttons

    def run_hook(self, answer, data, *args):
        notifier = FakeNotifier(answer)
        with mock.patch.object(hook_handler, "ClaudeNotifier", lambda: notifier):
            result = CliRunner().invoke(cli, ["hook", *args], input=json.dumps(data))
        return result, notifier

    def test_permission_answered_prints_decision(self):
        result, _ = self.run_hook({"action": "allow"}, {"hook_event_name": "PermissionRequest", "tool_name": "Bash"})
        self.assertEqual(result.exit_code, 0)
        decision = json.loads(result.stdout)["hookSpecificOutput"]["decision"]
        self.assertEqual(decision, {"behavior": "allow"})

    def test_permission_unanswered_prints_nothing(self):
        for answer in ({"action": "timeout"}, {"action": "open"}, None):
            result, _ = self.run_hook(answer, {"hook_event_name": "PermissionRequest", "tool_name": "Bash"})
            self.assertEqual((result.exit_code, result.stdout), (0, ""), answer)

    def test_reply_exits_2_with_text(self):
        data = {"hook_event_name": "Stop", "session_id": "t", "last_assistant_message": "Done."}
        result, _ = self.run_hook({"action": "reply", "text": " go on "}, data, "--reply-wait", "60")
        self.assertEqual(result.exit_code, 2)
        self.assertIn("go on", result.output)

    def test_no_reply_exits_0(self):
        data = {"hook_event_name": "Stop", "session_id": "t"}
        for answer in ({"action": "timeout"}, {"action": "reply", "text": "  "}):
            result, _ = self.run_hook(answer, data, "--reply-wait", "60")
            self.assertEqual(result.exit_code, 0, answer)

    def test_reply_helper_failed_falls_back_to_plain(self):
        data = {"hook_event_name": "Stop", "session_id": "t", "last_assistant_message": "Done."}
        result, notifier = self.run_hook(None, data, "--reply-wait", "60")
        self.assertEqual(result.exit_code, 0)
        self.assertEqual(len(notifier.sent), 1)

    def test_usage_error_is_not_exit_2(self):
        result = subprocess.run([sys.executable, "-m", "claude_notify.cli", "hook", "--bogus"],
                                input="{}", capture_output=True, text=True)
        self.assertEqual(result.returncode, 1, result.stderr)


class ReadAnswerTest(unittest.TestCase):
    def read(self, content, returncode=0):
        with tempfile.TemporaryDirectory() as tmp:
            answer_file = Path(tmp) / "a.json"
            if content is not None:
                answer_file.write_text(content, encoding="utf-8")
            return ClaudeNotifier._read_answer(answer_file, mock.Mock(returncode=returncode))

    def test_answers(self):
        self.assertEqual(self.read('{"action":"allow"}'), {"action": "allow"})
        self.assertIsNone(self.read('{"action":"error"}'))
        self.assertEqual(self.read(None), {"action": "timeout"})
        self.assertIsNone(self.read(None, returncode=1))


class SummarizeTest(unittest.TestCase):
    def test_keeps_underscores_strips_markdown(self):
        self.assertEqual(summarize("**Fixed** `hook_handler.py`"), "Fixed hook_handler.py")

    def test_cuts_long_text(self):
        summary = summarize("word " * 100)
        self.assertLessEqual(len(summary), 120)
        self.assertTrue(summary.endswith("…"))


if __name__ == "__main__":
    unittest.main()

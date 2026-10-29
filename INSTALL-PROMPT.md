# Install prompt

Paste everything below the line into Claude Code (or any agent that can run commands and edit files). It is the same on macOS, Windows and Linux.

---

Install the Claude Code plugin `notifier` from the marketplace `awesome-claude-notifier` (GitHub: franciuto/awesome-claude-notifier). It sends desktop notifications for Claude Code events. Do these steps in order, and tell me what you did after each one.

1. Install the plugin. In Claude Code run `/plugin marketplace add franciuto/awesome-claude-notifier`, then `/plugin install notifier@awesome-claude-notifier`. If you can only use a shell, run `claude plugin marketplace add franciuto/awesome-claude-notifier` and then `claude plugin install notifier@awesome-claude-notifier`.

2. In my user settings file (`~/.claude/settings.json`, on Windows `%USERPROFILE%\.claude\settings.json`) set `"preferredNotifChannel": "notifications_disabled"`, so Claude Code's own notifications don't repeat ours. Keep every other setting as it is. If the file doesn't exist, create it.

3. Look in the same file for old hooks whose command contains `claude-notify hook`. They come from the earlier pip version and would send every notification twice. Show them to me and remove them once I agree. If `pip show claude-notify` finds the old package, offer to run `pip uninstall claude-notify` too.

4. Check what my system needs:
   - macOS: tell me to allow notifications for "Claude Notify" when macOS asks, and to set its style to Alerts in System Settings, Notifications, Claude Notify. Alerts keep the Allow, Deny and Reply buttons on screen.
   - Windows: check that Git for Windows is installed (`git --version`). The hooks run under Git Bash and don't work without it. If it is missing, tell me to install it from https://git-scm.com/download/win.
   - Linux: check that `notify-send` exists (`command -v notify-send`). If not, tell me how to install it, for example `sudo apt install libnotify-bin`. Linux gets plain notifications only, without buttons.

5. Send a test notification. Find the plugin folder under `~/.claude/plugins/cache/awesome-claude-notifier/notifier/` (it holds one folder per version; use the newest) and run:

   ```bash
   echo '{"hook_event_name":"Stop","session_id":"t","cwd":"/tmp/demo","last_assistant_message":"Test."}' | "<plugin root>/bin/claude-notify" hook
   ```

   Replace `<plugin root>` with that folder. Ask me whether a notification with a sound appeared. If not, show me the command's output and help me find out why.

Finally, tell me to start a new Claude Code session, because hooks are read when a session starts.

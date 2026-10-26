# awesome-claude-notifier

Native macOS notifications for [Claude Code](https://code.claude.com) hooks. Each notification tells you which session it is about and what happened, plays a sound picked for that kind of event, and lets you act on it without going back to the terminal: approve a tool, or reply to Claude from the notification itself.

This is a fork of [jamez01/claude-notify](https://github.com/jamez01/claude-notify) (MIT). The cross-platform `send`, `watch` and `config` commands from the original still work; the changes below are about the `hook` command on macOS.

## What a notification looks like

```
Refactor auth module · Done
I moved the token check into the middleware and added two tests for expired tokens.
```

- **Title**: the session name as Claude shows it (a name you gave with `/rename` or in the desktop app sidebar, else the auto-generated title), then the event group.
- **Body**: about two lines on what happened. For `Stop` it is the start of Claude's last message, with Markdown stripped.

## Event groups

Every hook event falls into one group. The group sets the title suffix and the sound.

| Group | Fires on | Sound file |
|---|---|---|
| Needs you | `PermissionRequest`, `permission_prompt`, `idle_prompt`, MCP elicitation, a background agent waiting, usage limit reset waiting for Enter | `claude-notify-needs-you.aiff` |
| Done | `Stop`, `SubagentStop`, `TaskCompleted`, `agent_completed` | `claude-notify-done.aiff` |
| Paused | `Stop` while background tasks are still running | `claude-notify-paused.aiff` |
| Error | `StopFailure` (rate limit, overload, auth, ...), `PermissionDenied` from auto mode | `claude-notify-error.aiff` |
| Limit | usage limit auto-resume fired or gave up | `claude-notify-limit.aiff` |
| Info | sign-in and anything else | `claude-notify-info.aiff` |

## Interactive notifications (CLI sessions)

- **Allow / Deny**: on `PermissionRequest` the notification has Allow and Deny buttons. Your choice goes back to Claude Code as the hook's `decision`. If you don't answer within 20 seconds the notification is withdrawn and the normal terminal dialog appears. When your terminal is the frontmost app the notification is skipped, so the dialog shows right away.
- **Reply**: on `Stop` the notification has a Reply field. The hook runs in the background with `asyncRewake`, so the terminal stays free. If you send a reply, the hook exits with code 2 and Claude Code wakes the session with your text. Typing a prompt in the terminal withdraws the pending Reply.

Claude receives a reply as a system reminder that says it came from you, not as a regular user turn.

In the Claude desktop app (`CLAUDE_CODE_ENTRYPOINT=claude-desktop`) the hooks send plain notifications only, since the app has its own UI for permissions and replies.

Claude Code [channels](https://code.claude.com/docs/en/channels) are the official way to relay permission prompts and messages, and they don't delay the terminal dialog. They need a Pro/Max account or an organization that enabled them; this project works with hooks alone.

## How it works on macOS

Notifications come from a small AppKit helper app, `Claude Notify.app`, built from `macos-helper/`. Using an app bundle is what gives the notification its own icon (instead of Script Editor's, as with `osascript`) and lets it carry buttons and a text field.

- Plain notifications run the helper's binary directly.
- Interactive ones launch it through LaunchServices (`open -n -W -a`), because macOS only routes a click or a typed reply to an instance it registered as the app. The helper writes the answer to `~/Library/Caches/claude-notify/responses/<id>.json`, where the hook picks it up.

If the helper is missing, plain notifications fall back to `osascript` (with the text escaped for AppleScript).

## Setup

Requirements: macOS 13 or later, Xcode command line tools (`swiftc`), Python 3.8+.

1. Install the Python package:

   ```bash
   pip install .
   ```

2. Build the helper app into `~/Applications`:

   ```bash
   ./macos-helper/build.sh
   ```

   The first notification asks for permission. To keep notifications with buttons on screen until you answer, set the style of **Claude Notify** to *Alerts* in System Settings → Notifications.

3. Install the sounds into `~/Library/Sounds`:

   ```bash
   ./sounds/install.sh
   ```

   Open `sounds/player.html` to listen to them.

4. Add the hooks from [`examples/claude-settings.json`](examples/claude-settings.json) to `~/.claude/settings.json`. Use the full path to `claude-notify` if it isn't on the `PATH` that Claude Code sees, for example `~/.venv/bin/claude-notify`.

   The example also sets `preferredNotifChannel` to `notifications_disabled`, so Claude Code's built-in terminal notifications don't duplicate these.

## Customizing

- **Sounds**: `sounds/generate.sh [group ...]` regenerates sounds with the [ElevenLabs](https://elevenlabs.io) sound effects API (needs an authenticated `elevenlabs` CLI) and installs them. Edit the prompts in the script to change the style.
- **Icon**: `python assets/icon/make_app_icon.py <square-image> assets/icon/app-icon.png` fits any square image into the macOS icon grid; then rerun `build.sh`. If macOS keeps showing the old icon, its icon cache is keyed by bundle id: change `CFBundleIdentifier` in `macos-helper/Info.plist` and rebuild.
- **Texts**: groups, notification texts and the reply wording live at the top of `claude_notify/hook_handler.py`.

## Hook command reference

```
claude-notify hook [--event-type EVENT] [--reply-wait SECONDS]
```

- `--event-type`: the hook event. Defaults to `hook_event_name` from the JSON on stdin.
- `--reply-wait`: for `Stop` and `Notification`, offer Reply and wait up to this many seconds. Use it only on a hook with `asyncRewake: true`.

`PermissionRequest` prints the decision JSON when you answer; `UserPromptSubmit` only withdraws a pending Reply and shows nothing.

## License

MIT, see [LICENSE](LICENSE). Original work by jamez01.

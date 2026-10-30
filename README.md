<p align="center">
  <img src="assets/banner.png" alt="Claude Code Notification Helper" width="720">
</p>

<p align="center">
  <a href="#onboard-your-agent"><img src="https://img.shields.io/badge/Onboard_your_agent-copy_the_prompt-D97757?style=for-the-badge&logo=claude&logoColor=white" alt="Onboard your agent: copy the install prompt"></a>
  <a href="https://github.com/franciuto/awesome-claude-notifier/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/franciuto/awesome-claude-notifier/ci.yml?style=for-the-badge&label=CI" alt="CI status"></a>
</p>

<p align="center">
  <a href="#macos"><img src="https://img.shields.io/badge/macOS-13%2B-000000?style=for-the-badge&logo=apple&logoColor=white" alt="macOS 13 or later"></a>
  <a href="#windows"><img src="https://img.shields.io/badge/Windows-10%20%7C%2011%20%C2%B7%20x64%20%7C%20ARM64-0078D4?style=for-the-badge&logo=data:image/svg%2bxml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCI+PHBhdGggZmlsbD0id2hpdGUiIGQ9Ik0wIDBoMTEuNHYxMS40SDB6TTEyLjYgMEgyNHYxMS40SDEyLjZ6TTAgMTIuNmgxMS40VjI0SDB6TTEyLjYgMTIuNkgyNFYyNEgxMi42eiIvPjwvc3ZnPg==" alt="Windows 10 or 11, x64 or ARM64"></a>
  <a href="#linux"><img src="https://img.shields.io/badge/Linux-notify--send-FCC624?style=for-the-badge&logo=linux&logoColor=black" alt="Linux with notify-send"></a>
</p>

# awesome-claude-notifier

Desktop notifications for Claude Code: **which session**, **what happened**, and **a sound** per kind of event. Answer permission prompts and reply to Claude **from the notification itself**.

| | macOS | Windows | Linux |
|---|:---:|:---:|:---:|
| Notification with session and summary | ✅ | ✅ | ✅ |
| A sound per group | ✅ | ✅ | — |
| **Allow** / **Deny** from the notification | ✅ | ✅ | — |
| **Reply** from the notification | ✅ | ✅ | — |

## What a notification looks like

<table align="center">
  <tr>
    <th>macOS</th>
    <th>Windows</th>
  </tr>
  <tr>
    <td valign="top"><img src="assets/demo/macos-stack.png" alt="macOS notifications from Claude Notify, one per group: Done with a Reply field, Info, Limit, Error, Paused, Needs you with Allow and Deny in its Options menu, and Done" width="360"></td>
    <td valign="top"><img src="assets/demo/windows-stack.png" alt="Windows toasts from Claude Notify, one per group: Done with a Reply field, Info, Limit, Error, Paused, Needs you with Allow and Deny, and Done" width="360"></td>
  </tr>
</table>

Title: session name and what happened. Body: the start of Claude's answer, in the language you chat in (English or Italian).

| Group | When |
|---|---|
| **Needs you** | Claude wants permission or an answer |
| **Done** | Claude finished |
| **Paused** | Finished, but something still runs in the background |
| **Error** | The turn failed, or auto mode blocked an action |
| **Limit** | Usage limit hit or reset |
| **Info** | Anything else |

🔊 Open `sounds/player.html` to hear the sounds.

## Answering from the notification

- **Allow / Deny** when Claude asks to use a tool (macOS: in the **Options** menu). No answer in **20 seconds**, or the terminal already in front: the usual terminal prompt.
- **Reply** when Claude finishes: what you type goes straight to that session.

> [!NOTE]
> macOS and Windows terminal sessions only. The Claude desktop app and Linux get plain notifications.

## Install

### By hand

Inside Claude Code:

```
/plugin marketplace add franciuto/awesome-claude-notifier
/plugin install notifier@awesome-claude-notifier
```

Then set `"preferredNotifChannel": "notifications_disabled"` in `~/.claude/settings.json` (Windows: `%USERPROFILE%\.claude\settings.json`), so Claude Code's own notifications don't repeat these, and **start a new session**.

### Onboard your agent

Copy/paste into Claude Code:

```text
Install the Claude Code plugin notifier@awesome-claude-notifier from https://github.com/franciuto/awesome-claude-notifier following its README: set "preferredNotifChannel": "notifications_disabled" in my ~/.claude/settings.json, check what my OS needs, send a test notification, then tell me to start a new session.
```

## What each system needs

### macOS

macOS 13+. Allow notifications for "Claude Notify" when asked, and set its style to **Alerts** (System Settings → Notifications) so buttons stay on screen.

### Windows

Windows 10 or 11, x64 or ARM64, plus [Git for Windows](https://git-scm.com/download/win): the hooks run under Git Bash.

### Linux

`notify-send` (`libnotify-bin` on Debian/Ubuntu, `libnotify` on Fedora/Arch). Plain notifications only: no buttons, no sounds.

## Update and uninstall

```
claude plugin marketplace update awesome-claude-notifier
claude plugin update notifier@awesome-claude-notifier
```

```
claude plugin uninstall notifier@awesome-claude-notifier
claude plugin marketplace remove awesome-claude-notifier
```

Start a new session after either. Leftover files you can delete: on macOS `~/Applications/Claude Notify.app`, `~/Library/Sounds/claude-notify-*.aiff`, `~/Library/Caches/claude-notify`, `~/Library/Logs/claude-notify-helper.log`; on Windows `%LOCALAPPDATA%\claude-notify` and `HKCU\Software\Classes\AppUserModelId\ClaudeNotify.Hook`.

## When something is off

| What you see | Fix |
|---|---|
| **No notification** | Start a new session. Check notifications are allowed for "Claude Notify" and Focus / Do Not Disturb is off. |
| **Claude Code's own notification too** | Set `"preferredNotifChannel": "notifications_disabled"`. |
| **Buttons vanish after a few seconds (macOS)** | Set the Claude Notify style to Alerts. |
| **Old icon, or macOS never asks for permission** | See "Things that bit us" in [docs/how-it-works.md](docs/how-it-works.md). |
| **Nothing on Windows** | Install [Git for Windows](https://git-scm.com/download/win). |
| **No sound** | macOS: the next notification restores `~/Library/Sounds/claude-notify-*.aiff`. Windows: check the "Windows PowerShell" volume. |

Test by hand (newest folder under `~/.claude/plugins/cache/awesome-claude-notifier/notifier/`):

```bash
echo '{"hook_event_name":"Stop","session_id":"t","cwd":"/tmp/demo","last_assistant_message":"Test."}' \
  | ~/.claude/plugins/cache/awesome-claude-notifier/notifier/<version>/bin/claude-notify hook
```

## Make it yours

- **Sounds:** edit the description in `sounds/generate.sh`, run `./sounds/generate.sh <group>` (needs the [ElevenLabs](https://elevenlabs.io) CLI and `ffmpeg`).
- **Icon:** `python3 assets/icon/make_app_icon.py your-image.png assets/icon/app-icon.png`; on Windows replace `windows/icon.png`.
- **Languages:** add a block of texts and common words in `internal/hook/i18n.go`.

After any change, run the Release workflow (Actions → Release) with a new version: it builds the binaries and the app and commits them with the bump. `make dist` builds them locally for testing. How it works: [docs/how-it-works.md](docs/how-it-works.md).

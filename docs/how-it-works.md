# How it works

There are three parts: a Python command that Claude Code runs as a hook, a tiny macOS app that shows the notification, and a folder where the two pass answers to each other.

## 1. Claude Code calls the hook

Each hook in `~/.claude/settings.json` runs `claude-notify hook` and pipes in a JSON description of the event. The command reads it and works out three things.

**The session name.** It reads the session transcript and takes the last title record: a name you set by hand (`custom-title`), or else the one Claude generated (`ai-title`). If there is none yet, it asks the desktop app's session list, and as a last resort it uses the folder name.

**The group and the text.** `describe_event` in `claude_notify/hook_handler.py` maps the event to a group and a short sentence:

| Event | Group | Text |
|---|---|---|
| `Stop` | Done, or Paused if background tasks are running | Start of Claude's last message |
| `PermissionRequest`, `permission_prompt` | Needs you | The tool and its command or file |
| `idle_prompt`, MCP elicitation, `agent_needs_input` | Needs you | Fixed sentence |
| `StopFailure` | Error | The error type and details |
| `PermissionDenied` | Error | The blocked tool and the reason |
| `quota_auto_resume_*` | Limit, or Needs you when it waits for Enter | Fixed sentence |
| `SubagentStop`, `TaskCompleted`, `agent_completed` | Done | Summary or task name |
| anything else | Info | Claude's message or the event name |

The text is cut to about 120 characters, which is two lines in a macOS banner.

**The sound.** Each group plays `~/Library/Sounds/claude-notify-<group>.aiff`. If the file is missing, macOS plays its default sound.

## 2. The app shows the notification

`Claude Notify.app` is built from `macos-helper/main.swift`. It exists for two reasons: a notification takes the icon of the app that sends it, and only an app can put buttons and a text field in one. Sending notifications with `osascript` gives you Script Editor's icon and no buttons. It is still the fallback when the app isn't installed.

For a plain notification the hook runs the app's binary directly and returns at once.

For a notification with buttons the hook starts the app with `open -n -W -a`. That detail matters: macOS delivers a click or a typed reply only to an app instance it launched itself. When the binary was run directly, a reply left the notification stuck and never arrived.

## 3. The answer comes back

When you click a button or send a reply, the app writes the answer to `~/Library/Caches/claude-notify/responses/<id>.json` and quits. The hook is waiting on that file:

- **Allow / Deny.** The hook prints a `PermissionRequest` decision, and Claude Code applies it. With no answer after 20 seconds it prints nothing, so Claude Code shows its own prompt.
- **Reply.** The `Stop` hook runs with `asyncRewake: true`, so it waits in the background while the terminal stays usable. When a reply arrives it prints the text to stderr and exits with code 2, which wakes the session and hands the text to Claude.

A pending Reply is withdrawn when you type a prompt in the terminal (the `UserPromptSubmit` hook) or when Claude finishes again, since only the newest one makes sense.

## The hook command

```
claude-notify hook [--event-type EVENT] [--reply-wait SECONDS]
```

`--event-type` defaults to `hook_event_name` from the input JSON. `--reply-wait` adds the Reply field and waits up to that many seconds; use it only on a hook that has `asyncRewake: true`, otherwise Claude Code waits for it.

## Things that bit us

- **macOS caches the icon by bundle id.** After changing the icon, `build.sh` bumps the build number, which is usually enough. If macOS still shows the old icon, change `CFBundleIdentifier` in `macos-helper/Info.plist` and build again. You'll be asked for notification permission once more.
- **A denied permission sticks to the bundle id too.** If the very first launch was refused, the app can't ask again. A new bundle id fixes it.
- **Channels would be nicer.** Claude Code [channels](https://code.claude.com/docs/en/channels) can relay permission prompts without holding back the terminal prompt for 20 seconds. They are still a research preview, and Team or Enterprise organizations have to switch them on first. This project sticks to hooks so it works on any account.

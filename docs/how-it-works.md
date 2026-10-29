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

If the app can't post the notification (for example, notifications are turned off for it), it writes `{"action":"error"}` instead, because `open -W` always exits 0. The hook then falls back to a plain notification. The hook also passes its own pid as `--parent`, and the app gives up as soon as that process is gone, so a killed hook never leaves a notification behind.

A pending Reply is withdrawn when you type a prompt in the terminal (the `UserPromptSubmit` hook) or when Claude finishes again, since only the newest one makes sense.

## On Windows

The same hook flow, with a PowerShell script in place of the app: `claude_notify/windows/toast.ps1`. The hook starts it with `powershell.exe` (Windows PowerShell 5.1, because PowerShell 7 can't load the WinRT notification types) and passes the request as JSON in the `CLAUDE_NOTIFY_REQUEST` environment variable, so quotes in the text never reach a command line.

- **Name and icon.** The script registers the app id `ClaudeNotify.Hook` under `HKCU\Software\Classes\AppUserModelId` with the name "Claude Notify" and `claude_notify/windows/icon.png`.
- **Sound.** Toasts of unpackaged apps can't play their own files, so the toast is silent and the script plays `%LOCALAPPDATA%\claude-notify\sounds\claude-notify-<group>.mp3` itself. If the file is missing, Windows plays its default sound.
- **Plain notifications** return at once; the script stays in the background until the sound has played.
- **Allow / Deny and Reply.** Windows PowerShell can't subscribe to WinRT events (`Register-ObjectEvent` refuses), so the waiting is done by `claude_notify/windows/ToastWaiter.cs`. The script compiles it once with the `csc.exe` that ships with the .NET Framework, into `cache\ToastWaiter-<hash>.dll`, and waits on it. The answer goes to `%LOCALAPPDATA%\claude-notify\cache\responses\<id>.json`, as on macOS.
- **Withdrawing.** Windows ends a process without running signal handlers, so the script watches the hook's pid and hides the toast once it is gone. A pending Reply is cancelled by ending the waiter with exit code 0; the pid file in `cache\waiters` is trusted only if that process started before the file was written.
- **Terminal in front.** The foreground window counts as the terminal when it belongs to one of the hook's parent processes, or to a known terminal such as Windows Terminal or VS Code. Clicking the toast body brings forward the first window up that same process tree.

## The hook command

```
claude-notify hook [--event-type EVENT] [--reply-wait SECONDS]
```

`--event-type` defaults to `hook_event_name` from the input JSON. `--reply-wait` adds the Reply field and waits up to that many seconds; use it only on a hook that has `asyncRewake: true`, otherwise Claude Code waits for it.

## Things that bit us

- **macOS caches the icon by bundle id.** After changing the icon, `build.sh` bumps the build number, which is usually enough. If macOS still shows the old icon, change `CFBundleIdentifier` in `macos-helper/Info.plist` and build again. You'll be asked for notification permission once more.
- **A denied permission sticks to the bundle id too.** If the very first launch was refused, the app can't ask again. A new bundle id fixes it.
- **Channels would be nicer.** Claude Code [channels](https://code.claude.com/docs/en/channels) can relay permission prompts without holding back the terminal prompt for 20 seconds. They are still a research preview, and Team or Enterprise organizations have to switch them on first. This project sticks to hooks so it works on any account.

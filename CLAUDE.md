# CLAUDE.md

macOS and Windows notifications for Claude Code hooks. Python package `claude_notify` (the `claude-notify hook` command) plus a Swift helper app (macOS) or a PowerShell toast script (Windows) that posts the notifications. Read `docs/how-it-works.md` before changing the hook flow.

## Map

| Path | What it is |
|---|---|
| `claude_notify/hook_handler.py` | Session name, event → group/text/sound, Allow/Deny, Reply |
| `claude_notify/notifier.py` | Posts notifications; `ask()` runs an interactive one and waits |
| `claude_notify/cli.py` | `hook` command: dispatch per event, exit codes |
| `claude_notify/windows/` | `toast.ps1` (toast, sound, waits for the answer), `ToastWaiter.cs` (click events, compiled on first use), icon, Win32 process helpers |
| `macos-helper/` | `Claude Notify.app` source; `build.sh` installs it to `~/Applications` |
| `sounds/` | MP3 per group, `install.sh` → `~/Library/Sounds`, `install.ps1` → `%LOCALAPPDATA%\claude-notify\sounds`, `generate.sh` (ElevenLabs) |
| `assets/` | Banner, app icon, `make_app_icon.py` |
| `examples/claude-settings.json` | The hooks users copy into `~/.claude/settings.json` |

`send` and `check` are from the original project and are not part of the macOS work.

## Commands

```bash
pip install .                       # reinstall after any Python change
./macos-helper/build.sh             # rebuild + install the helper app
swiftc -typecheck macos-helper/main.swift
python -m unittest discover tests   # hook contracts, no real notifications
./sounds/install.sh                 # install sounds
echo '{"hook_event_name":"Stop","session_id":"t","cwd":"/tmp/demo","last_assistant_message":"Test."}' \
  | claude-notify hook              # send a real test notification
```

For logic changes, test `HookHandler` with a fake notifier object (`send_notification`, `ask`) instead of posting real notifications.

## Contracts that must hold

- `PermissionRequest`: print decision JSON only when the user answered. No output means Claude Code shows its own prompt.
- `Stop` with `--reply-wait`: exit 2 with the reply on stderr wakes the session; exit 0 otherwise. Only valid under `asyncRewake: true`.
- Exit code 2 is reserved for Reply. Any other failure must exit 0 or 1, which Claude Code treats as non-blocking.
- In the desktop app (`CLAUDE_CODE_ENTRYPOINT=claude-desktop`) only plain notifications, no buttons and no waiting.
- One pending Reply per session, tracked in `~/Library/Caches/claude-notify/waiters/<session_id>.pid` (Windows: `%LOCALAPPDATA%\claude-notify\cache\waiters`).
- Windows: `toast.ps1` stays ASCII (PowerShell 5.1 reads it in the ANSI code page) and gets its text only through `CLAUDE_NOTIFY_REQUEST`.

## Common mistakes

| Mistake | Result | Do instead |
|---|---|---|
| Running the helper binary directly for an interactive notification | Click or reply never arrives, notification hangs | `open -n -W -a "Claude Notify.app" --args ...` (see `notifier.ask`) |
| Changing the icon and only rebuilding | macOS keeps the old cached icon | `build.sh` bumps the build number; if that fails, change `CFBundleIdentifier` |
| Killing a pid from a stale pid file | Can signal an unrelated process | Check the command line contains `claude-notify` first (see `cancel_reply_wait`) |
| Interpolating text into AppleScript | Quotes break out of the string | Escape `\` and `"` (see `_send_macos_notification`) |
| Adding a group without a sound | Default system sound plays | Add it to `GROUP_SOUNDS`, `sounds/mp3/`, `generate.sh`, both install scripts and `player.html` |
| `Register-ObjectEvent` on a toast in `toast.ps1` | "Windows PowerShell cannot subscribe to Windows RT events" | Handle the event in `ToastWaiter.cs` |
| Killing the Windows waiter with `os.kill` | Exit code 15 instead of 0 | `windows.terminate` (see `cancel_reply_wait`) |

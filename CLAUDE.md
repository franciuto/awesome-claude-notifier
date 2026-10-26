# CLAUDE.md

macOS notifications for Claude Code hooks. Python package `claude_notify` (the `claude-notify hook` command) plus a Swift helper app that posts the notifications. Read `docs/how-it-works.md` before changing the hook flow.

## Map

| Path | What it is |
|---|---|
| `claude_notify/hook_handler.py` | Session name, event → group/text/sound, Allow/Deny, Reply |
| `claude_notify/i18n.py` | Texts per language, `detect_language` (chat language → settings → English) |
| `claude_notify/notifier.py` | Posts notifications; `ask()` runs an interactive one and waits |
| `claude_notify/cli.py` | `hook` command: dispatch per event, exit codes |
| `macos-helper/` | `Claude Notify.app` source; `build.sh` installs it to `~/Applications` |
| `sounds/` | MP3 per group, `install.sh` → `~/Library/Sounds`, `generate.sh` (ElevenLabs) |
| `assets/` | Banner, app icon, `make_app_icon.py` |
| `examples/claude-settings.json` | The hooks users copy into `~/.claude/settings.json` |

`send`, `watch` and `config` are from the original project and are not part of the macOS work.

## Commands

```bash
pip install .                       # reinstall after any Python change
./macos-helper/build.sh             # rebuild + install the helper app
swiftc -typecheck macos-helper/main.swift
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
- One pending Reply per session, tracked in `~/Library/Caches/claude-notify/waiters/<session_id>.pid`.

## Common mistakes

| Mistake | Result | Do instead |
|---|---|---|
| Running the helper binary directly for an interactive notification | Click or reply never arrives, notification hangs | `open -n -W -a "Claude Notify.app" --args ...` (see `notifier.ask`) |
| Changing the icon and only rebuilding | macOS keeps the old cached icon | `build.sh` bumps the build number; if that fails, change `CFBundleIdentifier` |
| Killing a pid from a stale pid file | Can signal an unrelated process | Check the command line contains `claude-notify` first (see `cancel_reply_wait`) |
| Interpolating text into AppleScript | Quotes break out of the string | Escape `\` and `"` (see `_send_macos_notification`) |
| Adding a group without a sound | Default macOS sound plays | Add `sounds/mp3/<group>.mp3`, both scripts, `player.html` and the group name in every `i18n.TEXTS` block |
| Adding a language to `TEXTS` only | Never detected, texts stay in the fallback language | Also add its common words to `STOPWORDS` |
| Translating the buttons | Not wanted: Allow/Deny/Reply/Send stay English | Leave `registerCategories` in `main.swift` as is |
| Hard-coding a user-facing string in `hook_handler.py` | Wrong language in half the sessions | Put it in `i18n.TEXTS` for every language |

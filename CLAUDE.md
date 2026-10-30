# CLAUDE.md

macOS, Windows and Linux notifications for Claude Code hooks, shipped as a Claude Code plugin (`notifier`, marketplace `awesome-claude-notifier`). A Go binary `claude-notify` (the `claude-notify hook` command) plus a Swift helper app (macOS) or a PowerShell toast script (Windows) post the notifications. Read `docs/how-it-works.md` before changing the hook flow.

## Map

| Path | What it is |
|---|---|
| `cmd/claude-notify/` | `main.go`: the `hook` command, dispatch per event, exit codes, panic recovery |
| `internal/hook/` | Session name, event → group/text/sound (`describe.go`), texts per language and chat-language detection (`i18n.go`), Allow/Deny, Reply, pid files (`handler.go`), terminal-in-front per OS |
| `internal/notify/` | Posts notifications per OS; `Ask` runs an interactive one and waits; macOS app install (`ensureApp`) |
| `internal/winproc/` | Windows process helpers |
| `bin/claude-notify` | POSIX `sh` dispatcher that `exec`s the binary for this OS and CPU (LF line endings) |
| `bin/<os-arch>/` | Committed prebuilt binaries: `darwin` (universal), `linux-amd64`, `linux-arm64`, `windows-amd64`, `windows-arm64` |
| `hooks/hooks.json` | The four hooks the plugin registers |
| `.claude-plugin/` | `plugin.json` (the `version`) and `marketplace.json` |
| `windows/` | `toast.ps1` (toast, sound, waits for the answer), `ToastWaiter.cs` (click events, compiled on first use), `icon.png` |
| `macos-helper/` | `Claude Notify.app` source; `build.sh` writes the app into `macos/` |
| `macos/` | Committed prebuilt `Claude Notify.app` (universal, ad-hoc signed, sounds inside) |
| `sounds/` | MP3 per group in `mp3/`, `generate.sh` (ElevenLabs), `soften.sh` (fade-in and level cap, run by `generate.sh`; needs ffmpeg), `player.html` |
| `assets/` | Banner, app icon, `make_app_icon.py`, README mockups in `demo/` (HTML sources and their PNGs) |
| `scripts/test-dispatcher.sh` | Shell test for the dispatcher |
| `README.md`, `CHANGELOG.md` | User docs, including the install prompt for agents under "Onboard your agent"; the changelog |

## Commands

```bash
make test                           # go vet + go test, no real notifications
make fmt                            # gofmt -l -w .
make dist-go                        # rebuild the five Go binaries (reproducible)
make dist                           # dist-go + build.sh (the app, macOS only)
swiftc -typecheck macos-helper/main.swift
sh scripts/test-dispatcher.sh
echo '{"hook_event_name":"Stop","session_id":"t","cwd":"/tmp/demo","last_assistant_message":"Test."}' \
  | ./bin/claude-notify hook        # send a real test notification
```

For logic changes, test `Handler` with a fake `notify.Notifier` (`Send`, `Ask`) instead of posting real notifications.

Release: bump `version` in `.claude-plugin/plugin.json`, run `make dist`, commit `bin/` and `macos/` with the bump, update `CHANGELOG.md`, open a PR into `master`.

## Contracts that must hold

- `PermissionRequest`: print decision JSON only when the user answered. No output means Claude Code shows its own prompt.
- `Stop` with `--reply-wait`: exit 2 with the reply on stderr wakes the session; exit 0 otherwise. Only valid under `asyncRewake: true`.
- Exit code 2 is reserved for Reply. Any other failure must exit 0 or 1, which Claude Code treats as non-blocking. This includes Go panics and flag errors, and the dispatcher exits 0 when it has no binary.
- In the desktop app (`CLAUDE_CODE_ENTRYPOINT=claude-desktop`) only plain notifications, no buttons and no waiting.
- One pending Reply per session, tracked in `~/Library/Caches/claude-notify/waiters/<session_id>.pid` (Windows: `%LOCALAPPDATA%\claude-notify\cache\waiters`).
- Windows: `toast.ps1` stays ASCII (PowerShell 5.1 reads it in the ANSI code page) and gets its text only through `CLAUDE_NOTIFY_REQUEST`.
- `macos-helper/main.swift`, `windows/toast.ps1` and `windows/ToastWaiter.cs` are the interface the Go code targets; change them only on purpose.

## Common mistakes

| Mistake | Result | Do instead |
|---|---|---|
| Running the helper binary directly for an interactive notification | Click or reply never arrives, notification hangs | `open -n -g -W -a "Claude Notify.app" --args ...` (see `darwin.Ask`) |
| Changing the icon and only rebuilding | macOS keeps the old cached icon | `build.sh` sets the app version to the plugin version, so bump `version` in `plugin.json` and run `make dist`; if that isn't enough, change `CFBundleIdentifier` |
| Killing a pid from a stale pid file | Can signal an unrelated process | Check the command line contains `claude-notify` first (see `CancelReplyWait`) |
| Interpolating text into AppleScript | Quotes break out of the string | Escape `\` and `"` (see `darwin.Send`) |
| Adding a group without a sound | Default system sound plays | Add it to `groupSlugs` in `internal/hook/describe.go`, `sounds/mp3/`, `generate.sh` and `player.html`, then `make dist` |
| Adding a language to `languages` only | Never detected, texts stay in the fallback language | Also add its common words to `stopwords` in `i18n.go` |
| Translating the buttons or group names | Not wanted: Allow/Deny/Reply/Send and Done/Needs you/... stay English | Only body sentences go in `languages` |
| Hard-coding a user-facing sentence in `describe.go` or `handler.go` | Wrong language in half the sessions | Put it in `languages` in `i18n.go` for every language |
| `Register-ObjectEvent` on a toast in `toast.ps1` | "Windows PowerShell cannot subscribe to Windows RT events" | Handle the event in `ToastWaiter.cs` |
| Killing the Windows waiter with a plain kill | Exit code 15 instead of 0 | Use `stopWaiter` in `internal/hook/waiter_windows.go` |
| A Go panic or a `flag` default (it exits 2 on a bad flag) | Claude Code reads exit 2 as a Reply | Keep the recovery in `run` and `flag.ContinueOnError`; exit 1 |
| Committing binaries without a version bump | Users never get them, or get binaries that don't match the source | Rebuild `bin/` and `macos/` only with a `version` bump |
| Editing `bin/claude-notify` on Windows with CRLF | `sh` fails on the shebang under Git Bash | `.gitattributes` forces LF; keep it |

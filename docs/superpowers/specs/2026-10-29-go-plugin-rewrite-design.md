# Go rewrite and Claude Code plugin packaging

Status: draft, waiting for approval
Branch: `feat/go-rewrite` (from `dev`)

## Why

Installing claude-notify today takes three or four manual steps per OS: `pip install .` from a clone, building the macOS helper with the Xcode tools, installing the sounds, and hand-copying hooks into `settings.json`, often with the full path of a virtualenv. Linux is not documented at all.

The typical user is not technical and will most likely ask their own agent to "install claude-notify". The repo should carry a ready-made prompt for that. That is no excuse for a complicated process: the steps must be short for a human too.

Two decisions came out of the discussion with the owner:

- **No runtime to install.** Python is not reliably present: on a clean Mac `/usr/bin/python3` is a stub that asks for the Command Line Tools, and on Windows `python3` can open the Microsoft Store. The hook logic is rewritten in Go and shipped as prebuilt binaries.
- **No publishing channel to maintain.** No PyPI, no Apple Developer ID, no notarization, no release hosting. The GitHub repo itself is the distribution: it becomes a Claude Code plugin marketplace with one plugin.

## Goals

1. Install with two commands inside Claude Code, the same on every OS:
   ```
   /plugin marketplace add franciuto/awesome-claude-notifier
   /plugin install claude-notify@awesome-claude-notifier
   ```
   No clone, no `pip`, no build, no Xcode tools, no sound installer, no `settings.json` editing for hooks.
2. Same features as today on macOS and Windows: session titles, groups, per-group sounds, Allow/Deny, Reply, withdrawing a pending Reply, terminal-in-front detection, plain notifications in the desktop app.
3. Linux works with plain notifications, as today, without extra steps beyond having `notify-send`.
4. A ready-made, OS-independent prompt in the repo that a user can paste into their agent.
5. Every contract in `CLAUDE.md` ("Contracts that must hold") keeps holding, proven by Go tests.

## Non-goals

- New features during the port. Linux buttons (`notify-send --action`), Linux sounds, XDG cache paths and similar improvements come after the port, in separate changes (see "After the port").
- Rewriting the macOS helper (`macos-helper/main.swift`) or the Windows toast script (`toast.ps1`, `ToastWaiter.cs`). They stay byte-identical; their command-line and JSON contracts are the interface the Go code targets. Exception: `build.sh` changes as described below.
- Keeping the Python package or `pip install` as a second install path.
- The legacy `send` and `check` commands from the original project. They are dropped.

## Anti-goals

These would make the result a failure even if it technically works:

- An install path that needs a compiler, a language runtime, or a signed Apple artifact on the user's machine.
- A hook that can exit with code 2 for anything other than a Reply (a parse error, a missing binary, an unknown OS).
- Users receiving duplicated notifications after installing, without being told why.

## Constraints

- Claude Code plugin facts (from the official docs, checked on 2026-10-29):
  - Plugin hooks live in `hooks/hooks.json` with the same schema as `settings.json` hooks, including `timeout` and `asyncRewake`.
  - `${CLAUDE_PLUGIN_ROOT}` is the absolute path of the installed plugin version. It changes on every update and is also exported to hook processes as an environment variable.
  - Plugins are fetched with `git clone` into `~/.claude/plugins/cache/<marketplace>/<plugin>/<version>/`. On this Mac the cached files keep their executable bits and carry no `com.apple.quarantine` attribute.
  - Without `version` in `plugin.json`, every commit is an update; with `version`, users update only when it changes.
  - There is no post-install script. A plugin cannot set `preferredNotifChannel` (only `agent` and `subagentStatusLine` settings are honored).
  - The same handler in `settings.json` and in a plugin runs twice.
  - Shell-form hook commands run in `sh` on macOS/Linux and in Git Bash on Windows, falling back to PowerShell when Git Bash is missing.
- Windows: `toast.ps1` must stay ASCII and gets its input only through `CLAUDE_NOTIFY_REQUEST` (existing contract).
- macOS: interactive notifications must go through `open -n -W -a "<app>" --args ...` (existing contract).

## Design

### Repository layout after the change

```
.claude-plugin/
  marketplace.json        marketplace "awesome-claude-notifier", one plugin, source "./"
  plugin.json             name "claude-notify", version "0.2.0"
hooks/hooks.json          the four hooks from examples/claude-settings.json, pointing at bin/claude-notify
bin/
  claude-notify           POSIX sh dispatcher (LF line endings)
  darwin/claude-notify    universal binary (arm64 + amd64)
  linux-amd64/claude-notify
  linux-arm64/claude-notify
  windows-amd64/claude-notify.exe
  windows-arm64/claude-notify.exe
cmd/claude-notify/        main package
internal/hook/            event → group/text, session name, permission, reply, waiters
internal/notify/          Notifier interface + darwin / windows / linux implementations
internal/winproc/         Win32 process helpers (windows build tag)
macos/Claude Notify.app/  prebuilt helper app, ad-hoc signed, sounds inside
macos-helper/             Swift source, Info.plist, build.sh (developers only)
windows/                  toast.ps1, ToastWaiter.cs, icon.png (moved from claude_notify/windows/)
sounds/                   mp3 sources, generate.sh, player.html
INSTALL-PROMPT.md         the prompt for agents
CHANGELOG.md
Makefile
go.mod
.gitattributes
.github/workflows/ci.yml
```

Removed: `claude_notify/`, `pyproject.toml`, `tests/test_hook.py` (ported to Go), `sounds/install.sh`, `sounds/install.ps1`, `examples/claude-settings.json` (replaced by `hooks/hooks.json`). The Python-oriented `.gitignore` is replaced with one for this layout; it must not ignore `bin/` or `macos/`.

### Go program

- Single command: `claude-notify hook [--event-type EVENT] [--reply-wait SECONDS]`, same flags and behavior as today.
- Testable core: `run(args []string, stdin io.Reader, stdout, stderr io.Writer, env) int` returns the exit code; `main` only calls it and `os.Exit`s. Tests assert exit codes without spawning processes.
- `flag.NewFlagSet(..., flag.ContinueOnError)`, with parse errors mapped to exit 1. Go's default `flag` behavior exits with 2, which is reserved for Reply.
- A `Notifier` interface with `Send(...)` and `Ask(...)`, matching `send_notification` and `ask` today, so `internal/hook` is tested with a fake notifier like the Python tests do.
- Standard library only, plus `golang.org/x/sys/windows` for the Win32 calls that `ctypes` makes today. `CGO_ENABLED=0`.
- Asset lookup: the hook reads `CLAUDE_PLUGIN_ROOT` from its environment, falling back to two directories above the executable. Windows uses `windows/toast.ps1`, `windows/icon.png` and `sounds/mp3/<group>.mp3` straight from there, so no sound installer is needed. Cache and responses directories stay where they are today (`~/Library/Caches/claude-notify`, `%LOCALAPPDATA%\claude-notify\cache`).
- `summarize`: Python uses a lookbehind (`(?<=[.!?])\s+`), which Go's RE2 lacks. Sentence splitting is done with a small loop instead. The Python test cases are ported verbatim, plus cases for multi-sentence cuts and the `…` fallback.

### Dispatcher `bin/claude-notify`

A POSIX `sh` script that picks the binary by `uname -s` / `uname -m` (`PROCESSOR_ARCHITECTURE` on Windows Git Bash) and `exec`s it with all arguments. On an unknown platform or a missing binary it exits 0 silently. It never exits 2.

Hook commands use shell form:

```json
"command": "\"${CLAUDE_PLUGIN_ROOT}/bin/claude-notify\" hook --event-type Stop --reply-wait 3540"
```

### macOS helper app

- `macos/Claude Notify.app` is committed: a universal build of `main.swift`, ad-hoc signed by `build.sh`, with the six sounds converted to `.aiff` in `Contents/Resources`.
- The app is not launched from the plugin cache. `${CLAUDE_PLUGIN_ROOT}` changes on every update, and the helper relies on LaunchServices relaunching the registered copy to deliver clicks. Before posting, the Go binary copies the bundled app to `~/Applications/Claude Notify.app` with `ditto` (keeps the signature) when the installed `CFBundleVersion` differs or the app is missing. This is the same location as today.
- `build.sh` writes into `macos/` instead of `~/Applications`, builds for both architectures and merges them with `lipo`. `CFBundleVersion` becomes deterministic: it is derived from the plugin version instead of the current date. It still changes on every release, so the icon cache is still refreshed.
- Sounds: if `UNNotificationSound(named:)` finds the `.aiff` inside the bundle (spike S3), nothing else is needed. If not, the same ensure step copies them to `~/Library/Sounds`, as `install.sh` does today.

### Windows

The Go binary starts `powershell.exe ... -File <plugin root>/windows/toast.ps1` with `CLAUDE_NOTIFY_REQUEST`, exactly as `notifier.py` does today, including `CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP`. `toast.ps1` already compiles `ToastWaiter.cs` into the cache keyed by its hash, so the changing plugin path is harmless.

### Linux

Plain notifications through `notify-send`, as today. `Ask` returns "unavailable", so permission prompts fall back to the terminal and Reply falls back to a plain notification.

### The agent prompt (`INSTALL-PROMPT.md`)

One prompt, the same text for every OS. It tells the agent to:

1. Run the two `/plugin` commands (or `claude plugin marketplace add ...` and `claude plugin install ...` from a shell).
2. Set `preferredNotifChannel` to `notifications_disabled` in the user's `settings.json`, so Claude Code's own notifications don't duplicate ours.
3. Remove any old `claude-notify hook` entries from the user's `settings.json`, which would otherwise fire twice, and offer to `pip uninstall claude-notify`.
4. On macOS, tell the user to allow notifications for "Claude Notify" and set the style to Alerts. On Linux, check that `notify-send` exists and say how to install it if not. On Windows, check for Git for Windows (depends on spike S1).
5. Send a test notification by piping a sample `Stop` event into the binary.

The README shows the two commands first, then the manual equivalents of steps 2–4 for people who don't use an agent.

### Versioning and release

- `plugin.json` has an explicit `version`. Users only receive an update when it changes, so intermediate commits never ship binaries that don't match the source.
- Binaries and the `.app` are rebuilt and committed only together with a version bump (`make dist`).
- Semver. This change is `0.2.0`: the install method breaks, which a pre-1.0 minor bump allows.
- `CHANGELOG.md` in Keep a Changelog format, starting with `0.2.0`.
- Commit messages follow the existing repo style: one imperative sentence, no `feat:` prefix, Claude co-author trailer.
- Work happens on `feat/go-rewrite` in small commits and is merged into `dev` through a pull request. Tagging `v0.2.0` and merging to `master` are the owner's decisions.

### Build and CI

- `go.mod` pins the Go version with a `toolchain` line.
- Reproducible builds: `CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w"`.
- `Makefile` targets: `test` (`go vet`, `go test ./...`), `dist` (all Go binaries, macOS universal binary via `lipo`, `.app` via `build.sh`), `fmt`.
- `.gitattributes`: `bin/claude-notify text eol=lf` (Git for Windows defaults to `autocrlf=true`, and a CRLF shebang breaks the hook under Git Bash); the binaries under `bin/*/` and everything under `macos/` marked `binary -diff`.
- GitHub Actions (`.github/workflows/ci.yml`, tests only, no publishing): `gofmt` check, `go vet`, `go test` on ubuntu, macos and windows; `swiftc -typecheck macos-helper/main.swift` on macos; rebuild the Go binaries and fail if they differ from the committed ones (`git diff --exit-code bin/`). The `.app` is not checked for reproducibility.

### Tests

- Port every case in `tests/test_hook.py` to Go: permission answered / unanswered, reply exits 2 with text, no reply exits 0, helper failure falls back to plain, usage error is exit 1, answer file reading, summarize.
- Add: dispatcher picks the right binary per `uname` (shell test run in CI on ubuntu and macos), `CLAUDE_PLUGIN_ROOT` asset lookup, desktop app gets no buttons, stale waiter pid is not killed.
- Unit tests never post real notifications.

### Docs to update

`README.md` (install section rewritten around the plugin, Linux section added, "Make it yours" adjusted), `CLAUDE.md` (map, commands, contracts paths, common mistakes), `docs/how-it-works.md` (Go binary instead of Python, plugin hooks, app copy step), `sounds/player.html` and `generate.sh` paths if they move.

## Spikes before the implementation plan

Each spike is throwaway code, run on a real machine, with a pass criterion and a fallback.

| # | Question | Pass | Fallback |
|---|---|---|---|
| S1 | Windows: does the shell-form command reach the binary under Git Bash with a backslash `CLAUDE_PLUGIN_ROOT`? What happens under PowerShell-only? | Hook runs under Git Bash; under PowerShell it fails with an exit code other than 2 | Git for Windows becomes a stated prerequisite on Windows, checked by the agent prompt |
| S2 | macOS: a `.app` committed to the repo, installed through a real `/plugin install`, copied to `~/Applications` with `ditto`: does it launch via `open -n -W -a` without Gatekeeper, get notification permission, and route Allow/Deny and Reply back? | All three work | Copy with `cp -R` and re-run `codesign --force --sign -` locally; if that fails, stop and revisit |
| S3 | macOS: is an `.aiff` in `Contents/Resources` found by `UNNotificationSound(named:)`? | Custom sound plays | Copy sounds to `~/Library/Sounds` in the ensure step |
| S4 | A plugin `Stop` hook with `asyncRewake`, going through the `sh` dispatcher's `exec`, exiting 2: does it wake the session with the stderr text? | Session wakes | Investigate before going further; Reply depends on it |

S1 needs a Windows machine. If none is available, S1 is recorded as untested and the Windows prerequisite is stated conservatively (Git for Windows required).

## After the port (separate changes, not in this design)

- Linux: XDG cache path, Allow/Deny through `notify-send --action ... --wait` where libnotify ≥ 0.7.10, sounds through `paplay`/`pw-play`.
- A `SessionStart` hook that tells the user once if `preferredNotifChannel` is still on.

## Left to the builder

- Internal package boundaries beyond the three listed, function names, file split inside packages.
- The exact dispatcher script, as long as it meets the rules above.
- The wording of `INSTALL-PROMPT.md` and the README, following the existing plain style.

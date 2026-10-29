# Go Rewrite and Plugin Packaging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Python package with a Go binary shipped inside the repo, and package the repo as a Claude Code plugin that installs with two commands on macOS, Windows and Linux.

**Architecture:** `cmd/claude-notify` parses flags and maps outcomes to exit codes. `internal/hook` turns a hook event into a notification and handles Allow/Deny, Reply and waiters, through a `notify.Notifier` interface. `internal/notify` has one implementation per OS, which drives the unchanged Swift helper (macOS), `toast.ps1` (Windows) or `notify-send` (Linux). A POSIX `sh` dispatcher in `bin/` picks the prebuilt binary for the platform.

**Tech Stack:** Go 1.26 (standard library + `golang.org/x/sys/windows`), Swift helper (unchanged source), PowerShell 5.1 script (unchanged), POSIX sh, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-10-29-go-plugin-rewrite-design.md`

## Global Constraints

- Module path `github.com/franciuto/awesome-claude-notifier`; `go.mod` has `go 1.26` and `toolchain go1.26.5`.
- `CGO_ENABLED=0`; builds use `-trimpath -buildvcs=false -ldflags="-s -w"`.
- Exit code 2 only for a Reply. Every other failure, including Go panics and flag errors, exits 0 or 1.
- `PermissionRequest`: print decision JSON only when the user answered Allow or Deny.
- Desktop app (`CLAUDE_CODE_ENTRYPOINT=claude-desktop`): plain notifications only, no `Ask`.
- One pending Reply per session: pid file `<cache>/waiters/<session_id>.pid`. Before killing, macOS/Linux check that the process command line contains `claude-notify`; Windows checks that the process started no later than the pid file's mtime + 1 s, and ends it with exit code 0.
- Cache dir: macOS `~/Library/Caches/claude-notify`, Windows `%LOCALAPPDATA%\claude-notify\cache`, Linux unchanged from today (`~/Library/Caches/claude-notify`; XDG is out of scope).
- `macos-helper/main.swift`, `windows/toast.ps1`, `windows/ToastWaiter.cs` stay byte-identical to today's files (`toast.ps1` and `ToastWaiter.cs` are moved with `git mv`).
- macOS interactive notifications go through `open -n -g -W -a <app> --args ...`.
- Windows: `toast.ps1` gets its input only through `CLAUDE_NOTIFY_REQUEST`; it is started with `powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File <path>` and creation flags `CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP`.
- Notification title: `<session name> · <group>`. Body cut to 120 characters (runes), ending in `…` when cut mid-sentence.
- Groups and sound slugs: Needs you `needs-you`, Done `done`, Paused `paused`, Error `error`, Limit `limit`, Info `info`.
- Plugin version `0.2.0`. Marketplace name `awesome-claude-notifier`, plugin name `claude-notify`.
- Commit messages: one imperative sentence, no `feat:` prefix, no co-author or attribution trailer. Never `git add -A` (the unrelated deletion of `.claude/settings.local.json` stays unstaged).

## Review Focus

- **Panics.** A Go panic exits with code 2, which Claude Code reads as a Reply. Expect: any panic inside `run` is recovered and returns 1. Test in Task 1.
- **Huge transcript lines.** Transcripts have single JSON lines of several MB; `bufio.Scanner` stops at 64 KB. Expect: a title record after a 1 MB line is still found. Test in Task 2.
- **Non-ASCII text cut.** Italian accents and emoji in Claude's message. Expect: the summary is valid UTF-8 and at most 120 runes. Test in Task 2.
- **Odd stdin.** `null`, `[]`, `""`, empty input, invalid JSON. Expect: exit 1, message on stderr, no panic. Test in Task 1.
- **Spaces in paths.** A user name or `CLAUDE_PLUGIN_ROOT` containing spaces. Expect: the dispatcher still `exec`s the binary with all arguments intact. Test in Task 6.

---

### Task 1: Go module and command entry point

**Files:**
- Create: `go.mod`, `cmd/claude-notify/main.go`, `cmd/claude-notify/main_test.go`, `Makefile`, `.gitattributes`
- Replace: `.gitignore` (Go layout; must not ignore `bin/` or `macos/`)

**Interfaces:**
- Produces: `func run(args []string, stdin io.Reader, stdout, stderr io.Writer, h Hook) int` and the interface it dispatches to:
  ```go
  type Hook interface {
      Permission(data map[string]any) map[string]any      // nil = print nothing
      CancelReplyWait(sessionID string)
      InDesktopApp() bool
      NotifyAndWaitReply(event string, data map[string]any, wait time.Duration) string // "" = no reply
      Process(event string, data map[string]any) bool
  }
  ```
  `*hook.Handler` (Task 3) implements it. `main` builds `hook.NewHandler()` and calls `os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, h))`.

- [ ] **Step 1: Write the failing tests** in `cmd/claude-notify/main_test.go` with a `fakeHook` recording calls:
  - `TestUsageErrorIsExit1`: `run([]string{"hook","--bogus"}, ...)` → 1.
  - `TestUnknownCommandIsExit1`: `run([]string{"send"}, ...)` → 1; `run(nil, ...)` → 1.
  - `TestBadStdinIsExit1`: for inputs `""`, `"null"`, `"[]"`, `"\"x\""`, `"{"`, `"{}"` → 1, stderr non-empty.
  - `TestMissingEventIsExit1`: `{"session_id":"t"}` without `--event-type` → 1.
  - `TestPanicIsExit1`: fakeHook whose `Process` panics → 1.
  - `TestPermissionPrintsDecisionOnlyWhenAnswered`: fake returns `{"hookSpecificOutput":...}` → stdout is that JSON, exit 0; fake returns nil → stdout `""`, exit 0.
  - `TestUserPromptSubmitCancels`: calls `CancelReplyWait("t")`, exit 0.
  - `TestReplyExit2WithText`: `--reply-wait 60`, fake reply `"go on"` → exit 2, stderr contains `go on` and the prefix `Message from the user, sent by replying to a desktop notification.`
  - `TestNoReplyExit0`: fake reply `""` → 0.
  - `TestDesktopAppSkipsReply`: `InDesktopApp()` true with `--reply-wait 60` → `Process` called, exit 0.
  - `TestProcessFailureStillExit0`: `Process` returns false → exit 0, stderr has a warning.

- [ ] **Step 2: Run** `go test ./cmd/...` — Expected: FAIL (`run` undefined).

- [ ] **Step 3: Implement `run`.** `flag.NewFlagSet("hook", flag.ContinueOnError)` with `--event-type`/`-e` (string) and `--reply-wait` (int seconds), output to `stderr`. A deferred `recover` sets the return value to 1. Decode stdin into `map[string]any`; empty or non-object → 1. Event = flag or `hook_event_name`. Dispatch order and stderr text exactly as `claude_notify/cli.py:126-170`. The reply text sent on stderr is:
  ```
  Message from the user, sent by replying to a desktop notification. Treat it as a new message from them and answer normally:\n\n<reply>
  ```

- [ ] **Step 4: Add** `go.mod` (see Global Constraints), `.gitattributes` (`bin/claude-notify text eol=lf`, `bin/*/* binary -diff`, `macos/** binary -diff`), `.gitignore`, and `Makefile` targets `test` (`go vet ./... && go test ./...`) and `fmt` (`gofmt -l -w .`).

- [ ] **Step 5: Run** `make test` — Expected: PASS.

- [ ] **Step 6: Commit** — `git add go.mod cmd Makefile .gitattributes .gitignore` — "Add the Go entry point for the hook command".

### Task 2: Event texts and session name

**Files:**
- Create: `internal/hook/describe.go`, `internal/hook/describe_test.go`, `internal/hook/session.go`, `internal/hook/session_test.go`

**Interfaces:**
- Produces:
  - Group constants `NeedsYou, Done, Paused, Error, Limit, Info` (the display strings) and `func SoundSlug(group string) string` (`"needs-you"` … `"info"`, `""` for unknown).
  - `func Summarize(text string, max int) string` and `const SummaryMaxChars = 120`.
  - `func Describe(event string, data map[string]any) (group, summary string)`.
  - `func (h *Handler) SessionName(data map[string]any) string` using `h.DesktopSessionsDirs []string` (field defined in Task 3; declare the `Handler` struct here with that field and add the rest in Task 3).

- [ ] **Step 1: Write the failing tests.**
  - `TestSummarizeKeepsUnderscoresStripsMarkdown`: `Summarize("**Fixed** `+"`hook_handler.py`"+`", 120)` == `"Fixed hook_handler.py"`.
  - `TestSummarizeCutsLongText`: `Summarize(strings.Repeat("word ", 100), 120)` has ≤ 120 runes and ends with `"…"`.
  - `TestSummarizeKeepsWholeSentences`: `Summarize("One. Two two. "+strings.Repeat("x",130), 120)` == `"One. Two two."`.
  - `TestSummarizeUnicode`: `Summarize(strings.Repeat("perché è così 🎉 ", 20), 120)` is `utf8.ValidString`, ≤ 120 runes, ends with `"…"`.
  - `TestSummarizeStripsCodeBlocksLinksBullets`: "```go\nx\n```\n- [doc](http://x) here" → `"doc here"`.
  - `TestDescribe` table, one row per branch of `describe_event` in `claude_notify/hook_handler.py:209-243`, including `Stop` with `background_tasks` → Paused, `Notification` with `permission to use Bash` → `"Asking to use Bash"`, unknown `notification_type` → Info, `StopFailure` `rate_limit` with details, `PermissionDenied`, `SubagentStop` with empty message → `"A subagent finished"`, `TaskCompleted`, unknown event → (Info, event name).
  - `TestSessionNamePrefersCustomTitle`: transcript with `ai-title` then `custom-title` → custom title; only `ai-title` → it.
  - `TestSessionNameHugeLine`: transcript whose first line is 1 MB of JSON, then a `custom-title` record → custom title found.
  - `TestSessionNameDesktopApp`: temp dir `<d>/a/b/local_1.json` with `{"cliSessionId":"s1","title":"Desk"}` → `"Desk"`.
  - `TestSessionNameFallbacks`: no transcript, no desktop match, `cwd` `/tmp/demo` → `"demo"`; nothing → `"Session"`.

- [ ] **Step 2: Run** `go test ./internal/hook/` — Expected: FAIL.

- [ ] **Step 3: Implement.** Port the regexes of `summarize` (`claude_notify/hook_handler.py:109-127`) to `regexp` (use `(?s)` and `(?m)` flags). Go RE2 has no lookbehind; split sentences with a loop that ends a sentence after `.`, `!` or `?` followed by whitespace. Count and cut in runes. Read the transcript with `bufio.Reader.ReadString('\n')` (no line limit), skipping lines without `-title"` before `json.Unmarshal`. Desktop dirs are globbed with `filepath.Glob(dir+"/*/*/local_*.json")`, and a file is parsed only if it contains the session id.

- [ ] **Step 4: Run** `go test ./internal/hook/` — Expected: PASS.

- [ ] **Step 5: Commit** — "Port event texts and session names to Go".

### Task 3: Hook flow — permission, reply, waiters

**Files:**
- Create: `internal/notify/notify.go`, `internal/hook/handler.go`, `internal/hook/handler_test.go`, `internal/hook/platform_darwin.go`, `internal/hook/platform_windows.go`, `internal/hook/platform_other.go`, `internal/hook/waiter_unix.go`, `internal/hook/waiter_windows.go`
- Modify: `cmd/claude-notify/main.go` (use `hook.NewHandler()`)

**Interfaces:**
- Produces in `internal/notify/notify.go`:
  ```go
  type Request struct { Title, Body, Sound string; Urgent bool } // Sound = slug, "" = default sound
  type Answer struct { Action, Text string }                      // allow|deny|reply|open|timeout
  type Notifier interface {
      Send(r Request) bool
      Ask(r Request, actions string, wait time.Duration) *Answer // actions "permission"|"reply"; nil = unavailable
  }
  func CacheDir() string
  func New() Notifier // per-OS implementation, Tasks 4-5
  ```
- Produces in `internal/hook/handler.go`:
  ```go
  type Handler struct {
      Notifier             notify.Notifier
      Getenv               func(string) string
      UserAtTerminal       func() bool
      BringTerminalForward func()
      WaitersDir           string
      DesktopSessionsDirs  []string
  }
  func NewHandler() *Handler // real defaults from platform_*.go
  ```
  plus the five `Hook` methods from Task 1. `const PermissionWait = 20 * time.Second`.
- Consumes: `Describe`, `Summarize`, `SoundSlug`, `SessionName` (Task 2).

- [ ] **Step 1: Write the failing tests** with a fake `Notifier` (records `Send`/`Ask` calls, returns a preset `*Answer`) and a temp `WaitersDir`:
  - `TestPermissionAnswered`: Answer allow → `{"hookSpecificOutput":{"hookEventName":"PermissionRequest","decision":{"behavior":"allow"}}}`; deny → decision also has `"message":"The user denied permission from the notification."`.
  - `TestPermissionUnanswered`: answers `timeout`, `open`, nil → nil; `open` calls `BringTerminalForward`.
  - `TestPermissionSkippedAtTerminalOrDesktop`: `UserAtTerminal` true → nil, no `Ask`; `CLAUDE_CODE_ENTRYPOINT=claude-desktop` → nil, no `Ask`.
  - `TestPermissionRequestText`: `tool_input.command "ls -la"` → Ask body `"Bash: ls -la"`, title `"<name> · Needs you"`, sound `"needs-you"`, wait 20 s.
  - `TestReplyReturnsTrimmedText`: Answer reply `" go on "` → `"go on"`; blank text → `""`; timeout → `""`.
  - `TestReplyUnavailableFallsBackToPlain`: Ask returns nil → exactly one `Send`, returns `""`.
  - `TestReplyWritesAndRemovesPidFile`: during `Ask` the file `<WaitersDir>/t.pid` holds `os.Getpid()`; afterwards it is gone.
  - `TestCancelIgnoresForeignPid`: pid file containing the pid of a `sleep 30` child (not `claude-notify`) → the child is still alive after `CancelReplyWait`, and the pid file is removed. (Unix only; `//go:build !windows`.)
  - `TestProcessSendsGroupTitleAndSound`: `Stop` event → one `Send` with title `"S · Done"`, sound `"done"`; `StopFailure` → `Urgent` true.

- [ ] **Step 2: Run** `go test ./internal/hook/` — Expected: FAIL.

- [ ] **Step 3: Implement** the methods by porting `claude_notify/hook_handler.py:249-379`. Platform defaults:
  - `platform_darwin.go`: `UserAtTerminal` via `lsappinfo front` + `lsappinfo info -only bundleid` (regex and `TERMINAL_BUNDLE_IDS` from `hook_handler.py:95-99`, own id from `__CFBundleIdentifier`); `BringTerminalForward` = `open -b <id>`; desktop dir `~/Library/Application Support/Claude/claude-code-sessions`.
  - `platform_windows.go`: `UserAtTerminal` via `winproc.ForegroundApp` + `winproc.Ancestors` + `TERMINAL_EXES` (`hook_handler.py:102-106`); `BringTerminalForward` no-op; desktop dirs `%APPDATA%\Claude\claude-code-sessions` and `%LOCALAPPDATA%\Packages\Claude_*\LocalCache\Roaming\Claude\claude-code-sessions`.
  - `platform_other.go` (`//go:build !darwin && !windows`): `UserAtTerminal` false, `BringTerminalForward` no-op, the macOS desktop dir (same as today).
  - `waiter_unix.go`: `ps -p <pid> -o command=` must contain `claude-notify`, then `SIGTERM`. `waiter_windows.go`: `winproc.StartedAt` ≤ mtime + 1 s, then `winproc.Terminate` (exit code 0).
  - `notify.New()` may return a stub until Tasks 4-5; keep it compiling on all three OSes.

- [ ] **Step 4: Run** `make test` and `GOOS=windows go vet ./... && GOOS=linux go vet ./...` — Expected: PASS, no output from vet. (`winproc` arrives in Task 5; until then `platform_windows.go` and `waiter_windows.go` may hold TODO-free stubs returning false/no-op, replaced in Task 5.)

- [ ] **Step 5: Commit** — "Port Allow/Deny, Reply and waiters to Go".

### Task 4: macOS notifier and app install step

**Files:**
- Create: `internal/notify/paths.go`, `internal/notify/answer.go`, `internal/notify/answer_test.go`, `internal/notify/darwin.go`, `internal/notify/darwin_test.go`

**Interfaces:**
- Produces:
  - `func PluginRoot() string`: `CLAUDE_PLUGIN_ROOT` if set, else two directories above `os.Executable()` (`bin/<platform>/claude-notify` → repo root).
  - `func readAnswer(file string, exitOK bool) *Answer`: file JSON → Answer and the file is deleted; `action == "error"` → nil; missing file → `{Action:"timeout"}` if `exitOK` else nil.
  - `func ensureApp(bundled, installed string) error` (darwin): copy with `ditto` when `installed` is missing, or when both have the same `CFBundleIdentifier` but different `CFBundleVersion`. Different identifier → leave it alone, return nil. Read keys with `plutil -extract <key> raw -o - <app>/Contents/Info.plist`.
- Consumes: `Request`, `Answer`, `CacheDir` (Task 3).

- [ ] **Step 1: Write the failing tests.**
  - `TestReadAnswer` (all OSes): `{"action":"allow"}` → allow, file deleted; `{"action":"error"}` → nil; missing + exitOK → timeout; missing + !exitOK → nil; invalid JSON → nil.
  - `TestPluginRootFromEnv`: `CLAUDE_PLUGIN_ROOT=/x` → `/x`.
  - darwin only: `TestEnsureAppCopiesWhenMissing`, `TestEnsureAppUpdatesSameBundleID` (version 1 → 2 copied), `TestEnsureAppKeepsForeignBundleID` (installed has `com.example.other`, not overwritten). Build fake `.app` dirs with an `Info.plist` written by `plutil -create xml1` + `plutil -insert`.

- [ ] **Step 2: Run** `go test ./internal/notify/` — Expected: FAIL.

- [ ] **Step 3: Implement `darwin.go`** by porting `claude_notify/notifier.py` macOS parts:
  - Constants: installed app `~/Applications/Claude Notify.app`, bundled app `<PluginRoot>/macos/Claude Notify.app`, binary `Contents/MacOS/claude-notify-helper`, responses dir `<CacheDir>/responses`.
  - `Send` and `Ask` call `ensureApp` first; on error they log to stderr and go on with whatever is installed.
  - `Send`: run the installed helper binary directly with `--title --body [--sound claude-notify-<slug>.aiff | none]`, 15 s timeout; on failure, `osascript` `display notification` with `\` and `"` escaped (`notifier.py:186-200`).
  - `Ask`: `open -n -g -W -a <installed app> --args --title --body --actions --wait <secs> --id <uuid> [--sound]`. `signal.Notify` on SIGTERM/SIGINT/SIGHUP: read `<responses>/<id>.pid`, SIGTERM that pid, wait ≤ 3 s for `open`, then `os.Exit(0)`. Overall wait: `wait + 15 s`, then kill. Result: `readAnswer(<responses>/<id>.json, exit code == 0)`. UUID via `crypto/rand`.

- [ ] **Step 4: Run** `go test ./internal/notify/` — Expected: PASS. Real check: `echo '{"hook_event_name":"Stop","session_id":"t","cwd":"/tmp/demo","last_assistant_message":"Test."}' | CLAUDE_PLUGIN_ROOT=$PWD go run ./cmd/claude-notify hook` shows a notification (after Task 6 has produced `macos/`; until then with today's installed app).

- [ ] **Step 5: Commit** — "Add the macOS notifier with the app install step".

### Task 5: Windows and Linux notifiers

**Files:**
- Create: `internal/notify/windows.go`, `internal/notify/windows_request.go`, `internal/notify/windows_request_test.go`, `internal/notify/linux.go`, `internal/notify/linux_test.go`, `internal/winproc/winproc_windows.go`, `internal/winproc/ancestors.go`, `internal/winproc/ancestors_test.go`
- Modify: `internal/hook/platform_windows.go`, `internal/hook/waiter_windows.go` (real `winproc` calls)
- Move: `git mv claude_notify/windows/toast.ps1 windows/toast.ps1`, same for `ToastWaiter.cs` and `icon.png`

**Interfaces:**
- Produces:
  - `func toastRequest(r Request, actions string, wait time.Duration, id, response string, parent int, root, cache string, soundExists func(string) bool) map[string]any` (no build tag, so it is tested on every OS). Keys exactly as the header of `toast.ps1`: `title, body, sound, icon, actions, wait, id, response, parent, cache`. `sound` = `"none"` | `<root>/sounds/mp3/<slug>.mp3` when it exists | `"default"`.
  - `func notifySendArgs(r Request) []string` (no build tag): `--app-name=Claude`, `--urgency=critical|normal`, `--expire-time=10000`, title, body.
  - `winproc`: `Ancestors(pid uint32, procs map[uint32]Proc) []uint32` (pure, max 32, stops on loops), `type Proc struct{ Parent uint32; Exe string }`, and in the windows-only file `Processes() map[uint32]Proc`, `ForegroundApp() (pid uint32, exe string, procs map[uint32]Proc, err error)`, `StartedAt(pid uint32) (time.Time, bool)`, `Terminate(pid uint32)`.

- [ ] **Step 1: Write the failing tests.**
  - `TestToastRequestPlain`: no actions → `actions ""`, sound path under `<root>/sounds/mp3/done.mp3`, `icon` = `<root>/windows/icon.png`.
  - `TestToastRequestMissingSoundIsDefault`, `TestToastRequestPermissionWait` (`wait` 20, `parent` set, `response` set).
  - `TestNotifySendArgs`: Urgent → `--urgency=critical`.
  - `TestAncestors`: chain 4→3→2→1; loop 2→3→2 stops; unknown pid → empty.

- [ ] **Step 2: Run** `go test ./internal/...` — Expected: FAIL.

- [ ] **Step 3: Implement.** `windows.go` ports `_send_windows_notification`, `_ask_windows`, `_start_windows_toast` (`notifier.py:228-260`, `141-163`): JSON in `CLAUDE_NOTIFY_REQUEST`, `SysProcAttr.CreationFlags` per Global Constraints, `Send` returns after `Start` without waiting, `Ask` waits `wait + 15 s` then kills, and reads the answer with `readAnswer`. `winproc_windows.go` uses `golang.org/x/sys/windows` (`CreateToolhelp32Snapshot`, `Process32First/Next`, `GetForegroundWindow`, `GetWindowThreadProcessId`, `OpenProcess`, `GetProcessTimes`, `TerminateProcess(h, 0)`). `linux.go`: `Send` runs `notify-send` with `notifySendArgs`; `Ask` returns nil.

- [ ] **Step 4: Run** `make test`, `GOOS=windows GOARCH=amd64 go vet ./...`, `GOOS=windows GOARCH=arm64 go build ./...`, `GOOS=linux go vet ./...` — Expected: all pass.

- [ ] **Step 5: Commit** — "Add the Windows and Linux notifiers".

### Task 6: Plugin packaging, dispatcher and dist build

**Files:**
- Create: `.claude-plugin/marketplace.json`, `.claude-plugin/plugin.json`, `hooks/hooks.json`, `bin/claude-notify`, `scripts/test-dispatcher.sh`
- Modify: `macos-helper/build.sh`, `Makefile` (`dist` target)
- Generated and committed: `bin/darwin/claude-notify`, `bin/linux-amd64/claude-notify`, `bin/linux-arm64/claude-notify`, `bin/windows-amd64/claude-notify.exe`, `bin/windows-arm64/claude-notify.exe`, `macos/Claude Notify.app`

**Interfaces:**
- Consumes: the `hook` command from Task 1; `PluginRoot` layout from Task 4 (`<root>/macos/Claude Notify.app`, `<root>/windows/`, `<root>/sounds/mp3/`).

- [ ] **Step 1: Write the failing dispatcher test** `scripts/test-dispatcher.sh`: creates a temp plugin root whose path contains a space, puts fake binaries (`#!/bin/sh` printing `$0` and `"$@"` one per line) under `bin/darwin/`, `bin/linux-amd64/`, `bin/linux-arm64/`, copies `bin/claude-notify`, runs it with arguments `hook --event-type "Stop now"`, and asserts the fake for the current `uname` ran with the three arguments intact. A second case removes the fake binary and asserts exit code 0 with no output.

- [ ] **Step 2: Run** `sh scripts/test-dispatcher.sh` — Expected: FAIL (no dispatcher).

- [ ] **Step 3: Write `bin/claude-notify`** (POSIX sh, `chmod +x`): root = directory of `$0`; `uname -s` `Darwin` → `darwin/claude-notify`; `Linux` → `linux-amd64` or `linux-arm64` by `uname -m` (`aarch64|arm64` → arm64); `MINGW*|MSYS*|CYGWIN*` → `windows-arm64/claude-notify.exe` when `PROCESSOR_ARCHITECTURE` or `PROCESSOR_ARCHITEW6432` is `ARM64`, else `windows-amd64`. Binary present → `exec "$bin" "$@"`; otherwise `exit 0`.

- [ ] **Step 4: Run** `sh scripts/test-dispatcher.sh` — Expected: PASS.

- [ ] **Step 5: Write the manifests.** `plugin.json`: `{"name":"claude-notify","version":"0.2.0","description":"...","author":{"name":"Francesco Fontanesi"}}`. `marketplace.json`: `{"name":"awesome-claude-notifier","owner":{"name":"Francesco Fontanesi"},"plugins":[{"name":"claude-notify","source":"./","description":"..."}]}`. `hooks/hooks.json`: the four hooks of `examples/claude-settings.json` (same events, timeouts, `asyncRewake`, `--reply-wait 3540`), each command `"\"${CLAUDE_PLUGIN_ROOT}/bin/claude-notify\" hook --event-type <Event> [...]"`. No `preferredNotifChannel` (plugins can't set it).

- [ ] **Step 6: Change `macos-helper/build.sh`:** output `<repo>/macos/Claude Notify.app`; `swiftc -O -target arm64-apple-macos13` and `-target x86_64-apple-macos13`, then `lipo -create`; `CFBundleVersion` and `CFBundleShortVersionString` = `version` read from `.claude-plugin/plugin.json` with `plutil -extract version raw -o -`; `afconvert -f AIFF -d BEI16` of each `sounds/mp3/<slug>.mp3` into `Contents/Resources/claude-notify-<slug>.aiff`; then `codesign --force --sign -`.

- [ ] **Step 7: Add `make dist`:** the five Go binaries with the flags in Global Constraints (darwin: arm64 + amd64 merged with `lipo -create -output bin/darwin/claude-notify`), then `macos-helper/build.sh`. Run `make dist`, then `make dist` again and check `git status --porcelain bin/` shows nothing new the second time (reproducible).

- [ ] **Step 8: Validate** `claude plugin validate .` — Expected: no errors.

- [ ] **Step 9: Commit** — `git add .claude-plugin hooks bin scripts macos macos-helper/build.sh Makefile` — "Package claude-notify as a Claude Code plugin".

### Task 7: macOS spikes S2, S3, S4 with a real plugin install

**Files:**
- Modify: `docs/superpowers/specs/2026-10-29-go-plugin-rewrite-design.md` (spike results table)

- [ ] **Step 1: Push** `git push -u origin feat/go-rewrite`.
- [ ] **Step 2: Install the test plugin.** Add the marketplace from the branch (`claude plugin marketplace add franciuto/awesome-claude-notifier#feat/go-rewrite`; if refs aren't accepted, `claude plugin marketplace add <path of a fresh clone of the branch>` and record that S2 used a local clone). Then `claude plugin install notifier@awesome-claude-notifier`. Record `xattr -lr` of the cached `macos/` and `ls -l` of `bin/`.
- [ ] **Step 3: S2.** Rename the existing `~/Applications/Claude Notify.app` to `~/Applications/Claude Notify.app.bak`. Pipe a `Stop` event into the cached `bin/claude-notify hook`. Pass: the app is copied to `~/Applications`, the notification shows with the yellow icon, and in a live session Allow, Deny and Reply all reach Claude.
- [ ] **Step 4: S3.** Pass: the Done sound plays with `~/Library/Sounds/claude-notify-*.aiff` moved away. If not: add the copy of `Contents/Resources/*.aiff` to `~/Library/Sounds` to `ensureApp` (with a test), rebuild, re-check.
- [ ] **Step 5: S4.** In a session with only the plugin hooks active (old hooks commented out of `settings.json`, backed up first), reply to a Done notification. Pass: the session wakes with the text.
- [ ] **Step 6: Apply fallbacks** from the spec for any failed spike, with tests, before going on.
- [ ] **Step 7: Clean up:** uninstall the test plugin and marketplace, restore `settings.json` and the `.bak` app if the user wants to keep their old setup until release.
- [ ] **Step 8: Commit** the results table — "Record the macOS spike results".

### Task 8: Remove Python, docs, CI

**Files:**
- Delete: `claude_notify/`, `pyproject.toml`, `tests/`, `sounds/install.sh`, `sounds/install.ps1`, `examples/`
- Create: `INSTALL-PROMPT.md`, `CHANGELOG.md`, `.github/workflows/ci.yml`
- Modify: `README.md`, `CLAUDE.md`, `docs/how-it-works.md`, `sounds/generate.sh` (drop "install them with install.sh"), `sounds/player.html` (file label stays `claude-notify-<slug>.aiff`), `assets/icon/make_app_icon.py` docs in README (icon now `windows/icon.png`)

- [ ] **Step 1: Delete** the Python files with `git rm -r`. Run `make test` — Expected: PASS (nothing imports them).
- [ ] **Step 2: Write `INSTALL-PROMPT.md`** with the five steps of the spec section "The agent prompt". The test command for step 5 is `echo '{"hook_event_name":"Stop","session_id":"t","cwd":"/tmp/demo","last_assistant_message":"Test."}' | "<plugin root>/bin/claude-notify" hook`, where the agent finds the plugin root under `~/.claude/plugins/cache/awesome-claude-notifier/claude-notify/`.
- [ ] **Step 3: Rewrite the README install part:** a short "Upgrading from the pip version" note, then the two `/plugin` commands, a pointer to `INSTALL-PROMPT.md`, then per-OS manual notes (macOS: allow notifications, Alerts style; Windows: Git for Windows if S1 says so; Linux: `notify-send` and plain notifications only), `preferredNotifChannel`. Add a "Releasing" section for maintainers: bump `version`, `make dist`, `CHANGELOG.md`, PR to `master`, optional tag.
- [ ] **Step 4: Update `CLAUDE.md`** (map, commands `make test`/`make dist`/`swiftc -typecheck`, contract paths, mistakes table: add "Go panic or `flag` default → exit 2" and "Committing binaries without a version bump") and `docs/how-it-works.md` (Go binary, dispatcher, plugin hooks, app copy step, sounds from the plugin folder on Windows, Linux).
- [ ] **Step 5: Write `CHANGELOG.md`** (Keep a Changelog), `## [0.2.0] - unreleased`: Changed (Go, plugin install), Removed (Python package, `send`, `check`, sound installers).
- [ ] **Step 6: Write `.github/workflows/ci.yml`:** jobs `test` (matrix ubuntu/macos/windows: `gofmt -l .` empty, `go vet ./...`, `go test ./...`), `dispatcher` (ubuntu + macos: `sh scripts/test-dispatcher.sh`), `swift` (macos: `swiftc -typecheck macos-helper/main.swift`), `dist` (macos; only on PRs into `master` or when `.claude-plugin/plugin.json` changes: `make dist` without the app step, then `git diff --exit-code bin/`). Expose the Go-only part as `make dist-go` so the job can call it.
- [ ] **Step 7: Run** `make test && sh scripts/test-dispatcher.sh` — Expected: PASS.
- [ ] **Step 8: Commit** — "Remove the Python package and document the plugin install". Push.

### Task 9: Cross-platform checks and pull request

- [ ] **Step 1: Windows S1 + manual check, on x64 and on ARM64 (owner runs it, guided):** install through `/plugin install` from the branch; with Git Bash: plain toast with sound, Allow, Deny, Reply waking the session, a pending Reply withdrawn by typing, body click bringing the terminal forward, and the toast going away when Claude Code kills the hook. Then with Git Bash hidden from Claude Code: record the exit code (must not be 2). Record results in the spec; if the PowerShell-only case fails, state Git for Windows as a prerequisite in README and `INSTALL-PROMPT.md`.
- [ ] **Step 2: macOS full manual check** of the same list.
- [ ] **Step 3: Linux plain-notification check** (owner's machine or a desktop VM): `notify-send` present → notification shown; absent → hook exits 0.
- [ ] **Step 4: Commit** the results and any doc changes — "Record the cross-platform check results". Push.
- [ ] **Step 5: Open the PR** `feat/go-rewrite` → `dev` with `gh pr create`, body summarizing the change, the spike and manual results,, with no attribution line. Merging, the release to `master` and the `v0.2.0` tag are the owner's.

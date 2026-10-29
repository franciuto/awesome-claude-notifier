<p align="center">
  <img src="assets/banner.png" alt="Claude Code Notification Helper" width="720">
</p>

# awesome-claude-notifier

Claude Code works for minutes at a time, and you probably don't sit and watch it. This sends you a macOS, Windows or Linux notification when something happens: which session it was, what happened, and a sound you learn to recognize. When Claude asks to run a tool or finishes a task, you can answer from the notification itself.

## What a notification looks like

```
Refactor auth module · Done
I moved the token check into the middleware and added two tests for expired tokens.
```

The first line is the session name, the one Claude shows in its session list, followed by what happened. The second line is the start of Claude's answer, so most of the time you know whether to go back to the terminal without opening it.

Every event falls into one of six groups, and each group has its own sound:

| Group | When |
|---|---|
| Needs you | Claude wants permission or is waiting for an answer |
| Done | Claude finished |
| Paused | Claude finished, but something is still running in the background |
| Error | The turn failed, or auto mode blocked an action |
| Limit | You hit a usage limit, or it reset |
| Info | Anything else, like signing in |

Open `sounds/player.html` in a browser to hear them.

## Answering from the notification

Two notifications have controls.

When Claude asks to use a tool, the notification shows **Allow** and **Deny**. If you don't answer within 20 seconds it goes away and the usual prompt appears in the terminal. If the terminal is already the app in front, you get the prompt straight away and no notification.

When Claude finishes, the notification has a **Reply** field. What you type goes to that session and Claude picks it up immediately. If you go back and type in the terminal instead, the notification is withdrawn.

Both only work on macOS and Windows, in terminal sessions. The Claude desktop app has its own buttons for this, so there you get plain notifications.

## Install

**Upgrading from the pip version?** Remove the old setup first, or every notification arrives twice. Delete the `claude-notify hook` entries from the `hooks` in `~/.claude/settings.json`, then run `pip uninstall claude-notify`. You can also delete the sounds in `%LOCALAPPDATA%\claude-notify\sounds` on Windows. The `send` and `check` commands of the old package are gone.

It is a Claude Code plugin. Inside Claude Code, run:

```
/plugin marketplace add franciuto/awesome-claude-notifier
/plugin install notifier@awesome-claude-notifier
```

From a shell, the same two steps are `claude plugin marketplace add franciuto/awesome-claude-notifier` and `claude plugin install notifier@awesome-claude-notifier`.

Then set `"preferredNotifChannel": "notifications_disabled"` in `~/.claude/settings.json` (Windows: `%USERPROFILE%\.claude\settings.json`), so Claude Code's own notifications don't repeat these. A plugin can't set it for you.

Start a new `claude` session and you're set. There is nothing to build, no Python, and no hooks to copy: the plugin ships its own hooks and prebuilt binaries.

If you'd rather have your agent do it, paste [INSTALL-PROMPT.md](INSTALL-PROMPT.md) into it.

### macOS

You need macOS 13 or later. The first notification copies the bundled `Claude Notify.app` into `~/Applications` and installs the sounds into `~/Library/Sounds`; macOS can't play sounds from inside the app. Allow notifications when macOS asks. In System Settings → Notifications → Claude Notify, set the style to **Alerts**, so notifications with buttons stay on screen until you answer.

### Windows

You need Windows 10 or 11 on x64 or ARM64, and [Git for Windows](https://git-scm.com/download/win): Claude Code runs the plugin hooks under Git Bash. The notifications are Windows toasts shown by a PowerShell script that ships with the plugin. The first one registers the app as "Claude Notify". In Settings → System → Notifications you can turn off Focus assist for it or change how long banners stay.

### Linux

You need `notify-send` (package `libnotify-bin` on Debian and Ubuntu, `libnotify` on Fedora and Arch). You get plain notifications only: no buttons, no Reply, no sounds. Permission prompts and finished tasks still show up in the terminal as usual.

## Make it yours

To change a sound, edit its description in `sounds/generate.sh` and run the script with the group name, for example `./sounds/generate.sh error`. It uses the [ElevenLabs](https://elevenlabs.io) sound effects API, so you need their CLI logged in, and `ffmpeg` for `sounds/soften.sh`, which gives every new sound a short fade-in and caps its loudness. Then run `make dist` and bump `version` in `.claude-plugin/plugin.json`.

To change the icon, run `python3 assets/icon/make_app_icon.py your-image.png assets/icon/app-icon.png` with any square image. On Windows the icon is `windows/icon.png`: replace that file. On macOS `macos-helper/build.sh` builds `assets/icon/app-icon.png` into the app. In a fork, run `make dist` and bump the version so macOS drops its cached icon.

The notification texts are in `internal/hook/describe.go`.

For how the pieces fit together, see [docs/how-it-works.md](docs/how-it-works.md).

## Releasing

For maintainers. Binaries and the macOS app are committed, and users only get an update when `version` changes.

1. Bump `version` in `.claude-plugin/plugin.json`.
2. Run `make dist`. It builds the five Go binaries and the app (macOS only, with the Xcode command line tools). Commit `bin/` and `macos/` together with the bump.
3. Add the version to [CHANGELOG.md](CHANGELOG.md).
4. Open a pull request into `master`. CI rebuilds the Go binaries and fails if they differ from the committed ones.
5. Optionally tag the merge commit, for example `git tag v0.2.0`. No GitHub release is needed.

## Credits

Built on [claude-notify](https://github.com/jamez01/claude-notify) by jamez01, under the MIT license.

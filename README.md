<p align="center">
  <img src="assets/banner.png" alt="Claude Code Notification Helper" width="720">
</p>

# awesome-claude-notifier

Claude Code works for minutes at a time, and you probably don't sit and watch it. This sends you a macOS notification when something happens: which session it was, what happened, and a sound you learn to recognize. When Claude asks to run a tool or finishes a task, you can answer from the notification itself.

## What a notification looks like

```
Refactor auth module · Done
I moved the token check into the middleware and added two tests for expired tokens.
```

The first line is the session name, the one Claude shows in its session list, followed by what happened. The second line is the start of Claude's answer, so most of the time you know whether to go back to the terminal without opening it.

The second line follows the language you chat in: chat in Italian and you read "Chiede di usare Bash", chat in English and you read "Asking to use Bash". The group name in the title and the buttons stay in English either way. English and Italian are supported so far.

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

Both only work in terminal sessions. The Claude desktop app has its own buttons for this, so there you get plain notifications.

## Install

You need macOS 13 or later, Python 3.8 or later, and the Xcode command line tools.

1. Install the Python package:

   ```bash
   pip install .
   ```

2. Build the small app that shows the notifications. It goes into `~/Applications/Claude Notify.app`:

   ```bash
   ./macos-helper/build.sh
   ```

   Allow notifications when macOS asks. In System Settings → Notifications → Claude Notify, set the style to **Alerts**, so notifications with buttons stay on screen until you answer.

3. Install the sounds:

   ```bash
   ./sounds/install.sh
   ```

4. Copy the hooks from [`examples/claude-settings.json`](examples/claude-settings.json) into `~/.claude/settings.json`. If `claude-notify` isn't on your `PATH`, write its full path, for example `~/.venv/bin/claude-notify`.

Start a new `claude` session and you're set.

## Make it yours

To change a sound, edit its description in `sounds/generate.sh` and run the script with the group name, for example `./sounds/generate.sh error`. It uses the [ElevenLabs](https://elevenlabs.io) sound effects API, so you need their CLI logged in.

To change the icon, run `python assets/icon/make_app_icon.py your-image.png assets/icon/app-icon.png` with any square image, then build the app again.

The notification texts live in `claude_notify/i18n.py`, one block per language. To add a language, add a block of texts and a list of its common words there.

For how the pieces fit together, see [docs/how-it-works.md](docs/how-it-works.md).

## Credits

Built on [claude-notify](https://github.com/jamez01/claude-notify) by jamez01, under the MIT license.

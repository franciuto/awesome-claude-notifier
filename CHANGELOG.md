# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [0.2.1] - 2026-10-30

### Fixed

- Linux notifications show the plugin icon.

## [0.2.0] - 2026-10-29

### Added

- The fixed sentences in a notification follow the language you chat in. English and Italian are included; group names and buttons stay in English.
- README: an "Onboard your agent" button that leads to a ready install prompt with a copy button, plus sections on updating, uninstalling and fixing common problems.

### Changed

- The hook is a Go binary instead of a Python package. No Python is needed any more.
- Installation is a Claude Code plugin (`/plugin marketplace add franciuto/awesome-claude-notifier`, then `/plugin install notifier@awesome-claude-notifier`). The plugin ships the hooks, prebuilt binaries for macOS, Windows and Linux, and the macOS app.
- On macOS the first hook run copies the bundled app to `~/Applications` and installs the sounds into `~/Library/Sounds`.
- On Windows the sounds are played from the plugin folder.
- Linux gets plain notifications through `notify-send`.
- The app version follows the plugin version.
- The sounds fade in over 100 ms and are capped at -26 LUFS, so they no longer start with a jolt. `sounds/soften.sh` does this for new sounds.

### Removed

- The Python package and `pip install`. Upgrading: remove the old `claude-notify hook` entries from `~/.claude/settings.json` and run `pip uninstall claude-notify`.
- The `send` and `check` commands from the original project.
- The sound installers `sounds/install.sh` and `sounds/install.ps1`.
- `examples/claude-settings.json`: the plugin carries its own hooks.
- `INSTALL-PROMPT.md`: the prompt is now in the README.

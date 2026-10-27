# ha-notify

A small command-line tool that sends a push notification to your phone through [Home Assistant](https://www.home-assistant.io/). One static binary, no runtime dependencies. Notifications are defined as templates in a JSON file, with an icon, an image, action buttons and anything else the Home Assistant companion app supports.

It can also **ask** a quick question and print the answer. The notification gets Yes, No and Reply (text) buttons, and the command prints what you tapped or typed. That makes it handy for scripts and AI coding agents that want to ping you when a long task finishes or they need a decision.

## Install

Linux and macOS (amd64 and arm64):

```sh
curl -fsSL https://raw.githubusercontent.com/outsharked/ha-notify/master/install.sh | sh
```

This installs to `~/.local/bin` (override with `INSTALL_DIR=...`; pin a release with `VERSION=v0.1.0`) and verifies the download against the release's SHA-256 checksums. Prefer not to pipe to a shell? Download the binary and `checksums.txt` from the [releases page](https://github.com/outsharked/ha-notify/releases), or build it with Go: `go install github.com/outsharked/ha-notify@latest`.

## Setup

You need Home Assistant with the companion app signed in on your phone.

1. **Create a token.** In Home Assistant open your profile (bottom-left), then the **Security** tab, scroll to **Long-lived access tokens**, choose **Create token**, name it `ha-notify`, and copy it. It is shown only once.
2. **Find your phone's notify service.** In Home Assistant go to **Developer tools, Actions**, search for `notify.mobile_app_` and note the entry for your phone, for example `notify.mobile_app_my_phone`.
3. **Create the config** at `~/.config/ha-notify/config.json`. Start from [`ha-notify.example.json`](ha-notify.example.json) and set `url` (for example `http://homeassistant.local:8123`) and `default.target`.
4. **Save the token** in `~/.config/ha-notify/.env` as one line, `HA_TOKEN=your-token`, then `chmod 600` that file. The token is only ever read from the environment or this file, never from the config file or a command-line flag.
5. **Check it:** `ha-notify check`, then `ha-notify send finished --set task="It works"`.

Config lookup order: `--config`, `$HA_NOTIFY_CONFIG`, `./ha-notify.json`, then `<user config dir>/ha-notify/config.json`. `HA_URL` in the environment overrides `url`. The `.env` file is read from the same directory as the config.

## Use

```
ha-notify send finished --set task="Migration finished"
ha-notify ask needs-input --set task="Merge to main?" --timeout 10m
ha-notify send finished --title "Custom title" --message "Custom text"
ha-notify list      # template names
ha-notify check     # validate config and reach Home Assistant; sends nothing
ha-notify version
```

A template's `title`, `message` and `data` may contain `{placeholders}`, filled from `--set key=value`. `{{` and `}}` give literal braces. `{session}` defaults to the current directory's name. An unfilled placeholder is an error. `--title` and `--message` are sent exactly as typed.

`data` is passed to the companion app unchanged, so any field it supports works: `notification_icon` (an `mdi:` name, Android only), `color`, `image`, `clickAction`, `actions`, `channel`, `group`, `tag`, `importance`, `ttl`. iPhone ignores Android-only fields, so one config serves both. Reuse a `tag` and a new notification replaces the old one.

### Asking a question

`ask` sends the template with **Yes**, **No** and **Reply** (inline text box) buttons, waits on Home Assistant's websocket for the tap, prints the answer (`yes`, `no`, or the typed text) on stdout, and clears the notification. If nobody answers within `--timeout` (default 10m) it clears the notification, prints `no answer` on stderr and exits 3, so silence is never mistaken for yes. Each ask uses unique button ids, so concurrent asks do not see each other's answers. It is meant for quick one-off questions, not conversations.

Exit codes: 0 ok, 1 failure, 2 usage error, 3 no answer.

## Using it from Claude Code

[`skill/SKILL.md`](skill/SKILL.md) is a Claude Code skill that teaches a session to use ha-notify, but only when you ask ("ping me when you're done"). It also walks you through setup if the tool is not configured yet. Install it with:

```sh
mkdir -p ~/.claude/skills/ha-notify && curl -fsSL https://raw.githubusercontent.com/outsharked/ha-notify/master/skill/SKILL.md -o ~/.claude/skills/ha-notify/SKILL.md
```

## Development

`mise run test`, `mise run build` (cross-compiles into `bin/`), `mise run install`. Push a tag like `v0.1.0` to build binaries and publish a release through GitHub Actions.

## License

MIT, see [LICENSE](LICENSE).

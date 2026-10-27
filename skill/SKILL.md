---
name: ha-notify
description: Send a push notification to the user's phone through Home Assistant using the ha-notify CLI, ask them a quick yes/no or short-text question, or set the tool up (install it, create the Home Assistant token, write the config). Use only when the user asks to be notified or pinged (for example "ping me when you're done", "let me know on my phone", "I'm stepping away, notify me if you get stuck") or asks to set up ha-notify. Never notify unprompted.
---

# ha-notify

`ha-notify` sends a push notification to the user's phone via Home Assistant.

## Opt-in only

Off by default. Do not notify unless the user has asked in this session, e.g. "ping me when you're done or stuck". Once asked, keep notifying for the rest of the session until they say to stop. A session that was never asked must not notify, and there are no hooks doing it for you.

## How

- Work finished: `ha-notify send finished --set task="<one short line: what finished>"`
- Blocked and need the user: `ha-notify send needs-input --set task="<one short line: what you need decided>"`
- Need a quick decision from the user: `ha-notify ask needs-input --set task="<the question, answerable by yes/no or a short reply>" --timeout 10m`. It blocks and prints `yes`, `no`, or the text the user typed. Run it in the background so you are woken when it exits. Exit code 3 means no answer within the timeout: treat that as unanswered, never as yes. Use it only for quick one-off questions; do not use it to hold a conversation. A follow-up `ask` is fine when the answer genuinely leaves a decision open, but do not chat with the user by notification, and keep it to a couple of rounds.
- `ha-notify list` shows the available templates.

Send one notification at the moment you stop, not progress updates. Keep the line short and specific, since it shows on a lock screen.

If the command fails (it prints the reason), say so in chat and carry on. Do not retry in a loop. `ha-notify check` diagnoses config and token problems.

## Setup (when the tool is missing or `ha-notify check` fails)

Walk the user through it step by step, one step at a time, and confirm each before moving on.

1. **Install the binary** if `command -v ha-notify` finds nothing (Linux and macOS):
   `curl -fsSL https://raw.githubusercontent.com/outsharked/ha-notify/master/install.sh | sh`
   It installs to `~/.local/bin`; make sure that is on PATH.
2. **Create a token** (the user does this in Home Assistant): profile (bottom-left), the **Security** tab, **Long-lived access tokens**, **Create token**, name it `ha-notify`, and copy it. It is shown only once.
3. **Find the phone's notify service**: in Home Assistant, **Developer tools, Actions**, search `notify.mobile_app_` and note the entry for their phone (for example `notify.mobile_app_my_phone`). Ask the user for it and for their Home Assistant URL (for example `http://homeassistant.local:8123`).
4. **Write the config.** Download the example, `curl -fsSL https://raw.githubusercontent.com/outsharked/ha-notify/master/ha-notify.example.json -o ~/.config/ha-notify/config.json` (create the directory first), then set `url` and `default.target` in it.
5. **Store the token without ever seeing it.** Never ask the user to paste the token into the chat. Create the file with a placeholder and lock it down: `printf 'HA_TOKEN=\n' > ~/.config/ha-notify/.env && chmod 600 ~/.config/ha-notify/.env`. Then ask the user to open that file in their own editor, paste the token after `HA_TOKEN=`, and save.
6. **Verify**: run `ha-notify check`, then `ha-notify send finished --set task="Setup works"` and ask the user to confirm the notification arrived. A 401 from check means the token is wrong or was not saved; have them create a new one.

Notes: Android shows the `notification_icon` (an `mdi:` name); iPhone ignores it, and one config works for both. The token is only read from the environment or the `.env` beside the config, never from the config file.

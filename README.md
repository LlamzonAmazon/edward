# EDwaRd

EDwaRd is a local attack-surface picture of this Mac. One Go process, running as the installing user, lists what that user can already see: processes, listeners, installed apps, and startup items. It writes those facts to a JSON store, serves a page at [http://127.0.0.1:43393](http://127.0.0.1:43393), and, when asked, an agent reads that store.

It is not an antivirus. It has no kernel sensor, no signatures, no blocking, and no cloud. After install it makes no outbound connection.

A program that is malicious and does not match a playbook rule gets no callout. It can still appear as a process, an app, or a startup item. The callout is not a malware verdict.

## What you see

Eight sources around the host. Each hub shows how many items were collected. The graph draws raised and asked findings as callouts. Open a hub for the full list; that list is still in the store.

Drag to pan. Scroll to pan. Pinch, or Ctrl/Cmd and scroll, to zoom. Double-click empty space to reset.

Each callout has two readings: why it might matter, and why it might be fine. You decide. Clear it, ask an agent about it, or leave it raised.

One example: `node` listening on `0.0.0.0:3000` for 14 hours, started from Terminal.

- Might matter: anything on this network can try to connect. You may have meant this only for this machine.
- Might be fine: a common way to run a dev server and then forget it.

## How it works

Collectors run on a timer, as you, with fixed argument lists and timeouts. No shell. Command lines are redacted before they are stored (tokens, bearer values, `AKIA…` keys). Processes are grouped by name and path, so twenty copies of the same binary are one row. The process list itself is still used for listeners and hidden-path checks.

A six-rule playbook in Go evaluates each snapshot. The same snapshot produces the same findings.

1. Open to your network: listening on an address that is not loopback.
2. Unknown connection: outbound to a raw IP, on a port other than 22, 53, 80, or 443.
3. Running from a hidden folder: a path segment that starts with `.`.
4. Unsigned app: a `.app` bundle that `codesign` reports as unsigned or ad-hoc. Homebrew packages that are not app bundles are not this rule.
5. New login item: a startup item that was not on the first successful baseline.
6. Disguised file: a name with two extensions whose last one looks executable, and that was not on the first successful files baseline.

The page polls `GET /api/store`. It does not scan the machine. The agent does not scan either. Until you ask, the agent is told only that EDwaRd exists, that the page is on port 43393, and where the skill is. When you ask something like "anything sketchy on my laptop?", the agent reads the store and answers "EDwaRd has this, this, and that."

Notes you write for the agent are in the store and are read first.

## What it collects

As the installing user:

| Source | When | What |
| --- | --- | --- |
| Running programs | every few seconds | name, path, parent, uptime, for this user |
| Network | a short interval | TCP/UDP listeners and outbound sockets, with program name |
| Apps | every 10 minutes | `/Applications`, `~/Applications`, Homebrew prefix; version and signature |
| Starts at login | every 10 minutes | user LaunchAgents and login items |
| New files | every minute, off by default | names, sizes, times in Downloads, Desktop, and `/tmp`. Not contents. Turning it on triggers the macOS Files and Folders prompt |

Firewall, device policy, and logs are on the picture as empty shelves. This agent cannot see them.

Never collected: environment variables, SSH private keys, keychains, browser profiles, mail, documents, password-manager vaults, `/etc/shadow`, packet contents, kernel telemetry.

## Install

This build is macOS. You need Node (for the page) and Go (for the process).

```bash
make mock
make app
sh install.sh
```

Read [install.sh](install.sh) first. The script copies a local `EDwaRd.app` (from `EDWARD_BUNDLE` or `./EDwaRd.app`) into `~/Library/Application Support/EDwaRd`, writes a user LaunchAgent (`com.edward.agent`), links `~/.local/bin/edward`, and opens the page. No sudo, no password, no token. There is no checksum file in this tree, so the script does not download a binary.

If `127.0.0.1:43393` is already taken, install stops. It does not pick another port.

On a machine with no browser:

```bash
ssh -L 43393:127.0.0.1:43393 user@host
```

The first-run page can write a skill for the agents you pick (Cursor, Claude Code, Codex, OpenClaw). Default is to ask first. Skip leaves the page working and writes no agent file.

If you accept, it may write:

- Cursor: `~/.cursor/skills/edward/SKILL.md` only
- Claude Code: `~/.claude/CLAUDE.md` (sentinel block) and `~/.claude/skills/edward/SKILL.md` <!-- repo-leak:ok EDwaRd skill install path -->
- Codex: `~/.codex/AGENTS.md` (sentinel block) and `~/.codex/skills/edward/SKILL.md`
- OpenClaw: `~/.openclaw/skills/edward/SKILL.md` only

It does not write project `CLAUDE.md`, project `AGENTS.md`, or `.cursor/rules`.

Later, when a release and a checksum file exist, the same script is:

```bash
curl -fsSL https://raw.githubusercontent.com/OWNER/edward/main/install.sh | sh
```

Replace OWNER with the GitHub account that hosts this repo. Windows will use `install.ps1`. That script is not in this build.

## Pause and uninstall

Pause is on the page (Looking / Paused). Pause stops collectors. Resume runs them.

```bash
sh uninstall.sh
```

Read [uninstall.sh](uninstall.sh) first. Uninstall stops EDwaRd's process by binary path, removes the LaunchAgent and plist, deletes `~/Library/Application Support/EDwaRd` (the store and notes), removes `~/.local/bin/edward`, removes the sentinel blocks from `~/.claude/CLAUDE.md` and `~/.codex/AGENTS.md` only, and deletes the `edward` skill directories. Running it again succeeds. <!-- repo-leak:ok EDwaRd uninstall paths -->

It does not touch: project files, the rest of those two agent files, other skills, Cursor settings, OpenClaw workspace files, other LaunchAgents, other apps, other processes, the firewall, chat transcripts, or macOS privacy grants. Full Disk Access and the Downloads / Desktop prompt stay in System Settings until you remove them.

Check the machine is clean:

```bash
launchctl print "gui/$(id -u)/com.edward.agent"
test ! -e ~/Library/LaunchAgents/com.edward.agent.plist
test ! -e "$HOME/Library/Application Support/EDwaRd"
test ! -e ~/.local/bin/edward
test ! -d ~/.cursor/skills/edward
test ! -d ~/.claude/skills/edward  # repo-leak:ok EDwaRd skill path
test ! -d ~/.codex/skills/edward
test ! -d ~/.openclaw/skills/edward
grep -n "edward:start\|edward:end" ~/.claude/CLAUDE.md ~/.codex/AGENTS.md  # repo-leak:ok EDwaRd sentinel
curl -sf --max-time 2 http://127.0.0.1:43393/ ; echo "page exit $?"
lsof -nP -iTCP:43393 -sTCP:LISTEN
```

`launchctl print` does not find the service. `grep` prints nothing. `curl` fails to connect. `lsof` prints nothing, or it prints a program that is not EDwaRd.

Later:

```bash
curl -fsSL https://raw.githubusercontent.com/OWNER/edward/main/uninstall.sh | sh
```

## License

The license is not chosen yet.

## In this repo

- Collector, redaction, playbook, store, localhost server: `internal/`
- Store schema: `schema/store.schema.json`
- Page: `mock/`
- Skill template: `skill/edward/SKILL.md`
- User LaunchAgent: `install/`

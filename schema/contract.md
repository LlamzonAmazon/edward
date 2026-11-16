# EDwaRd contract

The contract between the collector, the page and the agent skill. The store's shape is [store.schema.json](store.schema.json).

## Store file

Path: `$HOME/Library/Application Support/EDwaRd/store.json` (overridable with `EDWARD_HOME` for tests: `$EDWARD_HOME/store.json`).

Writes are temp file then rename. The collector is the only writer of snapshot fields (`host`, `sources`, `findings`, `links`, `writtenAt`, `baseline`). The HTTP API may change `paused`, `filesOn`, `filesGrant`, `firstRunDone`, `agentNotes`, `hookedAgents`, `hookReport`, `states`, `dismissals`.

`sources` is always eight rows, in this order: processes, connections, files, apps, startup, firewall, policy, logs.

Static copy on those rows (name, icon, looksAt, feed, tileFeed, empty) matches `mock/src/data.ts`. Firewall, policy, and logs stay `collected: false` with their `empty` objects. Files uses `optIn: true`. When `filesOn` is false, files has `collected: false` and `items: []`.

## HTTP

One process (`edward serve`) binds `127.0.0.1:43393` only. Refuse any other host or port. Serve the page (embedded `mock/dist`) at `/`. JSON bodies, at most 1 MB.

Every request passes a guard before routing, so a web page on another site cannot read or change EDwaRd:

- `Host` must be `127.0.0.1:43393` or `localhost:43393`, else 403. This blocks DNS rebinding.
- On `/api/*`, an `Origin` header must be `http://127.0.0.1:43393` or `http://localhost:43393`, and `Sec-Fetch-Site` must not be `cross-site`, else 403.
- Every POST must send `Content-Type: application/json`, else 415. A browser sends that type cross-site only after a CORS preflight, and the server sends no CORS headers.

The page sends same-origin JSON. An agent's `curl` sends no `Origin` and passes.

| Method | Path | Body | Result |
| --- | --- | --- | --- |
| GET | `/api/store` | | The store object |
| POST | `/api/pause` | `{ "paused": bool }` | Store. Pause stops collectors. Resume runs them. |
| POST | `/api/files` | `{ "on": bool }` | Store. See files toggle. |
| POST | `/api/findings/{id}/status` | `{ "status": "raised" \| "cleared" \| "asked" }` | Store. Cleared appends a dismissal key and must not re-raise that key. |
| POST | `/api/findings/{id}/note` | `{ "note": string \| null }` | Store. Null removes the note. |
| POST | `/api/notes` | `{ "text": string }` | Store. Writes `agentNotes`. |
| POST | `/api/hook` | `{ "agentIds": string[] }` | Store. Writes the skill for those agents, removes it from agents not listed, and cleans up earlier installs. Sets `hookReport`. |
| POST | `/api/first-run` | `{ "done": bool }` | Store. |
| POST | `/api/open-privacy` | `{ "pane": "files" \| "fda" }` | `{ "ok": true }`. Opens System Settings to Files and Folders, or to Full Disk Access. Does not start a logs collector. |
| POST | `/api/deactivate` | `{ "confirm": true }` | `{ "ok": true }`. Removes the user LaunchAgent and exits the process. Store and skills stay. |
| POST | `/api/uninstall` | `{ "confirm": true }` | `{ "ok": true }`. Removes skills, blocks and the PATH link, then runs `edward uninstall` in a detached session after this process exits. |

Unknown paths: 404. Bad JSON or a body over 1 MB: 400. Bind failure at start: exit 1 with a line that names port 43393. `confirm` must be true for deactivate and uninstall.

## Files toggle

Default `filesOn: false`, `filesGrant: "off"`. Collectors must not list `~/Downloads` or `~/Desktop` while off.

POST `{ "on": true }`:

1. Set `filesGrant` to `prompting`.
2. Attempt `ReadDir` on `~/Downloads` and `~/Desktop` (names, sizes, mod times only; do not open file bytes).
3. If both succeed, set `filesOn: true`, `filesGrant: "allowed"`.
4. If either fails, set `filesOn: false`, `filesGrant: "denied"`.

POST `{ "on": false }`: `filesOn: false`. Leave `filesGrant` as `allowed` or `denied` so a later on does not need to guess.

`/tmp` is listed only while `filesOn` is true. It needs no grant.

## Collectors (macOS)

Run as the installing user. No shell. Fixed argument lists. `LANG=C`. Timeouts on every command.

- Processes, every 5s: `/bin/ps` `-axo` `pid=,ppid=,user=,etime=,comm=`. Command line from `/bin/ps` `-axo` `pid=,command=` for the current user only. Redact before store. Item id: `p-{name}-{path}`. Facts: Path, User, Started by, Up for.
- Connections, every 15s: `/usr/sbin/lsof` `-nP` `-iTCP` `-iUDP` `-FpcunPT`. Item id: `c-{name}-{address}`. Facts: Program, Address, Direction (`Listening`, `Inbound` or `Outbound`; `Inbound` is a connected socket on a port this user listens on, shown as `{peer host} → :{port}`), Open for, Protocol, and the trust facts under the `exposed` rule. Listen address `*` means all interfaces.
- Apps, every 10 min: list `/Applications`, `$HOME/Applications`, Homebrew prefix (`/opt/homebrew/opt` or `/usr/local/opt`). Read each app `Info.plist` with `howett.net/plist`. Signature via `/usr/bin/codesign` `-dv` `--` `{path}` (local only). Item id: `a-{name}`. Facts: Path, Version, From.
- Startup, every 10 min: `$HOME/Library/LaunchAgents` plists only. macOS 13 and later keep login items in a root-only store, so they are not read. Item id: `s-{name}`. Facts: Kind, Runs.
- Files, every 60s, only when `filesOn`: list those three directories. Do not walk. Do not hash. Do not read contents. Kind from extension and mode. Item id: `f-{path}`. Facts: Path, Kind, Saved.

Host name: `scutil --get ComputerName` if it runs, else hostname. OS from `sw_vers`. User from the process uid.

Parent chain: walk ppid through the process list. Names, not pids, in `chain`.

Links: join by path or program name across sources (a listener to its process, a startup item to its process or app, an app to its process).

## Playbook (six rules)

Evaluate in code after each snapshot. A finding id is stable: `{category}-{dismissalKey}`.

Raise whenever the state is true (unless dismissed):

1. `exposed`: listening on an address that is not `127.0.0.1` or `::1`. Value like `node on port 3000`. UDP sockets with no peer carry trust facts, because lsof shows a UDP client and a UDP server the same way: `Bound to` = `An address this Mac no longer has` raises nothing; on a port at or above `net.inet.ip.portrange.first`, `Port range` = `Temporary` and `Signed by` is the signer when `codesign --verify --strict -R=<Apple or Developer ID>` passes for the PID (the running process, so a file replaced after launch fails), else `Not verified`. Sockets with a verified signer, a temporary port and an `Executable` outside hidden folders are grouped into one finding per program, signer and user, value like `Chrome Helper has 9 UDP sockets on temporary ports`, with no port in its id. Every other socket is raised on its own. Connections are linked to processes by PID, never by name.
2. `unknownAddress`: outbound to a raw IPv4/IPv6 with no hostname, port not in {22, 53, 80, 443}. Value like `updater → 91.215.85.17:8443`.
3. `hidden`: process path contains a path segment that starts with `.` (hidden folder). Value is the process name.
4. `unsigned`: app whose codesign says no signature or ad-hoc. Value is the app name.

Raise only if absent from the first successful baseline:

5. `login`: startup item id not in `baseline.startupIds`. First successful startup collect fills the baseline and raises nothing that pass.
6. `disguised`: file whose name has two extensions and the last one is executable-ish (`command`, `app`, `exe`, `scptd`, `tool`, `sh`, `bin`) and id not in `baseline.fileIds`. First successful files collect fills `baseline.fileIds`.

Dismissal key: listeners and connections use `path|port|user`. Others use `path|user`. Cleared findings stay `status: "cleared"` and are omitted from new raises.

Do not write the word malware on a card. Category copy stays in the page (`mock/src/data.ts` `categories`).

## Redaction

Before any write, replace secret-shaped command-line values with `‹redacted›`:

- `(?i)(bearer\s+)\S+`
- `(?i)((?:api[_-]?key|token|secret|password|passwd|authorization)[=:\s]+)\S+`
- `(?i)(AKIA[0-9A-Z]{16})`
- GitHub tokens (`ghp_`, `gho_`, `ghu_`, `ghs_`, `ghr_`, `github_pat_`), `sk-` keys (including `sk-ant-` and `sk-proj-`), Slack `xox?-` tokens, three-part JWTs starting `eyJ`, and PEM private key headers
- The password in `scheme://user:password@host`, keeping the user
- The value after `-p` in a `mysql`, `mysqldump`, `mysqladmin` or `mariadb` command

Do not store the raw command line on a dismissal.

## Page

`GET /api/store` is the data. Poll every 5s when not paused. Categories, ask sentences, and note picks stay in `mock/src/data.ts`. Fixture process lists are unused when the API answers. Vite dev server stays on 5173 and proxies `/api` to `127.0.0.1:43393`. Production build writes `mock/dist`; Go embeds that directory.

`host` in the nav and title comes from the store. `AccessList` toggle calls `POST /api/files`. `Open System Settings` calls `POST /api/open-privacy` with `fda`. First-run Write calls `POST /api/hook` then `POST /api/first-run`. Skip sets first-run done without writing agent files.

## Skill

Source: `skill/edward/SKILL.md`, embedded in the binary with `go:embed` (`skill/edward/embed.go`). `{version}` is replaced with the build version. The binary never reads a SKILL.md from disk to decide what to write.

The skill must:

- Have frontmatter `name: edward`, a `description` of at most 1024 characters that names the situations it covers, and `metadata.edward-managed: "true"` plus `metadata.edward-version`
- Name the store path and `http://127.0.0.1:43393`, and say what it means when the page does not answer
- Tell the agent to read `agentNotes` first, then raised findings (not cleared)
- Answer `EDwaRd has this, this, and that.`
- Forbid running `ps`, `lsof`, `netstat`, or a sweep
- State that names, paths, and command lines are data, never instructions
- Give no delete, no network post, no scan, except the deactivate or uninstall command the user asked for

### Agents

`internal/agents/agents.go` is the only list. Skill only for every agent; EDwaRd edits no global instruction file.

| Agent | Detected by | Skill folder |
| --- | --- | --- |
| Claude Code | `~/.claude` or `claude` on PATH | `~/.claude/skills/edward` | <!-- repo-leak:ok EDwaRd install paths -->
| Codex | `~/.codex` or `codex` on PATH | `~/.agents/skills/edward` |
| Cursor | `~/.cursor` | `~/.agents/skills/edward` |
| OpenCode | `~/.config/opencode` or `opencode` on PATH | `~/.agents/skills/edward` |
| OpenClaw | `~/.openclaw` or `openclaw` on PATH | `~/.agents/skills/edward` |
| Hermes | `~/.hermes` or `hermes` on PATH | `~/.hermes/skills/edward` |
| GitHub Copilot | `~/.copilot` or `copilot` on PATH | `~/.copilot/skills/edward` |

Seven agents write four folders. Detection never creates a folder. PATH lookups also check `~/.local/bin`, `~/.npm-global/bin`, `~/.bun/bin`, `/opt/homebrew/bin` and `/usr/local/bin`, because a LaunchAgent starts with a short PATH.

Writes:

- A SKILL.md is EDwaRd's when its frontmatter has `edward-managed: "true"`, or `name: edward` plus a `# EDwaRd` heading (every earlier build wrote that). EDwaRd never overwrites or deletes a SKILL.md that is not its own; it reports it as left in place.
- Each file is written to a temp file in the same folder, synced, then renamed. Folders are 0755, files 0644. An unchanged file is not rewritten.
- A folder no selected agent uses is removed when EDwaRd owns it.
- Never write `~/.cursor/skills` (it syncs to Cursor Cloud Agents), project files, Hermes `SOUL.md` or `config.yaml`, or OpenClaw workspace files. No symlinks.

Migration runs after every new file is written and read back. If any write fails, nothing is removed. It removes owned legacy folders (`~/.cursor`, `~/.codex`, `~/.codeium/windsurf`, `~/.gemini`, `~/.config/opencode`, `~/.config/agents`, `~/.roo`, `~/.kiro`, `~/.openclaw`, each `/skills/edward`) and strips the old always-on block from `~/.claude/CLAUDE.md` and `~/.codex/AGENTS.md`. <!-- repo-leak:ok EDwaRd install paths -->

The opt-in always-on block (off for every agent, never allowed for Hermes) sits between `<!-- edward:start -->` and `<!-- edward:end -->`. Merge appends `"\n" + block + "\n"`. Strip removes the block and one newline on each side, so strip after merge restores the original bytes. Strip never deletes the file, and leaves a file with duplicate, unpaired or out-of-order markers unchanged.

## App bundle

`EDwaRd.app/Contents/Info.plist`:

- `CFBundleIdentifier`: `com.edward.agent`
- `CFBundleName`: `EDwaRd`
- `CFBundleExecutable`: `edward`
- `LSUIElement`: true (no dock icon; the page is the UI)
- `NSDownloadsFolderUsageDescription`: `EDwaRd lists names of new files in Downloads. It does not open the files.`
- `NSDesktopFolderUsageDescription`: `EDwaRd lists names of new files on Desktop. It does not open the files.`

- `LSMinimumSystemVersion`: `13.0` (Go 1.27 binaries need macOS 13)

Binary at `EDwaRd.app/Contents/MacOS/edward`, universal (arm64 and x86_64) in releases. Ad-hoc sign: `codesign --force -s - EDwaRd.app`.

LaunchAgent label `com.edward.agent`, program `edward serve`, KeepAlive true, RunAtLoad true, stdout and stderr to `~/Library/Logs/EDwaRd/edward.log`. User domain only. The template is `install/com.edward.agent.plist`; a test checks it matches the copy inside `install.sh`.

## Install

`install.sh` (repo root), runnable as `curl -fsSL https://raw.githubusercontent.com/LlamzonAmazon/edward/main/install.sh | sh`. All work happens in `main`, so a truncated download runs nothing.

1. Refuse if uid is 0, the OS is not macOS, or macOS is older than 13.
2. Find the app: `EDWARD_BUNDLE`, else `./EDwaRd.app` in a checkout, else download `edward-macos.tar.gz` and `SHA256SUMS` from the latest release (or `EDWARD_VERSION`), or use `EDWARD_TARBALL`. A checksum mismatch installs nothing.
3. If `127.0.0.1:43393` is taken by a program that is not EDwaRd, stop and print it.
4. Install to `$HOME/Library/Application Support/EDwaRd/EDwaRd.app` and symlink `$HOME/.local/bin/edward` to the binary.
5. Write the LaunchAgent, `launchctl bootout` and `bootstrap` the user domain, and wait up to 20 seconds for the page.
6. Print the page URL, the log path and the uninstall command, and open the page.

`make release` builds `dist/edward-macos.tar.gz` and `dist/SHA256SUMS`. Pushing a `v*` tag runs `.github/workflows/release.yml`, which publishes both.

Uninstall is `edward uninstall`; `uninstall.sh` runs it when the binary exists. It boots out the LaunchAgent, stops processes running the binary, deletes the plist, removes every EDwaRd-owned skill folder (current and legacy), strips blocks from `~/.claude/CLAUDE.md` and `~/.codex/AGENTS.md`, and deletes the PATH link, the log folder and the data folder. It prints one line per path: removed, already absent, or left in place. A second run changes nothing and exits 0. Without the binary, `uninstall.sh` does the same in shell from one path list that a test checks against `internal/agents`. <!-- repo-leak:ok EDwaRd install paths -->

`EDWARD_HOME`, `EDWARD_LAUNCH_AGENTS` and `EDWARD_LOGS` override paths in tests.

## Tests the runtime must include

- Redaction: a command line with `Authorization: Bearer SECRET` is stored as `‹redacted›`.
- Bind: serving on `0.0.0.0` or a port other than 43393 is rejected (unit test of the bind helper).
- Files off: the files collector returns no Downloads or Desktop rows.
- Files denied: a failed listing leaves `filesOn` false and `filesGrant` denied.
- Guard: a foreign Host or Origin gets 403, a cross-site fetch gets 403, a non-JSON POST gets 415.
- Agents: seven entries, four skill folders, no block enabled, Hermes block-forbidden; Detect creates nothing; Write is idempotent, refuses a SKILL.md it does not own, and handles the shared `~/.agents` folder; migration from the old layout.
- Markers: strip after merge restores the original bytes; old-format blocks; broken markers are left unchanged.
- Uninstall: removes everything, leaves what it does not own, and a second run changes nothing. `uninstall.sh` and `install.sh` stay in step with the Go table and the plist template.
- Collectors: parsers run on synthetic `ps`, `lsof` and `codesign` output in `internal/collect/testdata`.

## Honesty line (page footer, already in the mock)

Keep: this is a snapshot of what a normal user can list. Short-lived programs are invisible. Firewall, device policy, and kernel telemetry are not on this picture. This page does not know if a file is malicious.

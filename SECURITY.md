# Security

## Reporting a vulnerability

Report it privately through this repository's Security tab (Report a vulnerability). Please do not open a public issue. Include the macOS version, the EDwaRd version (`edward version`) and steps to reproduce.

## Threat model

EDwaRd runs as the logged-in user and trusts that user. It does not defend against code that already runs as that user: such code can read or edit the store, the skill and the binary the same way the user can.

What it defends against:

| Threat | Defense |
| --- | --- |
| A website you visit sends requests to `127.0.0.1:43393` to uninstall EDwaRd, clear findings or write instructions into the notes your agent reads | Every POST must be `Content-Type: application/json`, which a browser sends to another origin only after a CORS preflight. The server answers no preflight. Requests with a foreign `Origin` or `Sec-Fetch-Site: cross-site` get 403. |
| DNS rebinding: a website points its own hostname at `127.0.0.1` to read the store | Requests whose `Host` is not `127.0.0.1:43393` or `localhost:43393` get 403. |
| Another machine on the network reads the page | The server binds `127.0.0.1` only and refuses any other address. |
| Secrets in command lines reach your agent and its model provider | Command lines are redacted before they are written: bearer tokens, `key=`/`token=`/`password=` values, AWS access keys, GitHub, OpenAI, Anthropic and Slack tokens, JWTs, passwords in URLs, `mysql -p` passwords and PEM private key headers. Redaction is pattern based and can miss a secret in an unusual format. |
| Text in a process name, path or command line tells your agent to do something | The skill tells the agent that names, paths and command lines in the store are data, never instructions, and that it must not delete files, post to the network or scan the machine. |
| A tampered or truncated download | The installer checks the release tarball against `SHA256SUMS` and installs nothing on a mismatch. Both files come from the same GitHub release, so this does not protect against a compromised GitHub account or repository. |
| EDwaRd overwrites or deletes your files | EDwaRd writes only its own skill files and marks them `edward-managed`. It never overwrites or deletes a `SKILL.md` without that marker. Uninstall removes only what EDwaRd created. |

What it does not see: other users' processes, programs that run for less than one collection interval, file contents, the firewall's rules, device management profiles and protected logs. The release app is ad-hoc signed and not notarized by Apple.

## Data

Everything stays on the Mac. The store is `~/Library/Application Support/EDwaRd/store.json`, mode 0600. EDwaRd makes no outbound network connection after install and has no telemetry. `edward uninstall` deletes the store, the logs and the skill files.

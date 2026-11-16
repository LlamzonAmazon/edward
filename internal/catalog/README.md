# The catalog

The catalog is EDwaRd's set of checks drawn from public security sources. Each entry is one JSON file in `internal/catalog/entries/`. The files are compiled into the binary, so EDwaRd checks them without a network connection, and a new or changed entry reaches users in the next release. An item that matches one or more entries raises one finding in the "Matches a security report" category (`known` in the store), with a link to each entry's source.

## Kinds of entry

| Kind | Matches | Source | Expires |
| --- | --- | --- | --- |
| `version` | An app or macOS version inside a range NVD lists for a CVE in CISA's Known Exploited Vulnerabilities list | CISA KEV and NVD, written by the draft tool | No |
| `behavior` | What an ATT&CK technique leaves on a Mac, whatever program does it, such as a LaunchAgent that runs a program from `/tmp` | MITRE ATT&CK, written by hand | No |
| `indicator` | One malware family's fixed names or paths, such as a LaunchAgent label from a researcher's write-up | A vendor or researcher write-up, written by hand | Yes, required |

Behavior entries keep working when attackers change file names and servers. Indicators stop working when they do, so every indicator has an expiry date, and CI fails once that date passes until someone renews the entry from a fresh source or deletes it.

## What an entry can test

An entry tests the facts that EDwaRd's collectors write (`catalog.Facts` in `internal/catalog/catalog.go`):

| Source | Facts |
| --- | --- |
| `apps` | Path, Version, From, Signature, Bundle ID, Team ID, Kind (`Operating system` for macOS itself) |
| `startup` | Kind, Label, Runs, Environment (the names of the variables a LaunchAgent sets, never their values) |
| `processes` | Path, User, Started by, Up for |
| `connections` | Program, Address, Direction, Protocol |

A report that describes only things outside this list (a file hash, a domain, a LaunchDaemon, a Windows server) cannot become an entry until a collector reads that evidence.

## Entry format

```json
{
  "id": "beh-launchagent-temp-folder",
  "kind": "behavior",
  "title": "Starts a program from a temporary folder at login",
  "source": "startup",
  "when": [{ "fact": "Runs", "prefix": ["/tmp/", "/private/tmp/"] }],
  "technique": "T1543.001",
  "reports": [{ "name": "MITRE ATT&CK T1543.001 Launch Agent", "url": "https://attack.mitre.org/techniques/T1543/001/" }],
  "added": "2026-11-16",
  "tests": {
    "match": [{ "Runs": "/private/tmp/.u/agent" }],
    "noMatch": [{ "Runs": "/opt/homebrew/bin/node" }]
  }
}
```

- `when` lists conditions that must all hold. Each condition names one fact and exactly one operator: `is`, `prefix` or `contains` (lists of strings, any one may match) or `versions` (ranges with `from`, inclusive, and one of `below` or `through`). A fact the item does not have fails the condition.
- A value that starts with `~/` is under the user's home folder.
- `cve` is set on version entries. `technique` is an ATT&CK ID and is required on behavior and indicator entries.
- `expires` (a date) is required on indicators.
- `tests.match` and `tests.noMatch` are fact sets the entry must and must not match. Every entry needs at least one of each.

`go test ./internal/catalog/` checks every entry: the file name is the ID, every fact is one the collector writes, each condition has one operator, every report is a named https link, indicators have an unexpired date, and every test passes. `go test ./internal/collect/` fails if a collector stops writing a fact in the list.

## Adding entries from a write-up

1. Read the write-up and list what the threat leaves on a Mac: LaunchAgent labels and programs, install paths, bundle IDs, signing team IDs, environment variables.
2. Keep only what appears in the facts table above, and drop anything a common, legitimate app would also match.
3. Prefer a behavior entry when the write-up shows a technique that other families also use. Write an indicator for names and paths fixed to one family, with an expiry date about two years out.
4. Copy each value exactly from the write-up, and put the write-up's URL in `reports`.
5. Add a `match` test with the value from the write-up and a `noMatch` test with a similar, legitimate value.
6. Run `go test ./internal/catalog/`.

## Updating version entries

```bash
go run ./internal/catalog/draft -cache "$TMPDIR/edward-catalog"
```

The draft tool downloads CISA's KEV list, looks up each CVE for a product that can run on a Mac in NVD (one request every 6.5 seconds, the limit without an API key), and writes `kev-*.json` entries for the products in its table: Chrome, Edge, Firefox, Thunderbird, Safari, macOS, Acrobat and Acrobat Reader, VirtualBox, VMware Fusion, Docker Desktop and Homebrew's Git. It uses only NVD configurations that apply to macOS, because NVD sometimes lists a different fixed build per platform. It replaces earlier `kev-*.json` files and prints each CVE it could not turn into a version range. Responses are cached in the folder you pass, so a second run makes no requests.

## Limits

- Matching depends on the installed version and bundle ID. Chromium browsers other than Chrome and Edge (Brave, Arc, Opera) use their own version numbers, which NVD does not list.
- Firefox ESR shares Firefox's bundle ID, so an up-to-date ESR install can match a range for regular Firefox.
- The catalog is as current as the release that carries it.

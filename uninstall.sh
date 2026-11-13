#!/bin/sh
# EDwaRd uninstaller. No sudo, no password. Running twice is success.
#
# When the EDwaRd binary is still on disk, this runs `edward uninstall`, which
# does the whole job. The shell steps below are the fallback for a half-removed
# install whose binary is gone.
set -u

if [ "$(id -u)" -eq 0 ]; then
	echo "EDwaRd refuses to run as root. Run this as the user who installed it." >&2
	exit 1
fi

HOME=${HOME:-}
if [ -z "$HOME" ]; then
	echo "HOME is not set." >&2
	exit 1
fi

trim_slash() {
	p=$1
	while [ -n "$p" ] && [ "$p" != "${p%/}" ]; do
		p=${p%/}
	done
	printf '%s\n' "$p"
}

APPDIR=$(trim_slash "${EDWARD_HOME:-$HOME/Library/Application Support/EDwaRd}")
BIN="$APPDIR/EDwaRd.app/Contents/MacOS/edward"
LAUNCH_AGENTS=$(trim_slash "${EDWARD_LAUNCH_AGENTS:-$HOME/Library/LaunchAgents}")
PLIST="$LAUNCH_AGENTS/com.edward.agent.plist"
LOGS=$(trim_slash "${EDWARD_LOGS:-$HOME/Library/Logs/EDwaRd}")
LINK="$HOME/.local/bin/edward"
domain="gui/$(id -u)"

if [ -x "$BIN" ]; then
	"$BIN" uninstall
	exit $?
fi

# Skill folders, relative to HOME: the current agent table, then folders from
# earlier builds. internal/agents tests check this list against the Go table.
# skill-dirs:start
SKILL_DIRS="
.claude/skills/edward
.agents/skills/edward
.hermes/skills/edward
.copilot/skills/edward
.cursor/skills/edward
.codex/skills/edward
.codeium/windsurf/skills/edward
.gemini/skills/edward
.config/opencode/skills/edward
.config/agents/skills/edward
.roo/skills/edward
.kiro/skills/edward
.openclaw/skills/edward
"
# skill-dirs:end
# block-files:start
BLOCK_FILES="
.claude/CLAUDE.md
.codex/AGENTS.md
"
# block-files:end

status=0
report() {
	p=$2
	case $p in
	"$HOME"/*) p="~${p#"$HOME"}" ;;
	esac
	printf '%s %s\n' "$1" "$p"
}

remove_tree() {
	case $1 in
	"" | / | "$HOME" | "$HOME/")
		report "left in place" "$1 (refusing to delete)"
		status=1
		return
		;;
	esac
	if [ ! -e "$1" ] && [ ! -L "$1" ]; then
		report "already absent" "$1"
	elif rm -rf "$1" 2>/dev/null; then
		report "removed" "$1"
	else
		report "left in place" "$1"
		status=1
	fi
}

# An edward skill folder is EDwaRd's when its SKILL.md has the edward-managed
# marker, or the name and heading every earlier build wrote.
owned_skill() {
	f=$1/SKILL.md
	[ -f "$f" ] || return 1
	grep -q '^  edward-managed: "true"$' "$f" && return 0
	grep -q '^name: edward$' "$f" && grep -q '^# EDwaRd$' "$f"
}

remove_skill() {
	d=$HOME/$1
	if [ ! -e "$d" ] && [ ! -L "$d" ]; then
		report "already absent" "~/$1"
		return
	fi
	if [ -L "$d" ] || ! owned_skill "$d"; then
		report "left in place" "~/$1 (not EDwaRd's)"
		return
	fi
	rm -f "$d/SKILL.md"
	if rmdir "$d" 2>/dev/null; then
		report "removed" "~/$1"
	else
		report "left in place" "~/$1 (has other files)"
	fi
}

# Remove the marked block and the one blank line before it. Never deletes the
# file, and leaves it untouched when the markers are broken.
strip_block() {
	f=$HOME/$1
	if [ ! -f "$f" ]; then
		report "already absent" "~/$1"
		return
	fi
	awk '
		/<!-- edward:start -->/ { if (s) exit 2; s=NR }
		/<!-- edward:end -->/ { if (e) exit 2; e=NR }
		END { if (!s && !e) exit 1; if (s && e && s < e) exit 0; exit 2 }
	' "$f"
	case $? in
	1)
		report "already absent" "~/$1 block"
		return
		;;
	2)
		report "left in place" "~/$1 (broken edward markers)"
		return
		;;
	esac
	tmp=$(mktemp "${TMPDIR:-/tmp}/edward-strip.XXXXXX") || return
	awk '
		/<!-- edward:start -->/ { skip=1; if (held) { held=0 } ; next }
		/<!-- edward:end -->/ { skip=0; next }
		skip { next }
		{ if (held) print ""; held=0; if ($0 == "") { held=1; next } print }
		END { if (held) print "" }
	' "$f" >"$tmp" && cat "$tmp" >"$f"
	rm -f "$tmp"
	report "removed" "~/$1 block"
}

if [ "$(uname -s)" = Darwin ]; then
	loaded=0
	if [ -f "$PLIST" ] && launchctl bootout "$domain" "$PLIST" 2>/dev/null; then
		loaded=1
	fi
	if [ -z "${EDWARD_LAUNCH_AGENTS:-}" ] && launchctl print "$domain/com.edward.agent" >/dev/null 2>&1; then
		launchctl bootout "$domain/com.edward.agent" 2>/dev/null || true
		loaded=1
	fi
	if [ "$loaded" -eq 1 ]; then
		report "removed" "LaunchAgent com.edward.agent"
	else
		report "already absent" "LaunchAgent com.edward.agent"
	fi
fi

remove_tree "$PLIST"
for d in $SKILL_DIRS; do
	remove_skill "$d"
done
for f in $BLOCK_FILES; do
	strip_block "$f"
done

if [ -L "$LINK" ]; then
	case $(readlink "$LINK" 2>/dev/null) in
	*/EDwaRd.app/Contents/MacOS/edward) remove_tree "$LINK" ;;
	*) report "left in place" "$LINK (not EDwaRd's)" ;;
	esac
elif [ -e "$LINK" ]; then
	report "left in place" "$LINK (not EDwaRd's)"
else
	report "already absent" "$LINK"
fi
remove_tree "$LOGS"
remove_tree "$APPDIR"

echo "not touched: the rest of ~/.claude/CLAUDE.md and ~/.codex/AGENTS.md, other skills, project files, other LaunchAgents"
echo "not touched: macOS privacy grants. Remove an EDwaRd row in System Settings > Privacy & Security if one is there."
exit "$status"

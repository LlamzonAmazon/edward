#!/bin/sh
# EDwaRd uninstaller. No sudo, no password. Running twice is success.
set -u

if [ "$(id -u)" -eq 0 ]; then
	echo "EDwaRd refuses to run as root. Run this as the user who installed it." >&2
	exit 1
fi

status=0

trim_slash() {
	p=$1
	while [ -n "$p" ] && [ "$p" != "${p%/}" ]; do
		p=${p%/}
	done
	printf '%s\n' "$p"
}

report() {
	printf '%s %s\n' "$1" "$2"
}

HOME=${HOME:-}
if [ -z "$HOME" ]; then
	echo "HOME is not set." >&2
	exit 1
fi

if [ -n "${EDWARD_HOME:-}" ]; then
	APPDIR=$(trim_slash "$EDWARD_HOME")
else
	APPDIR="$HOME/Library/Application Support/EDwaRd"
fi
APP="$APPDIR/EDwaRd.app"
BIN="$APP/Contents/MacOS/edward"

if [ -n "${EDWARD_LAUNCH_AGENTS:-}" ]; then
	LAUNCH_AGENTS=$(trim_slash "$EDWARD_LAUNCH_AGENTS")
else
	LAUNCH_AGENTS="$HOME/Library/LaunchAgents"
fi
PLIST="$LAUNCH_AGENTS/com.edward.agent.plist"
LINK="$HOME/.local/bin/edward"
uid=$(id -u)
domain="gui/$uid"
os=$(uname -s)

stop_by_bin() {
	bin=$1
	if [ -z "$bin" ]; then
		return 0
	fi
	ps -axo pid=,command= 2>/dev/null | awk -v bin="$bin" -v self="$$" '
		{
			pid=$1
			sub(/^[[:space:]]*[0-9][0-9]*[[:space:]]*/, "")
			if (($0 == bin || index($0, bin " ") == 1) && pid != self) print pid
		}
	' | while IFS= read -r pid; do
		if [ -n "$pid" ]; then
			kill "$pid" 2>/dev/null || true
		fi
	done
	sleep 1
	ps -axo pid=,command= 2>/dev/null | awk -v bin="$bin" -v self="$$" '
		{
			pid=$1
			sub(/^[[:space:]]*[0-9][0-9]*[[:space:]]*/, "")
			if (($0 == bin || index($0, bin " ") == 1) && pid != self) print pid
		}
	' | while IFS= read -r pid; do
		if [ -n "$pid" ]; then
			kill -9 "$pid" 2>/dev/null || true
		fi
	done
}

bin_still_running() {
	bin=$1
	ps -axo pid=,command= 2>/dev/null | awk -v bin="$bin" '
		{
			sub(/^[[:space:]]*[0-9][0-9]*[[:space:]]*/, "")
			if ($0 == bin || index($0, bin " ") == 1) found=1
		}
		END { exit found ? 0 : 1 }
	'
}

remove_path() {
	path=$1
	if [ ! -e "$path" ] && [ ! -L "$path" ]; then
		report "already absent" "$path"
		return 0
	fi
	if [ -L "$path" ]; then
		if rm -f "$path" 2>/dev/null; then
			report "removed" "$path"
			return 0
		fi
		report "left in place" "$path"
		echo "Could not remove $path. Run as the user who installed it. This script does not use sudo." >&2
		status=1
		return 1
	fi
	if rm -rf "$path" 2>/dev/null; then
		report "removed" "$path"
		return 0
	fi
	report "left in place" "$path"
	echo "Could not remove $path. Run as the user who installed it. This script does not use sudo." >&2
	status=1
	return 1
}

strip_sentinels() {
	file=$1
	if [ ! -e "$file" ]; then
		report "already absent" "$file"
		return 0
	fi
	awk '
		/<!-- edward:start -->/ { if (s) exit 2; s=NR }
		/<!-- edward:end -->/ { if (e) exit 2; e=NR }
		END {
			if (!s && !e) exit 1
			if (s && e && s < e) exit 0
			exit 2
		}
	' "$file"
	rc=$?
	if [ "$rc" -eq 1 ]; then
		report "already absent" "$file"
		return 0
	fi
	if [ "$rc" -eq 2 ]; then
		report "left in place" "$file"
		echo "Broken edward sentinels in $file. Not rewriting." >&2
		return 0
	fi
	tmp=$file.edward-uninstall.$$
	if ! awk '
		BEGIN { skip=0 }
		/<!-- edward:start -->/ { skip=1; next }
		/<!-- edward:end -->/ { skip=0; next }
		skip { next }
		{ print }
	' "$file" >"$tmp"; then
		rm -f "$tmp"
		report "left in place" "$file"
		echo "Could not rewrite $file." >&2
		return 0
	fi
	if ! grep -q '[^[:space:]]' "$tmp"; then
		if rm -f "$file" "$tmp" 2>/dev/null; then
			report "removed" "$file"
			return 0
		fi
		rm -f "$tmp"
		report "left in place" "$file"
		echo "Could not remove $file. Run as the user who installed it. This script does not use sudo." >&2
		status=1
		return 1
	fi
	if mv "$tmp" "$file" 2>/dev/null; then
		report "removed" "$file"
		return 0
	fi
	rm -f "$tmp"
	report "left in place" "$file"
	echo "Could not rewrite $file. Run as the user who installed it. This script does not use sudo." >&2
	status=1
	return 1
}

# 1. Stop the program and remove the login item.
# When EDWARD_LAUNCH_AGENTS is set (tests), bootout only that plist, not the user-domain label.
if [ "$os" = Darwin ]; then
	unloaded=0
	if [ -f "$PLIST" ]; then
		if launchctl bootout "$domain" "$PLIST" 2>/dev/null; then
			unloaded=1
		fi
	fi
	if [ -z "${EDWARD_LAUNCH_AGENTS:-}" ]; then
		if launchctl print "$domain/com.edward.agent" >/dev/null 2>&1; then
			launchctl bootout "$domain/com.edward.agent" 2>/dev/null || true
			unloaded=1
		fi
	fi
	if [ "$unloaded" -eq 1 ]; then
		report "removed" "$domain/com.edward.agent"
	else
		report "already absent" "$domain/com.edward.agent"
	fi
else
	report "already absent" "$domain/com.edward.agent"
fi

if bin_still_running "$BIN"; then
	stop_by_bin "$BIN"
	if bin_still_running "$BIN"; then
		report "left in place" "process $BIN"
	else
		report "removed" "process $BIN"
	fi
else
	report "already absent" "process $BIN"
fi

remove_path "$PLIST" || true

# 2. Program files, store, notes.
remove_path "$APPDIR" || true
if [ -e "$APPDIR" ] || [ -L "$APPDIR" ]; then
	# Retry after a short wait in case the binary was still shutting down.
	sleep 1
	stop_by_bin "$BIN"
	remove_path "$APPDIR" || true
fi
if [ -L "$LINK" ]; then
	target=$(readlink "$LINK" 2>/dev/null || true)
	case "$target" in
	"$BIN" | */EDwaRd.app/Contents/MacOS/edward | "")
		remove_path "$LINK" || true
		;;
	*)
		report "left in place" "$LINK"
		;;
	esac
elif [ -e "$LINK" ]; then
	report "left in place" "$LINK"
else
	report "already absent" "$LINK"
fi

# 3. Sentinel blocks only.
strip_sentinels "$HOME/.claude/CLAUDE.md"
strip_sentinels "$HOME/.codex/AGENTS.md"

# 4. Skill directories (the edward dir only). Symlink: remove the link, not the target.
for d in \
	"$HOME/.cursor/skills/edward" \
	"$HOME/.claude/skills/edward" \
	"$HOME/.codex/skills/edward" \
	"$HOME/.openclaw/skills/edward"; do
	remove_path "$d" || true
done

# 5. Confirm the page on 43393 is gone. If our binary still holds it, kill and recheck.
if command -v lsof >/dev/null 2>&1 && lsof -nP -iTCP:43393 -sTCP:LISTEN 2>/dev/null | awk 'NR > 1 { found=1 } END { exit found ? 0 : 1 }'; then
	pid=$(lsof -nP -iTCP:43393 -sTCP:LISTEN -t 2>/dev/null | awk 'NR==1 { print; exit }') || pid=
	cmd=
	if [ -n "$pid" ]; then
		cmd=$(ps -p "$pid" -o command= 2>/dev/null || true)
	fi
	case "$cmd" in
	"$BIN" | "$BIN "* | *"/EDwaRd.app/Contents/MacOS/edward" | *"/EDwaRd.app/Contents/MacOS/edward "*)
		stop_by_bin "$BIN"
		if [ -n "$pid" ]; then
			kill "$pid" 2>/dev/null || true
			sleep 1
			kill -9 "$pid" 2>/dev/null || true
		fi
		if lsof -nP -iTCP:43393 -sTCP:LISTEN 2>/dev/null | awk 'NR > 1 { found=1 } END { exit found ? 0 : 1 }'; then
			report "left in place" "127.0.0.1:43393"
			echo "EDwaRd is still listening on 127.0.0.1:43393" >&2
			status=1
		else
			report "removed" "127.0.0.1:43393"
		fi
		;;
	*)
		report "left in place" "127.0.0.1:43393"
		echo "Another program has 127.0.0.1:43393 pid ${pid:-?} $cmd. Not killing it." >&2
		;;
	esac
else
	report "already absent" "127.0.0.1:43393"
fi

# 6. Final sweep + verify. Install artifacts must be gone.
for p in \
	"$PLIST" \
	"$APPDIR" \
	"$HOME/.cursor/skills/edward" \
	"$HOME/.claude/skills/edward" \
	"$HOME/.codex/skills/edward" \
	"$HOME/.openclaw/skills/edward"; do
	if [ -e "$p" ] || [ -L "$p" ]; then
		rm -rf "$p" 2>/dev/null || true
	fi
done
if [ -L "$LINK" ]; then
	target=$(readlink "$LINK" 2>/dev/null || true)
	case "$target" in
	"$BIN" | */EDwaRd.app/Contents/MacOS/edward | "")
		rm -f "$LINK" 2>/dev/null || true
		;;
	esac
fi

verify_absent() {
	path=$1
	if [ -e "$path" ] || [ -L "$path" ]; then
		report "left in place" "$path"
		echo "Uninstall left $path behind." >&2
		status=1
		return 1
	fi
	report "verified absent" "$path"
	return 0
}

verify_absent "$PLIST"
verify_absent "$APPDIR"
verify_absent "$HOME/.cursor/skills/edward"
verify_absent "$HOME/.claude/skills/edward"
verify_absent "$HOME/.codex/skills/edward"
verify_absent "$HOME/.openclaw/skills/edward"
if [ -L "$LINK" ]; then
	target=$(readlink "$LINK" 2>/dev/null || true)
	case "$target" in
	"$BIN" | */EDwaRd.app/Contents/MacOS/edward | "")
		report "left in place" "$LINK"
		echo "Uninstall left $LINK behind." >&2
		status=1
		;;
	*)
		report "left in place" "$LINK (not EDwaRd; not removed)"
		;;
	esac
elif [ -e "$LINK" ]; then
	report "left in place" "$LINK (not a symlink; not removed)"
else
	report "verified absent" "$LINK"
fi
if [ "$os" = Darwin ] && launchctl print "$domain/com.edward.agent" >/dev/null 2>&1; then
	report "left in place" "$domain/com.edward.agent"
	echo "LaunchAgent com.edward.agent is still loaded." >&2
	status=1
else
	report "verified absent" "$domain/com.edward.agent"
fi
for f in "$HOME/.claude/CLAUDE.md" "$HOME/.codex/AGENTS.md"; do
	if [ -f "$f" ] && grep -q "edward:start\|edward:end" "$f" 2>/dev/null; then
		report "left in place" "sentinels in $f"
		echo "Uninstall left edward sentinels in $f." >&2
		status=1
	fi
done

echo "not touched: project .cursor/rules, project CLAUDE.md, project AGENTS.md"
echo "not touched: the rest of ~/.claude/CLAUDE.md and ~/.codex/AGENTS.md"
echo "not touched: other agent files, other skills, Cursor settings, OpenClaw workspace AGENTS.md and USER.md"
echo "not touched: other LaunchAgents, other apps, other processes, the firewall"
echo "not touched: chat transcripts"
echo "not touched: macOS privacy grants (Full Disk Access, Downloads, Desktop). System Settings > Privacy & Security if an EDwaRd row remains."

if [ "$status" -ne 0 ]; then
	exit 1
fi
exit 0

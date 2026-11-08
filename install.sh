#!/bin/sh
# EDwaRd installer. macOS only in this build. No sudo, no password, no token.
set -eu

if [ "$(id -u)" -eq 0 ]; then
	echo "EDwaRd refuses to run as root. Run this as the user who will use it." >&2
	exit 1
fi

os=$(uname -s)
case "$os" in
Darwin) ;;
Linux)
	echo "This build installs on macOS only. Linux is not in this build." >&2
	exit 1
	;;
*)
	echo "This build installs on macOS only. $os is not in this build." >&2
	exit 1
	;;
esac

trim_slash() {
	p=$1
	while [ -n "$p" ] && [ "$p" != "${p%/}" ]; do
		p=${p%/}
	done
	printf '%s\n' "$p"
}

here=$(CDPATH= cd -- "$(dirname -- "$0")" 2>/dev/null && pwd) || here=.
if [ ! -f "$here/install/com.edward.agent.plist" ] && [ -f "./install/com.edward.agent.plist" ]; then
	here=$(pwd)
fi

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
TEMPLATE="$here/install/com.edward.agent.plist"
uid=$(id -u)
domain="gui/$uid"

port_in_use() {
	if command -v lsof >/dev/null 2>&1; then
		lsof -nP -iTCP:43393 -sTCP:LISTEN 2>/dev/null | awk 'NR > 1 { found=1 } END { exit found ? 0 : 1 }'
		return
	fi
	if command -v nc >/dev/null 2>&1; then
		nc -z 127.0.0.1 43393 >/dev/null 2>&1
		return
	fi
	return 1
}

listener_cmd() {
	pid=$(lsof -nP -iTCP:43393 -sTCP:LISTEN -t 2>/dev/null | awk 'NR==1 { print; exit }') || pid=
	if [ -z "$pid" ]; then
		return 0
	fi
	ps -p "$pid" -o command= 2>/dev/null || true
}

cmd_is_ours() {
	cmd=$1
	bin=$2
	case "$cmd" in
	"$bin" | "$bin "*) return 0 ;;
	esac
	return 1
}

bootout_agent() {
	if [ -f "$PLIST" ]; then
		launchctl bootout "$domain" "$PLIST" 2>/dev/null || true
	fi
	if [ -z "${EDWARD_LAUNCH_AGENTS:-}" ]; then
		launchctl bootout "$domain/com.edward.agent" 2>/dev/null || true
	fi
}

find_bundle() {
	if [ -n "${EDWARD_BUNDLE:-}" ]; then
		if [ ! -d "$EDWARD_BUNDLE" ]; then
			echo "EDWARD_BUNDLE is not a directory: $EDWARD_BUNDLE" >&2
			exit 1
		fi
		printf '%s\n' "$EDWARD_BUNDLE"
		return
	fi
	if [ -d "./EDwaRd.app" ]; then
		printf '%s\n' "$(CDPATH= cd -- "./EDwaRd.app" && pwd)"
		return
	fi
	if [ -d "$here/EDwaRd.app" ]; then
		printf '%s\n' "$here/EDwaRd.app"
		return
	fi
	if [ -f "$here/SHA256SUMS" ] || [ -f "$here/checksums.sha256" ] || [ -f "$here/EDwaRd.app.sha256" ]; then
		echo "A checksum file is here, but this build does not download a release. Run make app, then sh install.sh. Or set EDWARD_BUNDLE to a built .app." >&2
		exit 1
	fi
	echo "No EDwaRd.app next to this script or in the current directory. Run make app first, then sh install.sh. Or set EDWARD_BUNDLE to a built .app. This build does not download a binary (no checksum file)." >&2
	exit 1
}

if [ ! -f "$TEMPLATE" ]; then
	echo "Missing $TEMPLATE. Run this from the EDwaRd repo." >&2
	exit 1
fi

bundle=$(find_bundle)
if [ ! -f "$bundle/Contents/MacOS/edward" ]; then
	echo "The bundle has no Contents/MacOS/edward. Run make app." >&2
	exit 1
fi

if port_in_use; then
	cmd=$(listener_cmd)
	if ! cmd_is_ours "$cmd" "$BIN"; then
		echo "127.0.0.1:43393 is already in use. Stop that program, then run this again. EDwaRd does not pick another port." >&2
		if [ -n "$cmd" ]; then
			echo "Listener: $cmd" >&2
		fi
		exit 1
	fi
	bootout_agent
fi

mkdir -p "$APPDIR" "$LAUNCH_AGENTS" "$HOME/.local/bin"

rm -rf "$APP"
if command -v ditto >/dev/null 2>&1; then
	ditto "$bundle" "$APP"
else
	cp -R "$bundle" "$APP"
fi

if [ ! -f "$BIN" ]; then
	echo "The bundle has no Contents/MacOS/edward. Run make app." >&2
	exit 1
fi
chmod +x "$BIN" 2>/dev/null || true

ln -sfn "$BIN" "$LINK"

awk -v bin="$BIN" -v dir="$APPDIR" '
{
	gsub(/__BINARY__/, bin)
	gsub(/__DATADIR__/, dir)
	print
}
' "$TEMPLATE" >"$PLIST"

bootout_agent

if port_in_use; then
	cmd=$(listener_cmd)
	if ! cmd_is_ours "$cmd" "$BIN"; then
		echo "127.0.0.1:43393 is already in use. Stop that program, then run this again. EDwaRd does not pick another port." >&2
		if [ -n "$cmd" ]; then
			echo "Listener: $cmd" >&2
		fi
		exit 1
	fi
	# Ours is still up after bootout. Stop it by binary path, then recheck.
	ps -axo pid=,command= 2>/dev/null | while IFS= read -r line; do
		pid=$(printf '%s\n' "$line" | awk '{ print $1 }')
		rest=$(printf '%s\n' "$line" | sed 's/^[[:space:]]*[0-9][0-9]*[[:space:]]*//')
		case "$rest" in
		"$BIN" | "$BIN "*)
			if [ -n "$pid" ] && [ "$pid" -ne "$$" ]; then
				kill "$pid" 2>/dev/null || true
			fi
			;;
		esac
	done
	sleep 1
	if port_in_use; then
		echo "127.0.0.1:43393 is already in use. Stop that program, then run this again. EDwaRd does not pick another port." >&2
		exit 1
	fi
fi

if ! launchctl bootstrap "$domain" "$PLIST"; then
	echo "launchctl bootstrap failed for $PLIST" >&2
	exit 1
fi

echo "http://127.0.0.1:43393"
i=0
up=0
while [ "$i" -lt 20 ]; do
	if command -v curl >/dev/null 2>&1 && curl -sf --max-time 1 "http://127.0.0.1:43393/" >/dev/null 2>&1; then
		up=1
		break
	fi
	i=$((i + 1))
	sleep 1
done
if [ "$up" -ne 1 ]; then
	echo "Installed, but http://127.0.0.1:43393 did not answer within 20s. Check: launchctl print \"gui/$(id -u)/com.edward.agent\"" >&2
	exit 1
fi

if command -v open >/dev/null 2>&1; then
	open "http://127.0.0.1:43393" || true
fi

case ":$PATH:" in
*:"$HOME/.local/bin":*) ;;
*)
	echo "$HOME/.local/bin is not on PATH. The page still opens. Add that directory to PATH if you want the edward command."
	;;
esac

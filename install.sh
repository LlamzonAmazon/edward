#!/bin/sh
# EDwaRd installer for macOS. No sudo, no password, no account.
#
#   curl -fsSL https://raw.githubusercontent.com/LlamzonAmazon/edward/main/install.sh | sh
#
# Piped like that, it downloads the latest release and checks its SHA-256.
# Run as a file from a checkout after `make app` (sh install.sh), it
# installs ./EDwaRd.app instead. Piped, it always downloads.
#
#   EDWARD_VERSION=v1.0.0   install that release instead of the latest
#   EDWARD_TARBALL=path     install a local edward-macos.tar.gz (no download)
#   EDWARD_BUNDLE=path      install a built EDwaRd.app (no download)
#
# Everything runs inside main, so a download cut short runs nothing.
set -eu

REPO=LlamzonAmazon/edward
PORT=43393
PAGE=http://127.0.0.1:$PORT

die() {
	echo "EDwaRd: $*" >&2
	exit 1
}

trim_slash() {
	p=$1
	while [ -n "$p" ] && [ "$p" != "${p%/}" ]; do
		p=${p%/}
	done
	printf '%s\n' "$p"
}

xml_escape() {
	printf '%s' "$1" | sed -e 's/&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g'
}

check_platform() {
	if [ "$(id -u)" -eq 0 ]; then
		die "refusing to run as root. Run this as the user who will use EDwaRd."
	fi
	os=$(uname -s)
	if [ "$os" != Darwin ]; then
		die "this build installs on macOS only. $os is not supported."
	fi
	ver=$(sw_vers -productVersion 2>/dev/null || echo 0)
	if [ "${ver%%.*}" -lt 13 ] 2>/dev/null; then
		die "EDwaRd needs macOS 13 Ventura or later. This Mac runs $ver."
	fi
	[ -n "${HOME:-}" ] || die "HOME is not set."
}

port_in_use() {
	lsof -nP -iTCP:$PORT -sTCP:LISTEN >/dev/null 2>&1
}

listener_cmd() {
	pid=$(lsof -nP -iTCP:$PORT -sTCP:LISTEN -t 2>/dev/null | awk 'NR==1 { print; exit }') || pid=
	[ -n "$pid" ] && ps -p "$pid" -o command= 2>/dev/null || true
}

listener_is_edward() {
	case $(listener_cmd) in
	*/EDwaRd.app/Contents/MacOS/edward | */EDwaRd.app/Contents/MacOS/edward\ *) return 0 ;;
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

# fetch_release puts EDwaRd.app in $work, from EDWARD_TARBALL or a download.
fetch_release() {
	tarball=$work/edward-macos.tar.gz
	sums=$work/SHA256SUMS
	if [ -n "${EDWARD_TARBALL:-}" ]; then
		[ -f "$EDWARD_TARBALL" ] || die "EDWARD_TARBALL is not a file: $EDWARD_TARBALL"
		cp "$EDWARD_TARBALL" "$tarball"
		src_sums=$(dirname -- "$EDWARD_TARBALL")/SHA256SUMS
		[ -f "$src_sums" ] && cp "$src_sums" "$sums"
	else
		if [ -n "${EDWARD_VERSION:-}" ]; then
			base=https://github.com/$REPO/releases/download/$EDWARD_VERSION
		else
			base=https://github.com/$REPO/releases/latest/download
		fi
		echo "Downloading $base/edward-macos.tar.gz" >&2
		curl -fsSL --proto '=https' --tlsv1.2 -o "$tarball" "$base/edward-macos.tar.gz" ||
			die "download failed: $base/edward-macos.tar.gz"
		curl -fsSL --proto '=https' --tlsv1.2 -o "$sums" "$base/SHA256SUMS" ||
			die "download failed: $base/SHA256SUMS"
	fi
	if [ -f "$sums" ]; then
		(cd "$work" && grep ' edward-macos.tar.gz$' SHA256SUMS | shasum -a 256 -c - >/dev/null) ||
			die "SHA-256 of edward-macos.tar.gz does not match SHA256SUMS. Nothing was installed."
		echo "SHA-256 verified" >&2
	elif [ -z "${EDWARD_TARBALL:-}" ]; then
		die "no SHA256SUMS for this release. Nothing was installed."
	fi
	tar -xzf "$tarball" -C "$work" || die "could not unpack edward-macos.tar.gz"
	[ -d "$work/EDwaRd.app" ] || die "the release has no EDwaRd.app"
	printf '%s\n' "$work/EDwaRd.app"
}

find_bundle() {
	if [ -n "${EDWARD_BUNDLE:-}" ]; then
		[ -d "$EDWARD_BUNDLE" ] || die "EDWARD_BUNDLE is not a directory: $EDWARD_BUNDLE"
		printf '%s\n' "$EDWARD_BUNDLE"
	elif [ -z "${EDWARD_TARBALL:-}" ] && [ "$(basename -- "$0")" = install.sh ] && [ -f ./install.sh ] && [ -d ./EDwaRd.app ]; then
		printf '%s\n' "$(pwd)/EDwaRd.app"
	else
		fetch_release
	fi
}

# write_plist is the LaunchAgent. install/com.edward.agent.plist is the same
# text with placeholders; a Go test keeps the two in step.
write_plist() {
	b=$(xml_escape "$BIN")
	d=$(xml_escape "$APPDIR")
	l=$(xml_escape "$LOGDIR")
	# plist:start
	cat >"$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.edward.agent</string>
	<key>ProgramArguments</key>
	<array>
		<string>$b</string>
		<string>serve</string>
	</array>
	<key>WorkingDirectory</key>
	<string>$d</string>
	<key>EnvironmentVariables</key>
	<dict>
		<key>EDWARD_HOME</key>
		<string>$d</string>
	</dict>
	<key>StandardOutPath</key>
	<string>$l/edward.log</string>
	<key>StandardErrorPath</key>
	<string>$l/edward.log</string>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
</dict>
</plist>
EOF
	# plist:end
}

main() {
	check_platform

	APPDIR=$(trim_slash "${EDWARD_HOME:-$HOME/Library/Application Support/EDwaRd}")
	APP=$APPDIR/EDwaRd.app
	BIN=$APP/Contents/MacOS/edward
	LAUNCH_AGENTS=$(trim_slash "${EDWARD_LAUNCH_AGENTS:-$HOME/Library/LaunchAgents}")
	PLIST=$LAUNCH_AGENTS/com.edward.agent.plist
	LOGDIR=$(trim_slash "${EDWARD_LOGS:-$HOME/Library/Logs/EDwaRd}")
	LINK=$HOME/.local/bin/edward
	domain=gui/$(id -u)

	if port_in_use && ! listener_is_edward; then
		echo "127.0.0.1:$PORT is already in use by another program. Stop it, then run this again." >&2
		echo "Listener: $(listener_cmd)" >&2
		exit 1
	fi

	work=$(mktemp -d "${TMPDIR:-/tmp}/edward-install.XXXXXX")
	trap 'rm -rf "$work"' EXIT INT TERM

	bundle=$(find_bundle)
	[ -x "$bundle/Contents/MacOS/edward" ] || die "the bundle has no Contents/MacOS/edward"
	# A build from before `edward version` existed would start a server here
	# instead of printing; refuse it rather than install a stale app.
	new_version=$("$bundle/Contents/MacOS/edward" version 2>/dev/null) || new_version=
	case $new_version in
	[0-9]*) ;;
	*) die "$bundle is an old or broken build (edward version did not answer). Run make app again." ;;
	esac

	bootout_agent
	mkdir -p "$APPDIR" "$LAUNCH_AGENTS" "$LOGDIR" "$HOME/.local/bin"
	chmod 700 "$APPDIR"
	rm -rf "$APP"
	ditto "$bundle" "$APP"
	ln -sfn "$BIN" "$LINK"
	write_plist

	# An EDwaRd from an older build can outlive bootout. Stop it by binary path.
	i=0
	while port_in_use && [ "$i" -lt 5 ]; do
		if ! listener_is_edward; then
			die "127.0.0.1:$PORT was taken by another program during install."
		fi
		pkill -f "/EDwaRd.app/Contents/MacOS/edward" 2>/dev/null || true
		sleep 1
		i=$((i + 1))
	done

	launchctl bootstrap "$domain" "$PLIST" || die "launchctl bootstrap failed for $PLIST"

	i=0
	while ! curl -sf --max-time 1 "$PAGE/" >/dev/null 2>&1; do
		i=$((i + 1))
		if [ "$i" -ge 20 ]; then
			die "installed, but $PAGE did not answer within 20 seconds. Log: $LOGDIR/edward.log"
		fi
		sleep 1
	done

	echo
	echo "EDwaRd $new_version is running: $PAGE"
	echo "Log:       $LOGDIR/edward.log"
	echo "Uninstall: ~/.local/bin/edward uninstall"
	case ":$PATH:" in
	*":$HOME/.local/bin:"*) ;;
	*) echo "Add ~/.local/bin to PATH to run edward by name." ;;
	esac
	open "$PAGE" >/dev/null 2>&1 || true
}

main "$@"

#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"
chaos_guard

[ "$#" -eq 1 ] || ops_die "usage: disk.sh fill|clear"
: "${JUDGIFY_CHAOS_VOLUME:?set the isolated bounded test volume}"
ops_require_absolute_path "$JUDGIFY_CHAOS_VOLUME" "chaos volume"
[ "$JUDGIFY_CHAOS_VOLUME" != "/" ] || ops_die "host root is never a chaos volume"
ops_require_secure_directory "$JUDGIFY_CHAOS_VOLUME" "chaos volume"
ops_require_trusted_file \
	"$JUDGIFY_CHAOS_VOLUME/.judgify-chaos-volume" \
	"chaos volume marker"

pressure_file="$JUDGIFY_CHAOS_VOLUME/judgify-pressure.bin"
case "$1" in
	fill)
		bytes=${JUDGIFY_CHAOS_DISK_BYTES:-536870912}
		case "$bytes" in
			''|*[!0-9]*) ops_die "chaos disk bytes must be an integer" ;;
		esac
		[ "$bytes" -ge 1048576 ] && [ "$bytes" -le 1073741824 ] ||
			ops_die "chaos disk bytes must be between 1 MiB and 1 GiB"
		[ ! -e "$pressure_file" ] || ops_die "chaos pressure file already exists"
		blocks=$((bytes / 1048576))
		printf '%s\n' "planned chaos action: allocate ${blocks} MiB in isolated volume"
		dd if=/dev/zero of="$pressure_file" bs=1048576 count="$blocks" conv=fsync status=none
		;;
	clear)
		printf '%s\n' "planned chaos action: clear isolated pressure file"
		rm -f -- "$pressure_file"
		;;
	*)
		ops_die "unsupported disk chaos action"
		;;
esac

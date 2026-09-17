#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"
chaos_guard

[ "$#" -eq 1 ] || ops_die "usage: postgres.sh restart|pause|resume"
case "$1" in
	restart|pause|resume) ;;
	*) ops_die "unsupported PostgreSQL chaos action" ;;
esac
chaos_driver postgres "$1"


#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"
chaos_guard

[ "$#" -eq 2 ] || ops_die "usage: process.sh kill|restart api|judge-worker"
action=$1
target=$2
case "$action:$target" in
	kill:api|kill:judge-worker|restart:api|restart:judge-worker) ;;
	*) ops_die "unsupported process chaos action" ;;
esac
chaos_driver process "$action" "$target"


#!/bin/sh
# Convenience runner for the destructive backup/restore proof. It only supplies
# the acknowledgement and isolated fixture names the harness already demands;
# every check still lives in backup_restore_test.sh.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
# The fixture boots postgres:16, and the backup scripts refuse a client whose
# major version differs from the server. Homebrew's plain libpq keg tracks the
# newest release, so it must not be what this harness puts on PATH.
PATH="/opt/homebrew/opt/postgresql@16/bin:$PATH"
export PATH

JUDGIFY_BACKUP_RESTORE_TEST_ACK=DESTROY_ONLY_JUDGIFY_RESTORE_TEST_DATABASES
JUDGIFY_BACKUP_RESTORE_DRIVER="$ROOT/test/operations/backup_restore_driver.sh"
JUDGIFY_TEST_SOURCE_PGSERVICE=judgify_src
JUDGIFY_TEST_TARGET_PGSERVICE=judgify_tgt
JUDGIFY_TEST_SOURCE_DATABASE=source_judgify_restore_test
JUDGIFY_TEST_TARGET_DATABASE=target_judgify_restore_test
export JUDGIFY_BACKUP_RESTORE_TEST_ACK JUDGIFY_BACKUP_RESTORE_DRIVER
export JUDGIFY_TEST_SOURCE_PGSERVICE JUDGIFY_TEST_TARGET_PGSERVICE
export JUDGIFY_TEST_SOURCE_DATABASE JUDGIFY_TEST_TARGET_DATABASE

exec sh "$ROOT/test/operations/backup_restore_test.sh"

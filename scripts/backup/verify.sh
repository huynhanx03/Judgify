#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/../lib/operations.sh"

umask 077

[ "$#" -eq 1 ] || ops_die "usage: verify.sh /absolute/path/to/backup.tar"
bundle=$1
ops_require_trusted_file "$bundle" "backup bundle"

: "${JUDGIFY_BACKUP_SIGNING_PUBLIC_KEY_FILE:?set the manifest verification public key}"
: "${JUDGIFY_BACKUP_IDENTITY_FILE:?set the external age identity file}"
ops_require_trusted_file "$JUDGIFY_BACKUP_SIGNING_PUBLIC_KEY_FILE" "backup signing public key"
ops_require_private_file "$JUDGIFY_BACKUP_IDENTITY_FILE" "age identity file"
ops_require_command openssl
ops_require_command age
ops_require_command pg_restore
ops_require_command tar

checksum="${bundle}.sha256"
ops_require_trusted_file "$checksum" "backup bundle checksum"
expected_bundle_sha=$(awk 'NR == 1 && NF == 2 {print $1}' "$checksum")
case "$expected_bundle_sha" in
	''|*[!0-9a-f]*) ops_die "backup bundle checksum is malformed" ;;
esac
[ "${#expected_bundle_sha}" -eq 64 ] ||
	ops_die "backup bundle checksum is malformed"
[ "$(awk 'END {print NR}' "$checksum")" = "1" ] ||
	ops_die "backup bundle checksum file is malformed"
[ "$(awk 'NR == 1 && NF == 2 {print $2}' "$checksum")" = "$(basename "$bundle")" ] ||
	ops_die "backup bundle checksum names a different artifact"
[ "$expected_bundle_sha" = "$(ops_sha256 "$bundle")" ] ||
	ops_die "backup bundle checksum mismatch"

work=$(mktemp -d "$(ops_temp_root)/judgify-backup-verify.XXXXXX")
output_partial=
manifest_partial=
cleanup() {
	cleanup_status=$?
	trap - EXIT HUP INT TERM
	[ -z "$output_partial" ] || rm -f -- "$output_partial"
	[ -z "$manifest_partial" ] || rm -f -- "$manifest_partial"
	rm -rf -- "$work" || true
	exit "$cleanup_status"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

entries=$(tar -tf "$bundle")
[ "$entries" = "database.dump.age
manifest.env
manifest.sig" ] || ops_die "backup bundle contains unexpected entries"
entry_types=$(tar -tvf "$bundle" | cut -c1)
[ "$entry_types" = "-
-
-" ] || ops_die "backup bundle entries must all be regular files"
tar -xf "$bundle" -C "$work"
ops_require_regular_file "$work/database.dump.age" "encrypted database archive"
ops_require_regular_file "$work/manifest.env" "backup manifest"
ops_require_regular_file "$work/manifest.sig" "backup manifest signature"

openssl dgst -sha256 \
	-verify "$JUDGIFY_BACKUP_SIGNING_PUBLIC_KEY_FILE" \
	-signature "$work/manifest.sig" \
	"$work/manifest.env" >/dev/null ||
	ops_die "backup manifest signature is invalid"

manifest_schema=$(ops_manifest_value "$work/manifest.env" schema_version)
case "$manifest_schema" in
	1|2) ;;
	*) ops_die "unsupported backup manifest schema" ;;
esac
[ "$(ops_manifest_value "$work/manifest.env" format)" = "postgres-custom" ] ||
	ops_die "unsupported backup format"
[ "$(ops_manifest_value "$work/manifest.env" encryption)" = "age-v1" ] ||
	ops_die "unsupported backup encryption"
backup_id=$(ops_manifest_value "$work/manifest.env" backup_id)
database=$(ops_manifest_value "$work/manifest.env" database)
ops_safe_path_segment "$backup_id" 128 || ops_die "backup identity is malformed"
ops_safe_token "$database" 63 || ops_die "backup database identity is malformed"
created_epoch=$(ops_manifest_value "$work/manifest.env" created_epoch)
case "$created_epoch" in
	''|*[!0-9]*) ops_die "backup recovery-point timestamp is malformed" ;;
esac
created_at=$(ops_manifest_value "$work/manifest.env" created_at)
ops_safe_token "$created_at" 32 || ops_die "backup creation timestamp is malformed"

if [ "$manifest_schema" = "2" ]; then
	release_id=$(ops_manifest_value "$work/manifest.env" release_id)
	ops_safe_path_segment "$release_id" 120 || ops_die "backup release identity is malformed"
	migration_head=$(ops_manifest_value "$work/manifest.env" migration_head)
	case "$migration_head" in
		none) ;;
		??????????????)
			case "$migration_head" in *[!0-9]*) ops_die "backup migration head is malformed" ;; esac
			;;
		*) ops_die "backup migration head is malformed" ;;
	esac
	postgres_server_version=$(ops_manifest_value "$work/manifest.env" postgres_server_version)
	case "$postgres_server_version" in
		''|*[!0-9]*) ops_die "backup PostgreSQL version is malformed" ;;
	esac
fi

cipher_sha=$(ops_manifest_value "$work/manifest.env" ciphertext_sha256)
[ "${#cipher_sha}" -eq 64 ] || ops_die "encrypted database checksum is malformed"
[ "$cipher_sha" = "$(ops_sha256 "$work/database.dump.age")" ] ||
	ops_die "encrypted database checksum mismatch"
cipher_bytes=$(ops_manifest_value "$work/manifest.env" ciphertext_bytes)
case "$cipher_bytes" in
	''|*[!0-9]*) ops_die "encrypted database size is malformed" ;;
esac
[ "$cipher_bytes" = "$(ops_file_bytes "$work/database.dump.age")" ] ||
	ops_die "encrypted database size mismatch"

output=${JUDGIFY_VERIFIED_DUMP_OUTPUT:-"$work/database.dump"}
ops_require_absolute_path "$output" "verified dump output"
[ ! -e "$output" ] && [ ! -L "$output" ] ||
	ops_die "verified dump output already exists"
output_directory=$(dirname "$output")
ops_require_secure_directory "$output_directory" "verified dump output directory"
output_partial=$(mktemp "$output_directory/.judgify-verified-dump.XXXXXX")
age --decrypt \
	--identity "$JUDGIFY_BACKUP_IDENTITY_FILE" \
	"$work/database.dump.age" >"$output_partial"

plain_sha=$(ops_manifest_value "$work/manifest.env" plaintext_sha256)
[ "${#plain_sha}" -eq 64 ] || ops_die "decrypted database checksum is malformed"
[ "$plain_sha" = "$(ops_sha256 "$output_partial")" ] ||
	ops_die "decrypted database checksum mismatch"
plain_bytes=$(ops_manifest_value "$work/manifest.env" plaintext_bytes)
case "$plain_bytes" in
	''|*[!0-9]*) ops_die "decrypted database size is malformed" ;;
esac
[ "$plain_bytes" = "$(ops_file_bytes "$output_partial")" ] ||
	ops_die "decrypted database size mismatch"
pg_restore --list "$output_partial" >/dev/null ||
	ops_die "PostgreSQL custom archive is unreadable"
chmod 600 "$output_partial"
mv "$output_partial" "$output"
output_partial=

if [ -n "${JUDGIFY_VERIFIED_MANIFEST_OUTPUT:-}" ]; then
	manifest_output=$JUDGIFY_VERIFIED_MANIFEST_OUTPUT
	ops_require_absolute_path "$manifest_output" "verified manifest output"
	[ ! -e "$manifest_output" ] && [ ! -L "$manifest_output" ] ||
		ops_die "verified manifest output already exists"
	manifest_directory=$(dirname "$manifest_output")
	ops_require_secure_directory "$manifest_directory" "verified manifest output directory"
	manifest_partial=$(mktemp "$manifest_directory/.judgify-verified-manifest.XXXXXX")
	cp "$work/manifest.env" "$manifest_partial"
	chmod 600 "$manifest_partial"
	mv "$manifest_partial" "$manifest_output"
	manifest_partial=
fi

printf '%s\n' "backup verified: $backup_id"

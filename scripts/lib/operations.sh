#!/bin/sh

# Shared fail-closed helpers for Judgify operator scripts. This file performs
# no action when sourced and never prints secret-bearing environment values.

ops_die() {
	printf '%s\n' "judgify-ops: $*" >&2
	exit 1
}

ops_require_command() {
	command -v "$1" >/dev/null 2>&1 || ops_die "required command is unavailable: $1"
}

# Some platforms export TMPDIR with a trailing slash, which would make every
# derived path non-canonical and trip the path guards below.
ops_temp_root() {
	ops_temp_root_value="${TMPDIR:-/tmp}"
	while :; do
		case "$ops_temp_root_value" in
			/) break ;;
			*/) ops_temp_root_value="${ops_temp_root_value%/}" ;;
			*) break ;;
		esac
	done
	printf '%s' "$ops_temp_root_value"
}

ops_require_absolute_path() {
	case "${1-}" in
		/*) ;;
		*) ops_die "$2 must be an absolute path" ;;
	esac
	case "$1" in
		*'
'*|*''*|*'	'*) ops_die "$2 contains unsafe characters" ;;
	esac
	case "$1" in
		/) ;;
		*//*|*/./*|*/../*|*/.|*/..) ops_die "$2 must be lexically canonical" ;;
	esac
}

ops_require_regular_file() {
	ops_require_absolute_path "$1" "$2"
	[ -f "$1" ] || ops_die "$2 is not a regular file"
	[ ! -L "$1" ] || ops_die "$2 must not be a symbolic link"
}

ops_require_private_file() {
	ops_require_regular_file "$1" "$2"
	ops_require_trusted_owner_and_mode "$1" "$2"
	ops_private_mode=$(ops_file_mode "$1")
	case "$ops_private_mode" in
		600|400|440|640) ;;
		*) ops_die "$2 permissions must not grant access to other users (mode $ops_private_mode)" ;;
	esac
}

ops_file_device() {
	if stat -c '%d' "$1" >/dev/null 2>&1; then
		stat -c '%d' "$1"
	else
		stat -f '%d' "$1"
	fi
}

ops_file_uid() {
	if stat -c '%u' "$1" >/dev/null 2>&1; then
		stat -c '%u' "$1"
	else
		stat -f '%u' "$1"
	fi
}

ops_require_trusted_owner_and_mode() {
	ops_trusted_path=$1
	ops_trusted_label=$2
	ops_trusted_uid=$(ops_file_uid "$ops_trusted_path")
	ops_current_uid=$(id -u)
	case "$ops_trusted_uid" in
		0|"$ops_current_uid") ;;
		*) ops_die "$ops_trusted_label must be owned by root or the current operator" ;;
	esac
	ops_trusted_mode=$(ops_file_mode "$ops_trusted_path")
	case "$ops_trusted_mode" in
		''|*[!0-7]*) ops_die "$ops_trusted_label has an unreadable permission mode" ;;
	esac
	[ $((0$ops_trusted_mode & 0022)) -eq 0 ] ||
		ops_die "$ops_trusted_label must not be writable by group or other users (mode $ops_trusted_mode)"
}

ops_require_trusted_file() {
	ops_require_regular_file "$1" "$2"
	ops_require_trusted_owner_and_mode "$1" "$2"
}

ops_require_trusted_executable() {
	ops_require_trusted_file "$1" "$2"
	[ -x "$1" ] || ops_die "$2 is not executable"
}

ops_require_secure_directory() {
	ops_require_absolute_path "$1" "$2"
	[ -d "$1" ] || ops_die "$2 is not a directory"
	[ ! -L "$1" ] || ops_die "$2 must not be a symbolic link"
	ops_require_trusted_owner_and_mode "$1" "$2"
}

ops_ensure_secure_subdirectory() {
	ops_parent=$1
	ops_name=$2
	ops_label=$3
	ops_safe_path_segment "$ops_name" 255 || ops_die "$ops_label name is invalid"
	ops_require_secure_directory "$ops_parent" "parent of $ops_label"
	ops_child="$ops_parent/$ops_name"
	if [ ! -e "$ops_child" ] && [ ! -L "$ops_child" ]; then
		mkdir "$ops_child" || ops_die "cannot create $ops_label"
		chmod 700 "$ops_child" || ops_die "cannot secure $ops_label"
	fi
	ops_require_secure_directory "$ops_child" "$ops_label"
	printf '%s\n' "$ops_child"
}

ops_file_mode() {
	if stat -c '%a' "$1" >/dev/null 2>&1; then
		stat -c '%a' "$1"
	else
		stat -f '%Lp' "$1"
	fi
}

ops_sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	else
		ops_require_command shasum
		shasum -a 256 "$1" | awk '{print $1}'
	fi
}

ops_file_bytes() {
	if stat -c '%s' "$1" >/dev/null 2>&1; then
		stat -c '%s' "$1"
	else
		stat -f '%z' "$1"
	fi
}

ops_manifest_value() {
	file=$1
	key=$2
	awk -F= -v key="$key" '
		$1 == key {
			count++
			value = substr($0, length(key) + 2)
		}
		END {
			if (count != 1 || value == "") {
				exit 1
			}
			print value
		}
	' "$file"
}

ops_verify_transaction_activation() {
	ops_activation_file=$1
	ops_activation_manifest_sha=$2
	ops_activation_driver_sha=$3
	ops_activation_release_id=$4
	ops_require_trusted_file "$ops_activation_file" "transaction activation receipt"
	[ "$(ops_manifest_value "$ops_activation_file" schema_version)" = "1" ] ||
		ops_die "unsupported transaction activation receipt schema"
	[ "$(ops_manifest_value "$ops_activation_file" release_id)" = "$ops_activation_release_id" ] ||
		ops_die "transaction activation receipt names a different release"
	[ "$(ops_manifest_value "$ops_activation_file" release_manifest_sha256)" = "$ops_activation_manifest_sha" ] ||
		ops_die "transaction activation receipt belongs to a different manifest"
	[ "$(ops_manifest_value "$ops_activation_file" deployment_driver_sha256)" = "$ops_activation_driver_sha" ] ||
		ops_die "transaction activation receipt belongs to a different driver"
}

ops_record_transaction_activation() {
	ops_activation_transaction=$1
	ops_activation_manifest_sha=$2
	ops_activation_driver_sha=$3
	ops_activation_release_id=$4
	ops_require_secure_directory "$ops_activation_transaction" "deployment transaction"
	ops_activation_file="$ops_activation_transaction/activation.env"
	if [ -e "$ops_activation_file" ] || [ -L "$ops_activation_file" ]; then
		ops_verify_transaction_activation \
			"$ops_activation_file" \
			"$ops_activation_manifest_sha" \
			"$ops_activation_driver_sha" \
			"$ops_activation_release_id"
		return 0
	fi
	ops_activation_tmp=$(mktemp "$ops_activation_transaction/.activation.env.XXXXXX")
	if ! cat >"$ops_activation_tmp" <<EOF
schema_version=1
release_id=$ops_activation_release_id
release_manifest_sha256=$ops_activation_manifest_sha
deployment_driver_sha256=$ops_activation_driver_sha
activated_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
EOF
	then
		rm -f -- "$ops_activation_tmp"
		ops_die "cannot write transaction activation receipt"
	fi
	chmod 600 "$ops_activation_tmp" || {
		rm -f -- "$ops_activation_tmp"
		ops_die "cannot secure transaction activation receipt"
	}
	mv -f "$ops_activation_tmp" "$ops_activation_file" || {
		rm -f -- "$ops_activation_tmp"
		ops_die "cannot publish transaction activation receipt"
	}
}

ops_safe_token() {
	value=${1-}
	maximum=${2-128}
	[ -n "$value" ] || return 1
	[ "${#value}" -le "$maximum" ] || return 1
	case "$value" in
		*[!a-zA-Z0-9._:+-]*) return 1 ;;
	esac
	return 0
}

# Path segments are deliberately stricter than general manifest tokens. In
# particular, dot segments must never reach a path concatenation.
ops_safe_path_segment() {
	ops_segment=${1-}
	ops_segment_maximum=${2-128}
	ops_safe_token "$ops_segment" "$ops_segment_maximum" || return 1
	case "$ops_segment" in
		.|..|[!a-zA-Z0-9]*|*[!a-zA-Z0-9._+-]*) return 1 ;;
	esac
	return 0
}

# pg_dump emits SET commands for parameters that exist only from its own major
# version onward, so a dump taken with a client newer than the server is not
# restorable into that server. PostgreSQL 17 added transaction_timeout, which is
# how this was found: an 18 client produced a bundle that reported success and
# then failed at restore against 16. Refusing the mismatch turns an unrestorable
# backup into a backup that never claims to exist.
ops_postgres_client_major() {
	ops_client_report=$("$1" --version 2>/dev/null) ||
		ops_die "$1 did not report a version"
	ops_client_major=$(printf '%s\n' "$ops_client_report" |
		awk '{ for (i = 1; i <= NF; i++) if ($i ~ /^[0-9]/) { print $i; exit } }')
	ops_client_major=${ops_client_major%%[!0-9]*}
	case "$ops_client_major" in
		''|*[!0-9]*) ops_die "$1 reported an unparsable version" ;;
	esac
	printf '%s\n' "$ops_client_major"
}

ops_require_matching_postgres_client() {
	ops_matching_tool=${1-}
	ops_matching_server_num=${2-}
	case "$ops_matching_server_num" in
		''|*[!0-9]*) ops_die "PostgreSQL server version is not a version number" ;;
	esac
	ops_matching_server_major=$((ops_matching_server_num / 10000))
	ops_matching_client_major=$(ops_postgres_client_major "$ops_matching_tool")
	[ "$ops_matching_client_major" = "$ops_matching_server_major" ] ||
		ops_die "$ops_matching_tool is major version $ops_matching_client_major but the PostgreSQL server is major version $ops_matching_server_major; use the client that matches the server"
}

ops_write_metric_file() {
	target=${1-}
	body=${2-}
	[ -n "$target" ] || return 0
	ops_require_absolute_path "$target" "metrics file"
	directory=$(dirname "$target")
	ops_require_secure_directory "$directory" "metrics directory"
	[ ! -L "$target" ] || ops_die "metrics file must not be a symbolic link"
	if [ -e "$target" ]; then
		ops_require_trusted_file "$target" "metrics file"
	fi
	temporary=$(mktemp "$directory/.judgify-metrics.XXXXXX")
	umask 077
	if ! printf '%s\n' "$body" >"$temporary"; then
		rm -f -- "$temporary"
		ops_die "cannot write metrics file"
	fi
	mv -f "$temporary" "$target"
}

ops_metric_counter_value() {
	target=${1-}
	metric=${2-}
	ops_safe_token "$metric" 128 || ops_die "metric name is invalid"
	if [ -z "$target" ] || [ ! -e "$target" ]; then
		printf '%s\n' "0"
		return 0
	fi
	ops_require_trusted_file "$target" "metrics file"
	value=$(awk -v metric="$metric" '
		$1 == metric {
			if (NF != 2 || $2 !~ /^[0-9]+$/) {
				exit 2
			}
			count++
			value = $2
		}
		END {
			if (count > 1) {
				exit 2
			}
			if (count == 0) {
				print 0
			} else {
				print value
			}
		}
	' "$target") || ops_die "metric counter state is malformed"
	printf '%s\n' "$value"
}

ops_increment_metric_counter() {
	target=${1-}
	metric=${2-}
	[ -n "$target" ] || return 0
	current=$(ops_metric_counter_value "$target" "$metric")
	next=$((current + 1))
	[ "$next" -gt "$current" ] || ops_die "metric counter overflow"
	if [ -e "$target" ]; then
		body=$(awk -v metric="$metric" '$1 != metric { print }' "$target")
	else
		body=
	fi
	if [ -n "$body" ]; then
		body="$body
$metric $next"
	else
		body="$metric $next"
	fi
	ops_write_metric_file "$target" "$body"
}

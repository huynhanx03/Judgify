#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/../lib/operations.sh"

: "${JUDGIFY_SMOKE_BASE_URL:?set the candidate HTTPS base URL}"
case "$JUDGIFY_SMOKE_BASE_URL" in
	*'@'*|*'\'*|*' '*|*'\t'*|*'\r'*|*'\n'*)
		ops_die "smoke base URL must not contain credentials or control characters"
		;;
	*\?*|*\#*) ops_die "smoke base URL must not contain a query or fragment" ;;
	https://*/*|http://127.0.0.1:*/*|http://localhost:*/*)
		ops_die "smoke base URL must not contain a path or trailing slash"
		;;
	https://*) ;;
	http://127.0.0.1:*|http://localhost:*) ;;
	*) ops_die "smoke target must be HTTPS or an explicit loopback URL" ;;
esac

ops_require_command curl
: "${JUDGIFY_DEPLOY_DRIVER:?set the deployment driver for authenticated product smoke}"
ops_require_trusted_executable "$JUDGIFY_DEPLOY_DRIVER" "deployment driver"

request() {
	path=$1
	expected=$2
	output=$(mktemp "$(ops_temp_root)/judgify-smoke.XXXXXX")
	status=$(curl \
		--silent \
		--show-error \
		--noproxy '*' \
		--proto '=http,https' \
		--max-time 5 \
		--output "$output" \
		--write-out '%{http_code}' \
		"$JUDGIFY_SMOKE_BASE_URL$path") || {
		rm -f "$output"
		ops_die "smoke request failed: $path"
	}
	rm -f "$output"
	[ "$status" = "$expected" ] || ops_die "$path returned HTTP $status, expected $expected"
}

# Static ping proves only candidate product-listener connectivity. The rollout
# driver separately gates the role-local private /live and /ready endpoints.
request /ping 200

"$JUDGIFY_DEPLOY_DRIVER" smoke-authenticated "$JUDGIFY_SMOKE_BASE_URL"
"$JUDGIFY_DEPLOY_DRIVER" smoke-websocket "$JUDGIFY_SMOKE_BASE_URL"
"$JUDGIFY_DEPLOY_DRIVER" smoke-submission "$JUDGIFY_SMOKE_BASE_URL"

printf '%s\n' "candidate product smoke checks passed"

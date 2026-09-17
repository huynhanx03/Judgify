#!/usr/bin/env bash

set -euo pipefail

api_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
module_path="github.com/huynhanx03/judgify/internal/ports"
output_dir="$api_dir/internal/testutil/mocks"

generate_domain() {
	local domain="$1"
	local interfaces

	interfaces=$(/usr/bin/awk '/^type [A-Za-z0-9_]+ interface \{/{print $2}' "$api_dir/internal/ports/$domain.go" | /usr/bin/paste -sd, -)
	if [[ -z "$interfaces" ]]; then
		echo "no interfaces found in ports/$domain.go" >&2
		exit 1
	fi

	(
		cd "$api_dir"
		go run go.uber.org/mock/mockgen -package=mocks "$module_path" "$interfaces" >"$output_dir/$domain.go"
	)
}

mkdir -p "$output_dir"
for domain in identity problem submission contest cultivation material; do
	generate_domain "$domain"
done

#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
. "$root/scripts/lib/operations.sh"
api="$root/api"
common="${GO_COMMON_ROOT:-$root/../go-common}"
mode="${JUDGIFY_VERIFY_MODE:-development}"

case "$mode" in
  development|release) ;;
  *) echo "go verification: JUDGIFY_VERIFY_MODE must be development or release" >&2; exit 2 ;;
esac

for command in go gofmt git; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "go verification: required command is unavailable: $command" >&2
    exit 2
  }
done
for module in "$common" "$api"; do
  [[ -f "$module/go.mod" && -f "$module/go.sum" ]] || {
    echo "go verification: incomplete Go module: $module" >&2
    exit 2
  }
done

unformatted="$({ find "$common" "$api" -type f -name '*.go' -not -path '*/vendor/*' -print0 | xargs -0 gofmt -l; } | LC_ALL=C sort)"
if [[ -n "$unformatted" ]]; then
  echo "go verification: unformatted files:" >&2
  printf '%s\n' "$unformatted" >&2
  exit 1
fi

for module in "$common" "$api"; do
  (
    cd "$module"
    go mod verify
    go mod tidy -diff
    go vet ./...
  )
done

"$root/scripts/verify/generated.sh"
(
  cd "$common"
  go test -race -count=1 ./...
)
coverage="$(mktemp "$(ops_temp_root)/judgify-go-coverage.XXXXXX")"
authored_coverage="$(mktemp "$(ops_temp_root)/judgify-go-authored.XXXXXX")"
cleanup() {
  rm -f -- "$coverage" "$authored_coverage"
}
trap cleanup EXIT
(
  cd "$api"
  go test -race -count=1 -covermode=atomic -coverprofile="$coverage" ./...
  JUDGIFY_ARCH_STRICT=1 go test -count=1 ./internal/architecture
)

# Generated Ent code and generated mocks are 84% of the raw profile and carry no
# tests of their own, so a total taken over them measures code generation rather
# than authored code, and a floor set against it could not detect a real
# regression. Report the authored total instead.
grep -v -E '/internal/ent/generate/|/mocks/' "$coverage" >"$authored_coverage"
[[ -s "$authored_coverage" ]] || {
  echo "go verification: authored coverage profile is empty" >&2
  exit 1
}

# go tool cover resolves packages from the profile, so it must run inside the
# API module; the repository root is not a Go module.
total_coverage="$(cd "$api" && go tool cover -func="$authored_coverage" | awk '/^total:/ {gsub(/%/, "", $3); print $3}')"
[[ "$total_coverage" =~ ^[0-9]+([.][0-9]+)?$ ]] || {
  echo "go verification: could not read total coverage" >&2
  exit 1
}
if [[ "$mode" == "release" ]]; then
  floor="${JUDGIFY_GO_COVERAGE_FLOOR_PERCENT:?release verification requires the measured coverage floor}"
  awk -v actual="$total_coverage" -v floor="$floor" '
    BEGIN {
      if (floor !~ /^[0-9]+([.][0-9]+)?$/ || floor < 0 || floor > 100) exit 2
      if (actual + 0 < floor + 0) exit 1
    }
  ' || {
    status=$?
    if [[ "$status" -eq 2 ]]; then
      echo "go verification: coverage floor is malformed" >&2
    else
      echo "go verification: total coverage ${total_coverage}% is below ${floor}%" >&2
    fi
    exit 1
  }
fi

if command -v golangci-lint >/dev/null 2>&1; then
  (cd "$api" && golangci-lint run --timeout=5m)
elif [[ "$mode" == "release" ]]; then
  echo "go verification: release mode requires golangci-lint" >&2
  exit 2
fi
if command -v govulncheck >/dev/null 2>&1; then
  (cd "$common" && govulncheck ./...)
  (cd "$api" && govulncheck ./...)
elif [[ "$mode" == "release" ]]; then
  echo "go verification: release mode requires govulncheck" >&2
  exit 2
fi

printf 'Go verification passed; API authored coverage=%s%% mode=%s\n' "$total_coverage" "$mode"

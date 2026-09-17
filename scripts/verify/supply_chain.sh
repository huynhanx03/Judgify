#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
. "$root/scripts/lib/operations.sh"
mode="${1:---check}"
checksums_partial=""

cleanup() {
  [[ -z "$checksums_partial" ]] || rm -f -- "$checksums_partial"
}
trap cleanup EXIT

die() {
  echo "supply-chain verification: $*" >&2
  exit 1
}
case "$mode" in
  --check|--evidence) ;;
  *) echo "supply-chain verification: use --check or --evidence" >&2; exit 2 ;;
esac

[[ -f "$root/api/go.sum" && -f "$root/website/package-lock.json" ]] ||
  die "immutable dependency lockfiles are required"
if [[ "$mode" == "--evidence" ]] && grep -Eq '^[[:space:]]*replace[[:space:]].*=>[[:space:]]+[.][.]/' "$root/api/go.mod"; then
  die "release module graph contains an external filesystem replace"
fi
if [[ "$mode" == "--evidence" ]] && grep -Eq 'github[.]com/huynhanx03/go-common[[:space:]]+v0[.]0[.]0([[:space:]]|$)' "$root/api/go.mod"; then
  die "go-common must use an immutable released version"
fi
while IFS= read -r base_image; do
  case "$base_image" in
    dependencies|builder|docker_cli|inspector_builder|runtime) continue ;;
  esac
  [[ "$base_image" =~ @sha256:[0-9a-f]{64}$ ]] ||
    die "every Docker base image must be digest pinned: $base_image"
done < <(awk '/^[[:space:]]*FROM[[:space:]]+/ {print $2}' "$root"/docker/*/Dockerfile)
if grep -n -E '^[[:space:]]+image:[[:space:]]+[^$][^[:space:]@]+(:[^[:space:]@]+)?$' \
  "$root/deploy/compose.production.yml" >/dev/null; then
  die "literal production images must be digest pinned"
fi

[[ "$mode" == "--evidence" ]] || {
  printf '%s\n' "supply-chain source contracts verified"
  exit 0
}

[[ "${JUDGIFY_RELEASE_EVIDENCE_ACK:-}" == "GENERATE_UNSIGNED_RELEASE_EVIDENCE" ]] ||
  die "evidence mode requires explicit release-evidence acknowledgement"
evidence="${JUDGIFY_RELEASE_EVIDENCE_DIR:?set an empty absolute evidence directory}"
ops_require_secure_directory "$evidence" "release evidence directory"
case "$evidence" in
  "$root"|"$root"/*) die "release evidence directory must be outside the source tree" ;;
esac
[[ -z "$(find "$evidence" -mindepth 1 -maxdepth 1 -print -quit)" ]] ||
  die "release evidence directory must be empty"
for command in git go node npm syft grype gitleaks; do
  command -v "$command" >/dev/null 2>&1 || die "evidence mode requires $command"
done
[[ -z "$(git -C "$root" status --porcelain=v1)" ]] ||
  die "release evidence requires a clean source tree"
images_file="${JUDGIFY_RELEASE_IMAGES_FILE:?set the approved digest-pinned image inventory}"
ops_require_trusted_file "$images_file" "release image inventory"

umask 077
commit="$(git -C "$root" rev-parse --verify HEAD)"
[[ "$commit" =~ ^[0-9a-f]{40}$ ]] || die "source commit is not immutable"
{
  printf 'schema_version=1\n'
  printf 'source_commit=%s\n' "$commit"
  printf 'go_version=%s\n' "$(go version | tr ' ' '_')"
  printf 'node_version=%s\n' "$(node --version)"
  printf 'npm_version=%s\n' "$(npm --version)"
  printf 'syft_version=%s\n' "$(syft version -o json | tr -d '\n')"
  printf 'created_at=%s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
} >"$evidence/build.env"

syft "dir:$root/api" -o spdx-json="$evidence/api.spdx.json"
syft "dir:$root/website" -o spdx-json="$evidence/website.spdx.json"
grype "sbom:$evidence/api.spdx.json" --fail-on high -o json >"$evidence/api-vulnerabilities.json"
grype "sbom:$evidence/website.spdx.json" --fail-on high -o json >"$evidence/website-vulnerabilities.json"
gitleaks detect --source "$root" --no-git --redact --report-format json \
  --report-path "$evidence/secret-scan.json"

image_names="$evidence/.image-names"
: >"$image_names"
while IFS='=' read -r image_name image_reference; do
  [[ -n "$image_name$image_reference" ]] || continue
  [[ "$image_name" =~ ^[a-z][a-z0-9-]{0,31}$ ]] ||
    die "release image inventory contains an invalid logical name"
  [[ "$image_reference" =~ @sha256:[0-9a-f]{64}$ ]] ||
    die "release image $image_name is not digest pinned"
  [[ "$image_reference" != *@sha256:0000000000000000000000000000000000000000000000000000000000000000 ]] ||
    die "release image $image_name uses the documentation-only zero digest"
  ! grep -Fx "$image_name" "$image_names" >/dev/null ||
    die "release image inventory repeats $image_name"
  printf '%s\n' "$image_name" >>"$image_names"
  syft "$image_reference" -o spdx-json="$evidence/image-${image_name}.spdx.json"
  grype "sbom:$evidence/image-${image_name}.spdx.json" --fail-on high -o json \
    >"$evidence/image-${image_name}-vulnerabilities.json"
done <"$images_file"
for required_image in api worker migrate website sandbox envoy; do
  grep -Fx "$required_image" "$image_names" >/dev/null ||
    die "release image inventory is missing $required_image"
done
rm -f -- "$image_names"
(
  cd "$root/api"
  go list -m -json all >"$evidence/go-modules.json"
)
(
  cd "$root/website"
  npm audit --package-lock-only --omit=dev --audit-level=high --json >"$evidence/npm-audit.json"
)

checksums_partial="$(mktemp "$evidence/.SHA256SUMS.XXXXXX")"
(
  cd "$evidence"
  for artifact in *; do
    if command -v sha256sum >/dev/null 2>&1; then
      sha256sum "$artifact"
    else
      shasum -a 256 "$artifact"
    fi
  done
) | LC_ALL=C sort >"$checksums_partial"
chmod 600 "$checksums_partial"
mv "$checksums_partial" "$evidence/SHA256SUMS"
checksums_partial=""

printf '%s\n' "unsigned release evidence generated; signing/provenance remains an approved release-environment action"

#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
migrations="$root/api/internal/ent/migrate/migrations"
mode="${1:---check}"

case "$mode" in
  --check|--integration) ;;
  *) echo "migration verification: use --check or --integration" >&2; exit 2 ;;
esac
[[ -f "$migrations/atlas.sum" ]] || {
  echo "migration verification: atlas.sum is missing" >&2
  exit 1
}

sql_files=()
while IFS= read -r sql_file; do
  sql_files+=("$sql_file")
done < <(find "$migrations" -maxdepth 1 -type f -name '*.sql' -print | LC_ALL=C sort)
[[ "${#sql_files[@]}" -gt 0 ]] || {
  echo "migration verification: no checked-in SQL migrations" >&2
  exit 1
}
previous=""
for file in "${sql_files[@]}"; do
  name="$(basename "$file")"
  [[ "$name" =~ ^([0-9]{14})_[a-z0-9][a-z0-9_]*[.]sql$ ]] || {
    echo "migration verification: invalid migration filename: $name" >&2
    exit 1
  }
  version="${BASH_REMATCH[1]}"
  [[ -z "$previous" || "$version" > "$previous" ]] || {
    echo "migration verification: versions are duplicate or unordered" >&2
    exit 1
  }
  grep -Fq "$name " "$migrations/atlas.sum" || {
    echo "migration verification: atlas.sum does not bind $name" >&2
    exit 1
  }
  previous="$version"
done
sum_entries="$(awk 'NR > 1 && NF == 2 {count++} END {print count + 0}' "$migrations/atlas.sum")"
[[ "$sum_entries" -eq "${#sql_files[@]}" ]] || {
  echo "migration verification: atlas.sum inventory differs from SQL inventory" >&2
  exit 1
}

if command -v atlas >/dev/null 2>&1; then
  atlas migrate validate --dir "file://$migrations"
elif [[ "$mode" == "--integration" ]]; then
  echo "migration verification: integration mode requires Atlas" >&2
  exit 2
fi

if grep -REn 'Schema[.]Create|client[.]Schema[.]Create|schema[.]Create' \
  "$root/api/cmd" "$root/api/internal/infrastructure" >/dev/null; then
  echo "migration verification: runtime auto-migration is forbidden" >&2
  exit 1
fi

if [[ "$mode" == "--integration" ]]; then
  "$root/scripts/verify/database_baseline.sh"
fi
printf 'Migration inventory verified; files=%s mode=%s\n' "${#sql_files[@]}" "$mode"

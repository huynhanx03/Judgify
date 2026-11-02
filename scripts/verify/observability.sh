#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
. "$ROOT/scripts/lib/operations.sh"

metrics="$ROOT/ops/observability/metrics.txt"
slos="$ROOT/ops/observability/slos.yaml"
alerts="$ROOT/ops/observability/alerts.yaml"
code_catalog="$ROOT/api/internal/platform/observability/catalog.go"

for required in "$metrics" "$slos" "$alerts" "$code_catalog"; do
	[ -f "$required" ] || ops_die "missing observability contract: $required"
done

LC_ALL=C sort -c -u "$metrics" ||
	ops_die "metric catalog must be sorted and unique"
while IFS= read -r metric; do
	ops_safe_token "$metric" 128 || ops_die "invalid metric catalog entry: $metric"
done <"$metrics"

work=$(mktemp -d "$(ops_temp_root)/judgify-observability.XXXXXX")
cleanup() {
	rm -rf -- "$work"
}
trap cleanup EXIT
sed -n 's/^[[:space:]]*Metric[A-Za-z0-9_]*[[:space:]]*Metric[[:space:]]*=[[:space:]]*"\(judgify_[a-z0-9_]*\)".*/\1/p' \
	"$code_catalog" >"$work/go-metrics.txt"
{
	cat "$work/go-metrics.txt"
	grep -Eho 'judgify_(backup|restore)_[a-z0-9_]+' \
		"$ROOT/scripts/backup/postgres.sh" \
		"$ROOT/scripts/restore/postgres.sh"
} | LC_ALL=C sort -u >"$work/emitted-metrics.txt"
if ! cmp -s "$metrics" "$work/emitted-metrics.txt"; then
	printf '%s\n' "observability metric catalog differs from code and operator emitters" >&2
	diff -u "$metrics" "$work/emitted-metrics.txt" >&2 || true
	exit 1
fi

check_manifest_metrics() {
	manifest=$1
	awk '/^[[:space:]]+metric:/ {print $2}' "$manifest" |
	while IFS= read -r metric; do
		grep -Fx "$metric" "$metrics" >/dev/null ||
			ops_die "$(basename "$manifest") references unknown metric $metric"
	done
}
check_manifest_metrics "$slos"
check_manifest_metrics "$alerts"

check_runbooks() {
	manifest=$1
	awk '/^[[:space:]]+runbook:/ {print $2}' "$manifest" |
	while IFS= read -r runbook; do
		case "$runbook" in
			ops/runbooks/*.md) ;;
			*) ops_die "unsafe runbook reference in $(basename "$manifest")" ;;
		esac
		[ -f "$ROOT/$runbook" ] ||
			ops_die "missing runbook referenced by $(basename "$manifest"): $runbook"
	done
}
check_runbooks "$slos"
check_runbooks "$alerts"

awk '
	/^[[:space:]]+target:/ {
		if ($2 <= 0 || $2 > 1) exit 1
		targets++
	}
	/^[[:space:]]+budget:/ {
		if ($2 <= 0 || $2 >= 1) exit 1
		budgets++
	}
	/^[[:space:]]+window:/ { windows++ }
	END {
		if (targets == 0 || targets != budgets || targets != windows) exit 1
	}
' "$slos" || ops_die "SLO targets, budgets, or windows are invalid"

awk '
	/^[[:space:]]+threshold:/ {
		if ($2 <= 0) exit 1
		thresholds++
	}
	/^[[:space:]]+runbook:/ { runbooks++ }
	END {
		if (thresholds == 0 || thresholds != runbooks) exit 1
	}
' "$alerts" || ops_die "alert thresholds or runbook coverage are invalid"

if grep -E 'by[[:space:]]*\([^)]*(cid|correlation_id|user_id|session_id|submission_id|job_id|event_id|email|ip_address)' \
	"$slos" "$alerts" >/dev/null; then
	ops_die "observability query groups by a forbidden high-cardinality label"
fi

ops_require_command node
node - "$ROOT" <<'NODE'
const fs = require("fs");
const path = require("path");
const root = process.argv[2];
const catalog = new Set(fs.readFileSync(path.join(root, "ops", "observability", "metrics.txt"), "utf8").trim().split(/\n/));
const allowedLabels = new Set([
  "role", "environment", "version", "method", "route", "status_class", "state",
  "query_class", "outcome", "kind", "direction", "class", "projection", "probe",
  "check", "storage", "event", "le",
]);
function baseMetric(name) {
  if (catalog.has(name)) return name;
  for (const suffix of ["_bucket", "_sum", "_count"]) {
    if (name.endsWith(suffix) && catalog.has(name.slice(0, -suffix.length))) {
      return name.slice(0, -suffix.length);
    }
  }
  return null;
}
function verifyExpression(file, expression) {
  for (const metric of expression.match(/\bjudgify_[a-z0-9_]+\b/g) || []) {
    if (!baseMetric(metric)) throw new Error(`${file}: query references unknown metric ${metric}`);
  }
  for (const group of expression.matchAll(/\b(?:by|without)\s*\(([^)]*)\)/g)) {
    for (const label of group[1].split(",").map((value) => value.trim()).filter(Boolean)) {
      if (!allowedLabels.has(label)) throw new Error(`${file}: query groups by unbounded label ${label}`);
    }
  }
  for (const selector of expression.matchAll(/\{([^}]*)\}/g)) {
    for (const matcher of selector[1].matchAll(/(?:^|,)\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:!?=|!?~)/g)) {
      if (!allowedLabels.has(matcher[1])) throw new Error(`${file}: query filters unknown label ${matcher[1]}`);
    }
  }
}
const files = [
  "dashboard-api.json",
  "dashboard-judge.json",
  "dashboard-product.json",
];
for (const file of files) {
  const full = path.join(root, "ops", "observability", file);
  const dashboard = JSON.parse(fs.readFileSync(full, "utf8"));
  if (dashboard.schemaVersion !== 1 || !dashboard.title || !/^(?:1[05]|[2-9][0-9])s$/.test(dashboard.refresh) || !Array.isArray(dashboard.panels) || dashboard.panels.length === 0) {
    throw new Error(`${file}: invalid dashboard envelope`);
  }
  for (const panel of dashboard.panels) {
    if (!panel.title || !panel.unit || typeof panel.expr !== "string" || panel.expr.length > 1024) {
      throw new Error(`${file}: invalid panel`);
    }
    if (/\b(cid|correlation_id|user_id|session_id|submission_id|job_id|event_id|email|ip_address)\b/.test(panel.expr)) {
      throw new Error(`${file}: forbidden high-cardinality label`);
    }
    verifyExpression(file, panel.expr);
  }
}
for (const file of ["slos.yaml", "alerts.yaml"]) {
  const text = fs.readFileSync(path.join(root, "ops", "observability", file), "utf8");
  for (const match of text.matchAll(/^\s+(?:query|expression):\s+'([^']+)'\s*$/gm)) {
    verifyExpression(file, match[1]);
  }
}

const goRoot = path.join(root, "api");
const goFiles = [];
function walk(directory) {
  for (const entry of fs.readdirSync(directory, {withFileTypes: true})) {
    const full = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      if (!full.includes(`${path.sep}internal${path.sep}ent${path.sep}generate`)) walk(full);
    } else if (
      entry.isFile() &&
      entry.name.endsWith(".go") &&
      !entry.name.endsWith("_test.go")
    ) {
      goFiles.push(full);
    }
  }
}
walk(goRoot);
const goSource = goFiles.map((file) => fs.readFileSync(file, "utf8")).join("\n");
const catalogSource = fs.readFileSync(path.join(goRoot, "internal", "platform", "observability", "catalog.go"), "utf8");
for (const match of catalogSource.matchAll(/^\s*(Metric[A-Za-z0-9_]+)\s+Metric\s*=\s*"judgify_[a-z0-9_]+"/gm)) {
  const references = goSource.match(new RegExp(`\\b${match[1]}\\b`, "g")) || [];
  if (references.length < 3) {
    throw new Error(`catalog.go: ${match[1]} is declared but has no composition or lifecycle emitter`);
  }
}
NODE

printf '%s\n' "observability contracts verified"

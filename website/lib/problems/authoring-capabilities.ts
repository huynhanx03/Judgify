import type { ProblemAuthoringCatalog } from "@/types/problem";

const IDENTITY_SEPARATOR = "\u0000";
export const AUTHORING_CATALOG_CHANGED_REASON =
  "authoring_catalog_changed" as const;

export function checkerIdentity(key: string, version: string): string {
  return `${key}${IDENTITY_SEPARATOR}${version}`;
}

export function splitCheckerIdentity(value: string): [string, string] {
  const separator = value.indexOf(IDENTITY_SEPARATOR);
  return separator < 1
    ? ["", ""]
    : [value.slice(0, separator), value.slice(separator + 1)];
}

export function compatibleCheckerIdentities(
  catalog: ProblemAuthoringCatalog,
  runtimeKeys: readonly string[],
): ReadonlySet<string> {
  if (runtimeKeys.length === 0) {
    return new Set(
      catalog.checkers.map((checker) =>
        checkerIdentity(checker.key, checker.version)
      ),
    );
  }

  const requiredRuntimes = new Set(runtimeKeys);
  const supportedRuntimesByChecker = new Map<string, Set<string>>();
  for (const capability of catalog.capabilities) {
    const identity = checkerIdentity(
      capability.checker_key,
      capability.checker_version,
    );
    const supportedRuntimes = supportedRuntimesByChecker.get(identity) ??
      new Set<string>();
    supportedRuntimes.add(capability.runtime_key);
    supportedRuntimesByChecker.set(identity, supportedRuntimes);
  }

  return new Set(
    catalog.checkers
      .map((checker) => checkerIdentity(checker.key, checker.version))
      .filter((identity) => {
        const supportedRuntimes = supportedRuntimesByChecker.get(identity);
        return supportedRuntimes !== undefined &&
          [...requiredRuntimes].every((runtimeKey) =>
            supportedRuntimes.has(runtimeKey)
          );
      }),
  );
}

export function compatibleRuntimeKeys(
  catalog: ProblemAuthoringCatalog,
  identity: string,
): string[] {
  const [checkerKey, checkerVersion] = splitCheckerIdentity(identity);
  if (!checkerKey || !checkerVersion) return [];

  const supportedRuntimeKeys = new Set(
    catalog.capabilities
      .filter(
        (capability) =>
          capability.checker_key === checkerKey &&
          capability.checker_version === checkerVersion,
      )
      .map((capability) => capability.runtime_key),
  );
  return catalog.runtimes
    .map((runtime) => runtime.key)
    .filter((runtimeKey) => supportedRuntimeKeys.has(runtimeKey));
}

export function isCompatibleAuthoringSelection(
  catalog: ProblemAuthoringCatalog,
  runtimeKeys: readonly string[],
  identity: string,
): boolean {
  if (
    runtimeKeys.length === 0 ||
    new Set(runtimeKeys).size !== runtimeKeys.length
  ) {
    return false;
  }

  const [checkerKey, checkerVersion] = splitCheckerIdentity(identity);
  if (
    !catalog.checkers.some(
      (checker) =>
        checker.key === checkerKey && checker.version === checkerVersion,
    )
  ) {
    return false;
  }

  const activeRuntimeKeys = new Set(
    catalog.runtimes.map((runtime) => runtime.key),
  );
  return runtimeKeys.every((runtimeKey) => activeRuntimeKeys.has(runtimeKey)) &&
    compatibleCheckerIdentities(catalog, runtimeKeys).has(identity);
}

export function authoringCatalogsMatch(
  current: ProblemAuthoringCatalog,
  candidate: ProblemAuthoringCatalog,
): boolean {
  return JSON.stringify(current) === JSON.stringify(candidate);
}

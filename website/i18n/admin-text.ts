import { admin } from "@/i18n/catalog.admin.vi";
import type {
  AdminTextArguments,
  AdminTextKey,
  InterpolationValue,
} from "@/i18n/types";

export type { AdminParamsFor, AdminTextKey } from "@/i18n/types";

const UNSAFE_PATH_SEGMENTS = new Set(["__proto__", "constructor", "prototype"]);

function resolveLeaf(key: string): unknown {
  let current: unknown = admin;

  for (const segment of key.split(".")) {
    if (
      UNSAFE_PATH_SEGMENTS.has(segment) ||
      !current ||
      typeof current !== "object" ||
      !(segment in current)
    ) {
      throw new Error(`Unknown admin text key: ${key}`);
    }
    current = (current as Record<string, unknown>)[segment];
  }

  return current;
}

function interpolateNamed(
  message: string,
  params: Record<string, InterpolationValue> | undefined,
  key: string,
): string {
  return message.replace(/\{([A-Za-z][A-Za-z0-9_]*)\}/g, (_match, name: string) => {
    if (!params || !(name in params)) {
      throw new Error(`Missing interpolation parameter "${name}" for ${key}`);
    }
    return String(params[name]);
  });
}

/** Resolve static Vietnamese administrator copy without runtime locale or network loading. */
export function adminText<Key extends AdminTextKey>(
  key: Key,
  ...[params]: AdminTextArguments<Key>
): string {
  const leaf = resolveLeaf(key);

  if (typeof leaf === "string") {
    return interpolateNamed(
      leaf,
      params as Record<string, InterpolationValue> | undefined,
      key,
    );
  }

  if (typeof leaf === "function") {
    const formatterParams = params as { args?: readonly unknown[] } | undefined;
    const result = (leaf as (...args: readonly unknown[]) => unknown)(
      ...(formatterParams?.args ?? []),
    );
    return result == null ? "" : String(result);
  }

  throw new Error(`Admin text key does not resolve to a leaf: ${key}`);
}

import type {
  JudgeRuntimeCatalog,
  JudgeRuntime,
  RuntimeKey,
} from "@/types/submission";
import type { Schema } from "@/lib/api/schema";

const MAXIMUM_CATALOG_RUNTIMES = 32;
const MAXIMUM_BROWSER_SOURCE_BYTES = 16 * 1024 * 1024;
const MAXIMUM_LANGUAGE_ID_LENGTH = 64;
const MAXIMUM_RUNTIME_KEY_LENGTH = 64;
const MAXIMUM_DISPLAY_NAME_LENGTH = 128;
const MAXIMUM_SOURCE_FILENAME_LENGTH = 128;
const MAXIMUM_EXTENSIONS_PER_LANGUAGE = 16;
const MAXIMUM_EXTENSION_LENGTH = 24;
const LANGUAGE_ID_PATTERN = /^[a-z0-9][a-z0-9._+-]*$/;
const RUNTIME_KEY_PATTERN = /^[a-z0-9][a-z0-9._-]{0,63}$/;
const FILE_EXTENSION_PATTERN = /^\.[a-z0-9][a-z0-9_+-]*$/i;
const SOURCE_FILENAME_PATTERN = /^[a-z0-9][a-z0-9._+-]*$/i;
const CONTROL_CHARACTER_PATTERN = /[\u0000-\u001f\u007f]/;

export class JudgeRuntimeCatalogError extends Error {
  constructor() {
    super("judge runtime catalog is invalid");
    this.name = "JudgeRuntimeCatalogError";
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function invalidCatalog(): never {
  throw new JudgeRuntimeCatalogError();
}

export const runtimeKeySchema: Schema<RuntimeKey> = {
  parse(value: unknown): RuntimeKey {
    if (
      typeof value !== "string" ||
      value.length === 0 ||
      value.length > MAXIMUM_RUNTIME_KEY_LENGTH ||
      !RUNTIME_KEY_PATTERN.test(value)
    ) {
      throw new TypeError("runtime key is invalid");
    }
    return value as RuntimeKey;
  },
};

function hasExactKeys(
  value: Record<string, unknown>,
  keys: readonly string[],
): boolean {
  const actual = Object.keys(value);
  return (
    actual.length === keys.length &&
    actual.every((key) => keys.includes(key))
  );
}

function parseRuntime(value: unknown): JudgeRuntime {
  if (!isRecord(value)) return invalidCatalog();

  if (
    !hasExactKeys(value, [
      "runtime_key",
      "language",
      "display_name",
      "source_filename",
      "file_extensions",
      "compiled",
    ])
  ) {
    return invalidCatalog();
  }

  const {
    runtime_key: runtimeKey,
    language,
    display_name: displayName,
    source_filename: sourceFilename,
    file_extensions: extensions,
    compiled,
  } = value;
  if (
    typeof runtimeKey !== "string" ||
    runtimeKey.length === 0 ||
    runtimeKey.length > MAXIMUM_RUNTIME_KEY_LENGTH ||
    !RUNTIME_KEY_PATTERN.test(runtimeKey) ||
    typeof language !== "string" ||
    language.length === 0 ||
    language.length > MAXIMUM_LANGUAGE_ID_LENGTH ||
    !LANGUAGE_ID_PATTERN.test(language) ||
    typeof displayName !== "string" ||
    displayName.length === 0 ||
    displayName.length > MAXIMUM_DISPLAY_NAME_LENGTH ||
    displayName !== displayName.trim() ||
    CONTROL_CHARACTER_PATTERN.test(displayName) ||
    typeof sourceFilename !== "string" ||
    sourceFilename.length === 0 ||
    sourceFilename.length > MAXIMUM_SOURCE_FILENAME_LENGTH ||
    !SOURCE_FILENAME_PATTERN.test(sourceFilename) ||
    !Array.isArray(extensions) ||
    extensions.length === 0 ||
    extensions.length > MAXIMUM_EXTENSIONS_PER_LANGUAGE ||
    typeof compiled !== "boolean"
  ) {
    return invalidCatalog();
  }

  const normalizedExtensions: string[] = [];
  const seenExtensions = new Set<string>();
  for (const extension of extensions) {
    if (
      typeof extension !== "string" ||
      extension.length > MAXIMUM_EXTENSION_LENGTH ||
      !FILE_EXTENSION_PATTERN.test(extension)
    ) {
      return invalidCatalog();
    }
    const normalized = extension.toLowerCase();
    if (seenExtensions.has(normalized)) return invalidCatalog();
    seenExtensions.add(normalized);
    normalizedExtensions.push(normalized);
  }

  return Object.freeze({
    runtime_key: runtimeKey as RuntimeKey,
    language,
    display_name: displayName,
    source_filename: sourceFilename,
    file_extensions: Object.freeze(normalizedExtensions),
    compiled,
  });
}

/**
 * Validate the public runtime payload before it controls file reads or commands.
 * The API client generic is compile-time only, so this trust boundary is kept
 * explicit and returns a deeply immutable catalog.
 */
export function parseJudgeRuntimeCatalog(value: unknown): JudgeRuntimeCatalog {
  if (!isRecord(value)) return invalidCatalog();

  const {
    version,
    maximum_source_code_bytes: maximumSourceCodeBytes,
    runtimes,
  } = value;
  if (
    !hasExactKeys(value, [
      "version",
      "maximum_source_code_bytes",
      "runtimes",
    ]) ||
    !Number.isSafeInteger(version) ||
    (version as number) < 1 ||
    !Number.isSafeInteger(maximumSourceCodeBytes) ||
    (maximumSourceCodeBytes as number) < 1 ||
    (maximumSourceCodeBytes as number) > MAXIMUM_BROWSER_SOURCE_BYTES ||
    !Array.isArray(runtimes) ||
    runtimes.length === 0 ||
    runtimes.length > MAXIMUM_CATALOG_RUNTIMES
  ) {
    return invalidCatalog();
  }

  const parsedRuntimes = runtimes.map(parseRuntime);
  const runtimeKeys = new Set(
    parsedRuntimes.map((runtime) => runtime.runtime_key),
  );
  if (runtimeKeys.size !== parsedRuntimes.length) return invalidCatalog();

  return Object.freeze({
    version: version as number,
    maximum_source_code_bytes: maximumSourceCodeBytes as number,
    runtimes: Object.freeze(parsedRuntimes),
  });
}

export const judgeRuntimeCatalogSchema: Schema<JudgeRuntimeCatalog> = {
  parse(value: unknown): JudgeRuntimeCatalog {
    return parseJudgeRuntimeCatalog(value);
  },
};

export function sourceFileMatchesRuntime(
  filename: string,
  runtime: JudgeRuntime,
): boolean {
  const extensionStart = filename.lastIndexOf(".");
  if (extensionStart < 0) return false;
  const extension = filename.slice(extensionStart).toLowerCase();
  return runtime.file_extensions.some(
    (candidate) => extension === candidate.toLowerCase(),
  );
}

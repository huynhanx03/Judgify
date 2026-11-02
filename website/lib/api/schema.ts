/**
 * Small dependency-free runtime schema boundary.
 *
 * Domain modules may compose these primitives without coupling the product to
 * a validation framework. Network JSON is parsed exactly once here before a
 * DTO reaches React or a service consumer.
 */

export interface Schema<T> {
  parse(value: unknown, path?: string): T;
}

export type InferSchema<TSchema> =
  TSchema extends Schema<infer TValue> ? TValue : never;

export class ContractError extends TypeError {
  constructor(
    message: string,
    readonly path = "$",
  ) {
    super(`${path}: ${message}`);
    this.name = "ContractError";
  }
}

const FORBIDDEN_OBJECT_KEYS = new Set([
  "__proto__",
  "constructor",
  "prototype",
]);
const DEFAULT_MAXIMUM_JSON_CHARACTERS = 8 * 1024 * 1024;
const DEFAULT_MAXIMUM_DEPTH = 64;
const DEFAULT_MAXIMUM_NODES = 200_000;

function fail(path: string, message: string): never {
  throw new ContractError(message, path);
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return (
    typeof value === "object" &&
    value !== null &&
    !Array.isArray(value) &&
    (Object.getPrototypeOf(value) === Object.prototype ||
      Object.getPrototypeOf(value) === null)
  );
}

export function recordAt(
  value: unknown,
  path = "$",
): Record<string, unknown> {
  if (!isRecord(value)) fail(path, "expected an object");
  return value;
}

export function assertOnlyKeys(
  value: Record<string, unknown>,
  allowed: readonly string[],
  path = "$",
): void {
  const allowlist = new Set(allowed);
  for (const key of Object.keys(value)) {
    if (!allowlist.has(key)) fail(`${path}.${key}`, "unknown field");
  }
}

export function stringSchema(options: {
  minimumLength?: number;
  maximumLength?: number;
  pattern?: RegExp;
  label?: string;
} = {}): Schema<string> {
  const {
    minimumLength = 0,
    maximumLength = Number.MAX_SAFE_INTEGER,
    pattern,
    label = "string",
  } = options;
  return {
    parse(value: unknown, path = "$"): string {
      if (typeof value !== "string") fail(path, `expected ${label}`);
      if (
        value.length < minimumLength ||
        value.length > maximumLength ||
        (pattern && !pattern.test(value))
      ) {
        fail(path, `invalid ${label}`);
      }
      return value;
    },
  };
}

export function integerSchema(options: {
  minimum?: number;
  maximum?: number;
  label?: string;
} = {}): Schema<number> {
  const {
    minimum = Number.MIN_SAFE_INTEGER,
    maximum = Number.MAX_SAFE_INTEGER,
    label = "integer",
  } = options;
  return {
    parse(value: unknown, path = "$"): number {
      if (
        typeof value !== "number" ||
        !Number.isSafeInteger(value) ||
        value < minimum ||
        value > maximum
      ) {
        fail(path, `invalid ${label}`);
      }
      return value;
    },
  };
}

export const booleanSchema: Schema<boolean> = {
  parse(value: unknown, path = "$"): boolean {
    if (typeof value !== "boolean") fail(path, "expected a boolean");
    return value;
  },
};

export function literalSchema<const TValue extends string | number | boolean>(
  expected: TValue,
): Schema<TValue> {
  return {
    parse(value: unknown, path = "$"): TValue {
      if (value !== expected) fail(path, `expected literal ${String(expected)}`);
      return expected;
    },
  };
}

export function enumSchema<const TValues extends readonly string[]>(
  values: TValues,
): Schema<TValues[number]> {
  const allowed = new Set<string>(values);
  return {
    parse(value: unknown, path = "$"): TValues[number] {
      if (typeof value !== "string" || !allowed.has(value)) {
        fail(path, "invalid enum value");
      }
      return value as TValues[number];
    },
  };
}

export function optionalSchema<T>(schema: Schema<T>): Schema<T | undefined> {
  return {
    parse(value: unknown, path = "$"): T | undefined {
      return value === undefined ? undefined : schema.parse(value, path);
    },
  };
}

export function nullableSchema<T>(schema: Schema<T>): Schema<T | null> {
  return {
    parse(value: unknown, path = "$"): T | null {
      return value === null ? null : schema.parse(value, path);
    },
  };
}

export function arraySchema<T>(
  item: Schema<T>,
  options: {
    minimumLength?: number;
    maximumLength?: number;
    unique?: boolean;
  } = {},
): Schema<T[]> {
  const {
    minimumLength = 0,
    maximumLength = 10_000,
    unique = false,
  } = options;
  if (
    !Number.isSafeInteger(minimumLength) ||
    !Number.isSafeInteger(maximumLength) ||
    minimumLength < 0 ||
    maximumLength < minimumLength
  ) {
    throw new RangeError("invalid array schema boundary");
  }
  return {
    parse(value: unknown, path = "$"): T[] {
      if (!Array.isArray(value)) fail(path, "expected an array");
      if (value.length < minimumLength) fail(path, "array is below boundary");
      if (value.length > maximumLength) fail(path, "array exceeds boundary");
      const parsed = value.map((entry, index) =>
        item.parse(entry, `${path}[${index}]`),
      );
      if (unique && new Set(parsed).size !== parsed.length) {
        fail(path, "array entries must be unique");
      }
      return parsed;
    },
  };
}

export type ObjectShape = Record<string, Schema<unknown>>;
export type ObjectOutput<TShape extends ObjectShape> = {
  [TKey in keyof TShape]: InferSchema<TShape[TKey]>;
};

export function strictObjectSchema<const TShape extends ObjectShape>(
  shape: TShape,
): Schema<ObjectOutput<TShape>> {
  const keys = Object.keys(shape);
  return {
    parse(value: unknown, path = "$"): ObjectOutput<TShape> {
      const source = recordAt(value, path);
      assertOnlyKeys(source, keys, path);
      const output: Record<string, unknown> = {};
      for (const key of keys) {
        output[key] = shape[key].parse(source[key], `${path}.${key}`);
      }
      return output as ObjectOutput<TShape>;
    },
  };
}

export const voidSchema: Schema<void> = {
  parse(value: unknown, path = "$"): void {
    if (value !== null && value !== undefined) {
      fail(path, "expected an empty response");
    }
  },
};

interface SafetyBudget {
  nodes: number;
}

function assertJsonValue(
  value: unknown,
  path: string,
  depth: number,
  budget: SafetyBudget,
): void {
  budget.nodes += 1;
  if (budget.nodes > DEFAULT_MAXIMUM_NODES) {
    fail(path, "JSON document exceeds node boundary");
  }
  if (depth > DEFAULT_MAXIMUM_DEPTH) {
    fail(path, "JSON document exceeds nesting boundary");
  }

  if (
    value === null ||
    typeof value === "string" ||
    typeof value === "boolean"
  ) {
    return;
  }
  if (typeof value === "number") {
    if (!Number.isFinite(value)) fail(path, "JSON number must be finite");
    return;
  }
  if (Array.isArray(value)) {
    value.forEach((entry, index) =>
      assertJsonValue(entry, `${path}[${index}]`, depth + 1, budget),
    );
    return;
  }
  if (!isRecord(value)) fail(path, "unsupported JSON value");
  for (const [key, child] of Object.entries(value)) {
    if (FORBIDDEN_OBJECT_KEYS.has(key)) {
      fail(`${path}.${key}`, "unsafe object key");
    }
    assertJsonValue(child, `${path}.${key}`, depth + 1, budget);
  }
}

export const jsonValueSchema: Schema<unknown> = {
  parse(value: unknown, path = "$"): unknown {
    assertJsonValue(value, path, 0, { nodes: 0 });
    return value;
  },
};

function skipWhitespace(source: string, start: number): number {
  let index = start;
  while (
    index < source.length &&
    (source[index] === " " ||
      source[index] === "\n" ||
      source[index] === "\r" ||
      source[index] === "\t")
  ) {
    index += 1;
  }
  return index;
}

function scanString(source: string, start: number): number {
  if (source[start] !== '"') fail("$", "invalid JSON string");
  let index = start + 1;
  while (index < source.length) {
    const character = source[index];
    if (character === '"') return index + 1;
    if (character === "\\") {
      index += 1;
      if (index >= source.length) fail("$", "invalid JSON escape");
      if (source[index] === "u") {
        const codepoint = source.slice(index + 1, index + 5);
        if (!/^[0-9a-fA-F]{4}$/.test(codepoint)) {
          fail("$", "invalid JSON unicode escape");
        }
        index += 4;
      }
    } else if (character.charCodeAt(0) < 0x20) {
      fail("$", "invalid JSON control character");
    }
    index += 1;
  }
  return fail("$", "unterminated JSON string");
}

function scanPrimitive(source: string, start: number): number {
  const remainder = source.slice(start);
  const match = /^(?:true|false|null|-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?)/.exec(
    remainder,
  );
  if (!match) fail("$", "invalid JSON value");
  return start + match[0].length;
}

function scanValue(
  source: string,
  start: number,
  depth: number,
  budget: SafetyBudget,
): number {
  budget.nodes += 1;
  if (budget.nodes > DEFAULT_MAXIMUM_NODES) {
    fail("$", "JSON document exceeds node boundary");
  }
  if (depth > DEFAULT_MAXIMUM_DEPTH) {
    fail("$", "JSON document exceeds nesting boundary");
  }

  let index = skipWhitespace(source, start);
  if (source[index] === '"') return scanString(source, index);
  if (source[index] === "[") {
    index = skipWhitespace(source, index + 1);
    if (source[index] === "]") return index + 1;
    while (index < source.length) {
      index = scanValue(source, index, depth + 1, budget);
      index = skipWhitespace(source, index);
      if (source[index] === "]") return index + 1;
      if (source[index] !== ",") fail("$", "invalid JSON array");
      index = skipWhitespace(source, index + 1);
    }
    return fail("$", "unterminated JSON array");
  }
  if (source[index] === "{") {
    const keys = new Set<string>();
    index = skipWhitespace(source, index + 1);
    if (source[index] === "}") return index + 1;
    while (index < source.length) {
      if (source[index] !== '"') fail("$", "invalid JSON object key");
      const keyEnd = scanString(source, index);
      const key = JSON.parse(source.slice(index, keyEnd)) as string;
      if (keys.has(key)) fail(`$.${key}`, "duplicate JSON object key");
      keys.add(key);
      index = skipWhitespace(source, keyEnd);
      if (source[index] !== ":") fail("$", "invalid JSON object");
      index = scanValue(source, index + 1, depth + 1, budget);
      index = skipWhitespace(source, index);
      if (source[index] === "}") return index + 1;
      if (source[index] !== ",") fail("$", "invalid JSON object");
      index = skipWhitespace(source, index + 1);
    }
    return fail("$", "unterminated JSON object");
  }
  return scanPrimitive(source, index);
}

/**
 * JSON.parse accepts duplicate keys with last-write-wins semantics. Contract
 * parsing rejects them first so intermediaries cannot reinterpret a payload
 * differently from the browser.
 */
export function parseJsonDocument(
  source: string,
  maximumCharacters = DEFAULT_MAXIMUM_JSON_CHARACTERS,
): unknown {
  if (
    !Number.isSafeInteger(maximumCharacters) ||
    maximumCharacters < 1 ||
    maximumCharacters > 64 * 1024 * 1024
  ) {
    throw new RangeError("JSON size boundary is invalid");
  }
  if (source.length > maximumCharacters) {
    fail("$", "JSON document exceeds size boundary");
  }
  const end = skipWhitespace(
    source,
    scanValue(source, 0, 0, { nodes: 0 }),
  );
  if (end !== source.length) fail("$", "unexpected data after JSON document");
  const parsed: unknown = JSON.parse(source);
  return jsonValueSchema.parse(parsed);
}

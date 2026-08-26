import { jsonValueSchema } from "@/lib/api/schema";

const COMMAND_PURPOSE_VERSION = "judgify.command-purpose.v1";
const NAMESPACE_PATTERN = /^[a-z][a-z0-9-]{0,47}$/;
const MAXIMUM_CANONICAL_CHARACTERS = 1024 * 1024;

function bytesToHex(value: ArrayBuffer): string {
  return Array.from(new Uint8Array(value), (byte) =>
    byte.toString(16).padStart(2, "0"),
  ).join("");
}

/**
 * Produces a privacy-safe retry scope from the complete semantic command.
 * Only the SHA-256 digest is persisted by CommandAttemptStore.
 */
export async function commandPurpose(
  namespace: string,
  semanticCommand: unknown,
): Promise<string> {
  if (!NAMESPACE_PATTERN.test(namespace)) {
    throw new TypeError("invalid command-purpose namespace");
  }
  jsonValueSchema.parse(semanticCommand);
  const canonical = JSON.stringify([
    COMMAND_PURPOSE_VERSION,
    namespace,
    semanticCommand,
  ]);
  if (
    canonical.length < 1 ||
    canonical.length > MAXIMUM_CANONICAL_CHARACTERS
  ) {
    throw new TypeError("command purpose exceeds the supported boundary");
  }
  if (!globalThis.crypto?.subtle) {
    throw new TypeError("secure command fingerprinting is unavailable");
  }
  const digest = await globalThis.crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(canonical),
  );
  return `${namespace}-${bytesToHex(digest)}`;
}


import type { CreateSubmissionRequest } from "@/types/submission";

const SUBMISSION_PURPOSE_VERSION = "judgify.submission-command-purpose.v1";

function isWellFormed(value: string): boolean {
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index);
    if (code >= 0xd800 && code <= 0xdbff) {
      const next = value.charCodeAt(index + 1);
      if (next < 0xdc00 || next > 0xdfff) return false;
      index += 1;
      continue;
    }
    if (code >= 0xdc00 && code <= 0xdfff) return false;
  }
  return true;
}

function bytesToHex(value: ArrayBuffer): string {
  return Array.from(new Uint8Array(value), (byte) =>
    byte.toString(16).padStart(2, "0"),
  ).join("");
}

/**
 * Derives a privacy-safe browser retry scope from the complete semantic
 * command. Only its SHA-256 digest reaches storage; source code never does.
 * The API independently binds the same fields to its durable request hash.
 */
export async function submissionCommandPurpose(
  command: CreateSubmissionRequest,
): Promise<string> {
  const semanticValues = [
    command.problem_id,
    command.contest_id ?? null,
    command.contest_problem_id ?? null,
    command.runtime_key,
    command.source_code,
  ] as const;
  if (
    semanticValues.some(
      (value) =>
        value !== null &&
        (typeof value !== "string" ||
          value.length === 0 ||
          !isWellFormed(value)),
    ) ||
    (command.contest_id === undefined) !==
      (command.contest_problem_id === undefined)
  ) {
    throw new TypeError("invalid submission command purpose");
  }
  if (!globalThis.crypto?.subtle) {
    throw new TypeError("secure command fingerprinting is unavailable");
  }
  const canonical = JSON.stringify([
    SUBMISSION_PURPOSE_VERSION,
    ...semanticValues,
  ]);
  const digest = await globalThis.crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(canonical),
  );
  return `submission-${bytesToHex(digest)}`;
}

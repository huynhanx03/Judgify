import { JUDGE_CONTENT_LIMITS } from "@/constants/submission";

export type JudgeContentFailure =
  | "required"
  | "contains_nul"
  | "too_large";

export interface JudgeContentInspection {
  valid: boolean;
  reason: JudgeContentFailure | null;
  actualBytes: number;
  maximumBytes: number;
}

const encoder = new TextEncoder();
const strictDecoder = new TextDecoder("utf-8", { fatal: true });

export function utf8ByteLength(value: string): number {
  return encoder.encode(value).byteLength;
}

export function inspectJudgeContent(
  value: string,
  maximumBytes: number,
): JudgeContentInspection {
  const actualBytes = utf8ByteLength(value);
  let reason: JudgeContentFailure | null = null;
  if (actualBytes === 0) reason = "required";
  else if (value.includes("\0")) reason = "contains_nul";
  else if (actualBytes > maximumBytes) reason = "too_large";

  return {
    valid: reason === null,
    reason,
    actualBytes,
    maximumBytes,
  };
}

export function inspectSourceCode(value: string): JudgeContentInspection {
  return inspectJudgeContent(
    value,
    JUDGE_CONTENT_LIMITS.MAXIMUM_SOURCE_CODE_BYTES,
  );
}

export function inspectTestInput(value: string): JudgeContentInspection {
  return inspectJudgeContent(
    value,
    JUDGE_CONTENT_LIMITS.MAXIMUM_TEST_INPUT_BYTES,
  );
}

export function inspectExpectedOutput(value: string): JudgeContentInspection {
  return inspectJudgeContent(
    value,
    JUDGE_CONTENT_LIMITS.MAXIMUM_EXPECTED_OUTPUT_BYTES,
  );
}

export function decodeSourceUTF8(bytes: AllowSharedBufferSource): string {
  try {
    return strictDecoder.decode(bytes);
  } catch (cause) {
    const error = new Error("source file is not valid UTF-8", { cause });
    error.name = "JudgeSourceDecodeError";
    throw error;
  }
}

export function formatJudgeBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const kibibytes = bytes / 1024;
  if (kibibytes < 1024) {
    return `${Number.isInteger(kibibytes) ? kibibytes : kibibytes.toFixed(1)} KiB`;
  }
  const mebibytes = kibibytes / 1024;
  return `${Number.isInteger(mebibytes) ? mebibytes : mebibytes.toFixed(1)} MiB`;
}

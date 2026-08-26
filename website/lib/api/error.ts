import {
  correlationIDSchema,
  type ApiErrorBody,
  type CorrelationID,
} from "@/lib/api/contracts";
import {
  assertOnlyKeys,
  integerSchema,
  isRecord,
  recordAt,
  stringSchema,
} from "@/lib/api/schema";

const SAFE_KEY_PATTERN = /^[a-z][a-z0-9_.-]{0,63}$/;
const SENSITIVE_KEY_PATTERN =
  /(?:^|[_.-])(access|refresh)?_?(token|secret|password|cookie|csrf|credential)(?:$|[_.-])/i;
const MAXIMUM_ERROR_PARAMS = 32;
const MAXIMUM_ERROR_FIELDS = 64;

export interface ApiErrorOptions {
  code: string;
  status: number;
  cid: string;
  params?: Record<string, string | number | boolean>;
  fields?: Record<string, string>;
  retryable?: boolean;
  cause?: unknown;
}

export class ApiError extends Error {
  readonly code: string;
  readonly status: number;
  readonly cid: CorrelationID;
  readonly params?: Readonly<Record<string, string | number | boolean>>;
  readonly fields?: Readonly<Record<string, string>>;
  readonly retryable: boolean;

  constructor(options: ApiErrorOptions) {
    super(options.code, options.cause === undefined ? undefined : {
      cause: options.cause,
    });
    this.name = "ApiError";
    this.code = safeCode(options.code, "$.error.code");
    this.status = integerSchema({ minimum: 400, maximum: 599 }).parse(
      options.status,
      "$.status",
    );
    this.cid = correlationIDSchema.parse(options.cid, "$.meta.cid");
    this.params = options.params
      ? Object.freeze({ ...parseSafeParams(options.params, "$.error.params") })
      : undefined;
    this.fields = options.fields
      ? Object.freeze({ ...parseSafeFields(options.fields, "$.error.fields") })
      : undefined;
    this.retryable =
      options.retryable ?? isRetryableHTTPStatus(this.status);
  }
}

function safeCode(value: unknown, path: string): string {
  const code = stringSchema({
    minimumLength: 1,
    maximumLength: 96,
    pattern: SAFE_KEY_PATTERN,
    label: "machine error code",
  }).parse(value, path);
  if (SENSITIVE_KEY_PATTERN.test(code)) {
    throw new TypeError(`${path}: sensitive error code is not allowed`);
  }
  return code;
}

function assertSafeKey(key: string, path: string): void {
  if (!SAFE_KEY_PATTERN.test(key)) {
    throw new TypeError(`${path}: unsafe error detail key`);
  }
  if (SENSITIVE_KEY_PATTERN.test(key)) {
    throw new TypeError(`${path}: sensitive error detail key`);
  }
}

function parseSafeParams(
  value: unknown,
  path: string,
): Record<string, string | number | boolean> {
  const record = recordAt(value, path);
  const entries = Object.entries(record);
  if (entries.length > MAXIMUM_ERROR_PARAMS) {
    throw new TypeError(`${path}: too many error parameters`);
  }
  const parsed: Record<string, string | number | boolean> = {};
  for (const [key, entry] of entries) {
    assertSafeKey(key, `${path}.${key}`);
    if (
      typeof entry !== "string" &&
      typeof entry !== "number" &&
      typeof entry !== "boolean"
    ) {
      throw new TypeError(`${path}.${key}: unsafe error parameter value`);
    }
    if (
      (typeof entry === "string" && entry.length > 512) ||
      (typeof entry === "number" && !Number.isFinite(entry))
    ) {
      throw new TypeError(`${path}.${key}: error parameter exceeds boundary`);
    }
    parsed[key] = entry;
  }
  return parsed;
}

function parseSafeFields(
  value: unknown,
  path: string,
): Record<string, string> {
  const record = recordAt(value, path);
  const entries = Object.entries(record);
  if (entries.length > MAXIMUM_ERROR_FIELDS) {
    throw new TypeError(`${path}: too many field errors`);
  }
  const parsed: Record<string, string> = {};
  for (const [key, entry] of entries) {
    assertSafeKey(key, `${path}.${key}`);
    parsed[key] = safeCode(entry, `${path}.${key}`);
  }
  return parsed;
}

function parseErrorBody(value: unknown, path: string): ApiErrorBody {
  const record = recordAt(value, path);
  assertOnlyKeys(record, ["code", "params", "fields"], path);
  return {
    code: safeCode(record.code, `${path}.code`),
    ...(record.params === undefined
      ? {}
      : { params: parseSafeParams(record.params, `${path}.params`) }),
    ...(record.fields === undefined
      ? {}
      : { fields: parseSafeFields(record.fields, `${path}.fields`) }),
  };
}

function legacyErrorBody(
  record: Record<string, unknown>,
): ApiErrorBody {
  const numericCode = integerSchema({
    minimum: 1,
    maximum: Number.MAX_SAFE_INTEGER,
  }).parse(record.code, "$.code");
  stringSchema({ minimumLength: 1, maximumLength: 512 }).parse(
    record.message,
    "$.message",
  );

  let params: Record<string, string | number | boolean> | undefined;
  let fields: Record<string, string> | undefined;
  if (isRecord(record.data)) {
	const scalarParams: Record<string, string | number | boolean> = {};
	for (const [key, value] of Object.entries(record.data)) {
	  if (key === "params" || key === "fields") continue;
	  if (
	    typeof value === "string" ||
	    typeof value === "number" ||
	    typeof value === "boolean"
	  ) {
	    scalarParams[key] = value;
	  }
	}
    if ("params" in record.data) {
	  params = {
	    ...parseSafeParams(scalarParams, "$.data"),
	    ...parseSafeParams(record.data.params, "$.data.params"),
	  };
	} else if (Object.keys(scalarParams).length > 0) {
	  params = parseSafeParams(scalarParams, "$.data");
    }
    if ("fields" in record.data) {
      fields = parseSafeFields(record.data.fields, "$.data.fields");
    }
    // Even ignored legacy detail must never smuggle a credential-looking key.
    for (const key of Object.keys(record.data)) {
      if (SENSITIVE_KEY_PATTERN.test(key)) {
        throw new TypeError(`$.data.${key}: sensitive error detail key`);
      }
    }
  }
  return {
    code: `legacy_${numericCode}`,
    ...(params ? { params } : {}),
    ...(fields ? { fields } : {}),
  };
}

export function isRetryableHTTPStatus(status: number): boolean {
  return (
    status === 408 ||
    status === 425 ||
    status === 429 ||
    (status >= 500 && status <= 599)
  );
}

export function parseApiErrorEnvelope(
  value: unknown,
  status: number,
  responseCID?: string,
): ApiError {
  const record = recordAt(value);

  if ("error" in record && "meta" in record) {
    assertOnlyKeys(record, ["error", "meta"]);
    const meta = recordAt(record.meta, "$.meta");
    assertOnlyKeys(meta, ["cid"], "$.meta");
    const cid = correlationIDSchema.parse(
      meta.cid ?? responseCID,
      "$.meta.cid",
    );
    if (
      responseCID &&
      correlationIDSchema.parse(responseCID) !== cid
    ) {
      throw new TypeError("$.meta.cid: response header/body CID mismatch");
    }
    const body = parseErrorBody(record.error, "$.error");
    return new ApiError({ ...body, status, cid });
  }

  assertOnlyKeys(record, ["code", "message", "data", "pagination", "cid"]);
  const body = legacyErrorBody(record);
  const bodyCID =
    record.cid === undefined ? undefined : correlationIDSchema.parse(record.cid);
  if (bodyCID && responseCID && bodyCID !== responseCID) {
    throw new TypeError("$.cid: response header/body CID mismatch");
  }
  return new ApiError({
    ...body,
    status,
    cid: bodyCID ?? correlationIDSchema.parse(responseCID, "$.cid"),
  });
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError;
}

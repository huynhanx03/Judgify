import {
  arraySchema,
  assertOnlyKeys,
  booleanSchema,
  enumSchema,
  integerSchema,
  jsonValueSchema,
  nullableSchema,
  optionalSchema,
  recordAt,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";

declare const entityIDBrand: unique symbol;
declare const correlationIDBrand: unique symbol;
declare const isoDateTimeBrand: unique symbol;
declare const cursorBrand: unique symbol;

export type EntityID = string & {
  readonly [entityIDBrand]: "EntityID";
};
export type CorrelationID = string & {
  readonly [correlationIDBrand]: "CorrelationID";
};
export type ISODateTime = string & {
  readonly [isoDateTimeBrand]: "ISODateTime";
};
export type Cursor = string & {
  readonly [cursorBrand]: "Cursor";
};

const CANONICAL_UUID_PATTERN =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
const ISO_DATE_TIME_PATTERN =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/;
const CURSOR_PATTERN = /^[A-Za-z0-9._~+/=-]+$/;

export const entityIDSchema: Schema<EntityID> = {
  parse(value: unknown, path = "$"): EntityID {
    if (typeof value !== "string" || !CANONICAL_UUID_PATTERN.test(value)) {
      throw new TypeError(
        `${path}: expected a canonical UUID EntityID`,
      );
    }
    return value as EntityID;
  },
};

export function tryEntityID(value: unknown): EntityID | null {
  try {
    return entityIDSchema.parse(value);
  } catch {
    return null;
  }
}

export const correlationIDSchema: Schema<CorrelationID> = {
  parse(value: unknown, path = "$"): CorrelationID {
    if (typeof value !== "string" || !CANONICAL_UUID_PATTERN.test(value)) {
      throw new TypeError(`${path}: expected a canonical UUID CID`);
    }
    return value as CorrelationID;
  },
};

export const isoDateTimeSchema: Schema<ISODateTime> = {
  parse(value: unknown, path = "$"): ISODateTime {
    if (
      typeof value !== "string" ||
      !ISO_DATE_TIME_PATTERN.test(value) ||
      Number.isNaN(Date.parse(value))
    ) {
      throw new TypeError(`${path}: invalid RFC 3339 timestamp`);
    }
    return value as ISODateTime;
  },
};

/**
 * Compares RFC 3339 instants without losing the six sub-millisecond digits
 * that JavaScript Date truncates. Network schemas parse both values before
 * calling this helper, so offsets and nanosecond fractions remain exact.
 */
export function compareISODateTime(
  left: ISODateTime,
  right: ISODateTime,
): -1 | 0 | 1 {
  const leftNanoseconds = isoDateTimeEpochNanoseconds(left);
  const rightNanoseconds = isoDateTimeEpochNanoseconds(right);
  if (leftNanoseconds < rightNanoseconds) return -1;
  if (leftNanoseconds > rightNanoseconds) return 1;
  return 0;
}

function isoDateTimeEpochNanoseconds(value: ISODateTime): bigint {
  const fractionMatch = /\.(\d{1,9})(?=Z|[+-]\d{2}:\d{2}$)/.exec(value);
  const wholeSecond = fractionMatch
    ? value.replace(`.${fractionMatch[1]}`, "")
    : value;
  const epochMilliseconds = Date.parse(wholeSecond);
  if (!Number.isSafeInteger(epochMilliseconds)) {
    throw new TypeError("invalid RFC 3339 timestamp boundary");
  }
  const fraction = (fractionMatch?.[1] ?? "").padEnd(9, "0");
  return (
    BigInt(epochMilliseconds) * BigInt(1_000_000) +
    BigInt(fraction || "0")
  );
}

export const cursorSchema: Schema<Cursor> = {
  parse(value: unknown, path = "$"): Cursor {
    if (
      typeof value !== "string" ||
      value.length < 1 ||
      value.length > 512 ||
      !CURSOR_PATTERN.test(value)
    ) {
      throw new TypeError(`${path}: invalid cursor`);
    }
    return value as Cursor;
  },
};

export { enumSchema };

export interface PaginationMeta {
  current_page: number;
  page_size: number;
  total_pages: number;
  total_items: number;
  has_next: boolean;
  has_prev: boolean;
}

const rawPaginationMetaSchema = strictObjectSchema({
  current_page: integerSchema({ minimum: 1, label: "pagination current page" }),
  page_size: integerSchema({
    minimum: 1,
    maximum: 100,
    label: "pagination page size",
  }),
  total_pages: integerSchema({
    minimum: 1,
    label: "pagination total pages",
  }),
  total_items: integerSchema({
    minimum: 0,
    label: "pagination total items",
  }),
  has_next: booleanSchema,
  has_prev: booleanSchema,
});

export const paginationMetaSchema: Schema<PaginationMeta> = {
  parse(value: unknown, path = "$"): PaginationMeta {
    const pagination = rawPaginationMetaSchema.parse(value, path);
    if (pagination.current_page > pagination.total_pages) {
      throw new TypeError(`${path}: invalid pagination current page`);
    }
    if (pagination.has_prev !== (pagination.current_page > 1)) {
      throw new TypeError(`${path}: inconsistent pagination has_prev`);
    }
    if (pagination.has_next !== (pagination.current_page < pagination.total_pages)) {
      throw new TypeError(`${path}: inconsistent pagination has_next`);
    }
    return pagination;
  },
};

export interface ApiMeta {
  cid: CorrelationID;
  pagination?: PaginationMeta;
  cursor?: Cursor;
}

export interface ApiEnvelope<T> {
  data: T;
  meta: ApiMeta;
}

export interface ApiErrorBody {
  code: string;
  params?: Record<string, string | number | boolean>;
  fields?: Record<string, string>;
}

export interface ApiErrorEnvelope {
  error: ApiErrorBody;
  meta: { cid: CorrelationID };
}

export interface Paginated<T> {
  records: T[];
  pagination: PaginationMeta;
}

export interface SearchFilter {
  key: string;
  value: unknown;
  type: "search" | "filter" | "exact";
}

export interface SortOption {
  key: string;
  order: 1 | -1;
}

export interface QueryOptions {
  pagination?: {
    page: number;
    page_size: number;
  };
  filters?: SearchFilter[];
  sort?: SortOption[];
}

function parseMeta(value: unknown, fallbackCID?: string): ApiMeta {
  const record = value === undefined ? {} : recordAt(value, "$.meta");
  assertOnlyKeys(record, ["cid", "pagination", "cursor"], "$.meta");
  const cid = correlationIDSchema.parse(record.cid ?? fallbackCID, "$.meta.cid");
  const pagination =
    record.pagination === undefined
      ? undefined
      : paginationMetaSchema.parse(record.pagination, "$.meta.pagination");
  const cursor =
    record.cursor === undefined || record.cursor === null
      ? undefined
      : cursorSchema.parse(record.cursor, "$.meta.cursor");
  return {
    cid,
    ...(pagination ? { pagination } : {}),
    ...(cursor ? { cursor } : {}),
  };
}

function normalizePaginatedData(
  data: unknown,
  pagination: PaginationMeta | undefined,
): unknown {
  if (!pagination || !Array.isArray(data)) return data;
  return { records: data, pagination };
}

/**
 * Accepts the approved `{data,meta}` envelope and the current go-common
 * `{code,message,data,pagination,cid}` shape during the coordinated backend
 * cutover. Both normalize to one strict frontend contract.
 */
export function parseApiSuccessEnvelope<T>(
  value: unknown,
  schema: Schema<T>,
  responseCID?: string,
): ApiEnvelope<T> {
  const record = recordAt(value);

  if ("meta" in record && !("code" in record)) {
    assertOnlyKeys(record, ["data", "meta"]);
    const meta = parseMeta(record.meta, responseCID);
    if (
      responseCID &&
      correlationIDSchema.parse(responseCID) !== meta.cid
    ) {
      throw new TypeError("$.meta.cid: response header/body CID mismatch");
    }
    return {
      data: schema.parse(
        normalizePaginatedData(record.data, meta.pagination),
        "$.data",
      ),
      meta,
    };
  }

  assertOnlyKeys(record, ["code", "message", "data", "pagination", "cid"]);
  const code = integerSchema({ minimum: 20_000, maximum: 29_999 }).parse(
    record.code,
    "$.code",
  );
  if (code < 20_000 || code >= 30_000) {
    throw new TypeError("$.code: legacy response is not successful");
  }
  stringSchema({ minimumLength: 1, maximumLength: 512 }).parse(
    record.message,
    "$.message",
  );
  const bodyCID =
    record.cid === undefined || record.cid === null
      ? undefined
      : correlationIDSchema.parse(record.cid, "$.cid");
  if (bodyCID && responseCID && bodyCID !== responseCID) {
    throw new TypeError("$.cid: response header/body CID mismatch");
  }
  const pagination =
    record.pagination === undefined || record.pagination === null
      ? undefined
      : paginationMetaSchema.parse(record.pagination, "$.pagination");
  const meta = parseMeta(
    {
      cid: bodyCID ?? responseCID,
      ...(pagination ? { pagination } : {}),
    },
    responseCID,
  );
  return {
    data: schema.parse(
      normalizePaginatedData(record.data, pagination),
      "$.data",
    ),
    meta,
  };
}

export function paginatedSchema<T>(item: Schema<T>): Schema<Paginated<T>> {
  return strictObjectSchema({
    records: arraySchema(item),
    pagination: paginationMetaSchema,
  });
}

export const unknownApiDataSchema = jsonValueSchema;

export const optionalCursorSchema = optionalSchema(nullableSchema(cursorSchema));

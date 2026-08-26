import { ADMIN_TEXT } from "@/constants/admin-text";
import { tryEntityID } from "@/lib/api/contracts";
import type { AttributeDataType } from "@/types/admin-auxiliary";

/** Largest integer that round-trips exactly through JSON and JavaScript. */
export const JSON_SAFE_INTEGER_MAX = Number.MAX_SAFE_INTEGER;

export function parseNonnegativeSafeInteger(value: string): number | null {
  const parsed = Number(value);
  return value.trim() !== "" && Number.isSafeInteger(parsed) && parsed >= 0
    ? parsed
    : null;
}

export function isValidEntityIDSearch(value: string): boolean {
  return tryEntityID(value) !== null;
}

const ATTRIBUTE_DATA_TYPE_LABELS: Record<AttributeDataType, string> = {
  string: ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.TYPE_STRING,
  number: ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.TYPE_NUMBER,
  boolean: ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.TYPE_BOOLEAN,
  date: ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.TYPE_DATE,
};

export function attributeDataTypeLabel(value: AttributeDataType): string {
  return ATTRIBUTE_DATA_TYPE_LABELS[value];
}

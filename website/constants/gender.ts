import { TEXT } from "./text";

export const GENDER = {
  MALE: 0,
  FEMALE: 1,
  OTHER: 2,
} as const;

export type Gender = (typeof GENDER)[keyof typeof GENDER];

export interface GenderOption {
  value: Gender;
  label: string;
}

export const GENDER_OPTIONS = [
  { value: GENDER.MALE, label: TEXT.AUTH.GENDER_MALE },
  { value: GENDER.FEMALE, label: TEXT.AUTH.GENDER_FEMALE },
  { value: GENDER.OTHER, label: TEXT.AUTH.GENDER_OTHER },
] as const satisfies readonly GenderOption[];

export function parseGender(value: unknown): Gender | null {
  if (typeof value !== "number" && typeof value !== "string") return null;
  if (typeof value === "string" && value.trim() === "") return null;

  const parsed = typeof value === "number" ? value : Number(value);
  return GENDER_OPTIONS.some((option) => option.value === parsed)
    ? (parsed as Gender)
    : null;
}

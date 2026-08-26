import type { vi } from "@/i18n/catalog.vi";
import type { admin } from "@/i18n/catalog.admin.vi";

export type InterpolationValue = string | number | boolean | bigint;

type Join<Prefix extends string, Key extends string> = Prefix extends ""
  ? Key
  : `${Prefix}.${Key}`;

/** Dot-delimited paths that terminate at a string or formatter function. */
export type LeafPaths<T, Prefix extends string = ""> = {
  [Key in keyof T & string]: T[Key] extends string
    ? Join<Prefix, Key>
    : T[Key] extends (...args: infer _Arguments) => unknown
      ? Join<Prefix, Key>
      : T[Key] extends object
        ? LeafPaths<T[Key], Join<Prefix, Key>>
        : never;
}[keyof T & string];

export type PathValue<T, Path extends string> = Path extends keyof T
  ? T[Path]
  : Path extends `${infer Head}.${infer Tail}`
    ? Head extends keyof T
      ? PathValue<T[Head], Tail>
      : never
    : never;

type PlaceholderNames<Value extends string> =
  Value extends `${string}{${infer Name}}${infer Rest}`
    ? Name | PlaceholderNames<Rest>
    : never;

type NamedParams<Value extends string> = [PlaceholderNames<Value>] extends [never]
  ? undefined
  : Record<PlaceholderNames<Value>, InterpolationValue>;

type CatalogParamsFor<C, Key extends LeafPaths<C>> = PathValue<
  C,
  Key
> extends (...args: infer Arguments) => unknown
  ? { args: Arguments }
  : PathValue<C, Key> extends string
    ? NamedParams<PathValue<C, Key>>
    : never;

type CatalogTextArguments<C, Key extends LeafPaths<C>> =
  CatalogParamsFor<C, Key> extends undefined
    ? [params?: undefined]
    : [params: CatalogParamsFor<C, Key>];

// Public catalog types (unchanged public API)
export type TextKey = LeafPaths<typeof vi>;
export type ParamsFor<Key extends TextKey> = CatalogParamsFor<typeof vi, Key>;
export type TextArguments<Key extends TextKey> = CatalogTextArguments<typeof vi, Key>;

// Admin catalog types
export type AdminTextKey = LeafPaths<typeof admin>;
export type AdminParamsFor<Key extends AdminTextKey> = CatalogParamsFor<typeof admin, Key>;
export type AdminTextArguments<Key extends AdminTextKey> = CatalogTextArguments<typeof admin, Key>;

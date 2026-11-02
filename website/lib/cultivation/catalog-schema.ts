import {
  entityIDSchema,
  paginatedSchema,
} from "@/lib/api/contracts";
import {
  arraySchema,
  integerSchema,
  optionalSchema,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import type {
  ElementResponse,
  LevelResponse,
  RankResponse,
  RarityResponse,
} from "@/types/cultivation";
import type { Paginated } from "@/types/api";

const MAXIMUM_CATALOG_ITEMS = 1_000;
const MAXIMUM_SAFE_API_INTEGER = Number.MAX_SAFE_INTEGER;

type CatalogRecord = { id: string };

function assertUniqueCatalogRecords<T extends CatalogRecord>(
  records: T[],
  path: string,
): void {
  const identities = new Set<string>();
  for (const record of records) {
    if (identities.has(record.id)) {
      throw new TypeError(`${path}: duplicate catalog identity`);
    }
    identities.add(record.id);
  }
}

export function uniqueCatalogListSchema<T extends CatalogRecord>(
  itemSchema: Schema<T>,
  options: { maximumLength?: number } = {},
): Schema<T[]> {
  const listSchema = arraySchema(itemSchema, {
    maximumLength: options.maximumLength ?? MAXIMUM_CATALOG_ITEMS,
  });
  return {
    parse(value: unknown, path = "$"): T[] {
      const records = listSchema.parse(value, path);
      assertUniqueCatalogRecords(records, path);
      return records;
    },
  };
}

export function uniqueCatalogPageSchema<T extends CatalogRecord>(
  itemSchema: Schema<T>,
): Schema<Paginated<T>> {
  const pageSchema = paginatedSchema(itemSchema);
  return {
    parse(value: unknown, path = "$"): Paginated<T> {
      const page = pageSchema.parse(value, path);
      assertUniqueCatalogRecords(page.records, `${path}.records`);
      if (
        page.records.length > page.pagination.page_size ||
        page.records.length > page.pagination.total_items
      ) {
        throw new TypeError(`${path}: inconsistent catalog pagination`);
      }
      return page;
    },
  };
}

const levelSchemaShape = {
  id: entityIDSchema,
  name: stringSchema({
    minimumLength: 1,
    maximumLength: 100,
    label: "level name",
  }),
  min_exp: integerSchema({
    minimum: 0,
    maximum: MAXIMUM_SAFE_API_INTEGER,
    label: "level minimum experience",
  }),
  description: optionalSchema(
    stringSchema({
      maximumLength: 500,
      label: "level description",
    }),
  ),
};

const rankSchemaShape = {
  id: entityIDSchema,
  name: stringSchema({
    minimumLength: 1,
    maximumLength: 100,
    label: "rank name",
  }),
  min_rating: integerSchema({
    minimum: 0,
    maximum: MAXIMUM_SAFE_API_INTEGER,
    label: "rank minimum rating",
  }),
  description: optionalSchema(
    stringSchema({
      maximumLength: 500,
      label: "rank description",
    }),
  ),
};

const raritySchemaShape = {
  id: entityIDSchema,
  name: stringSchema({
    minimumLength: 1,
    maximumLength: 50,
    label: "rarity name",
  }),
  code: stringSchema({
    minimumLength: 1,
    maximumLength: 20,
    label: "rarity code",
  }),
  weight: integerSchema({
    minimum: 1,
    maximum: MAXIMUM_SAFE_API_INTEGER,
    label: "rarity weight",
  }),
  description: optionalSchema(
    stringSchema({
      maximumLength: 255,
      label: "rarity description",
    }),
  ),
};

const elementSchemaShape = {
  id: entityIDSchema,
  name: stringSchema({
    minimumLength: 1,
    maximumLength: 50,
    label: "element name",
  }),
  code: stringSchema({
    minimumLength: 1,
    maximumLength: 20,
    label: "element code",
  }),
  description: optionalSchema(
    stringSchema({
      maximumLength: 255,
      label: "element description",
    }),
  ),
  version: integerSchema({
    minimum: 1,
    maximum: MAXIMUM_SAFE_API_INTEGER,
    label: "element version",
  }),
};

export const levelSchema = strictObjectSchema(
  levelSchemaShape,
) as Schema<LevelResponse>;
export const rankSchema = strictObjectSchema(
  rankSchemaShape,
) as Schema<RankResponse>;
export const raritySchema = strictObjectSchema(
  raritySchemaShape,
) as Schema<RarityResponse>;
export const elementSchema = strictObjectSchema(
  elementSchemaShape,
) as Schema<ElementResponse>;

export const levelPageSchema: Schema<Paginated<LevelResponse>> =
  uniqueCatalogPageSchema(levelSchema);
export const rankPageSchema: Schema<Paginated<RankResponse>> =
  uniqueCatalogPageSchema(rankSchema);
export const rarityPageSchema: Schema<Paginated<RarityResponse>> =
  uniqueCatalogPageSchema(raritySchema);
export const elementPageSchema: Schema<Paginated<ElementResponse>> =
  uniqueCatalogPageSchema(elementSchema);

export const levelListSchema = uniqueCatalogListSchema(levelSchema);
export const rankListSchema = uniqueCatalogListSchema(rankSchema);
export const rarityListSchema = uniqueCatalogListSchema(raritySchema);
export const elementListSchema = uniqueCatalogListSchema(elementSchema);

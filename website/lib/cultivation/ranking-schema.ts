import { entityIDSchema } from "@/lib/api/contracts";
import {
  arraySchema,
  enumSchema,
  integerSchema,
  optionalSchema,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import type {
  RankingEntryPayload,
  RankingPagePayload,
  RankingPrivacy,
} from "@/types/ranking";

const displayModeSchema = enumSchema(["public", "anonymous"] as const);
export const rankingVisibilitySchema = enumSchema([
  "public",
  "anonymous",
  "hidden",
] as const);

const rankingEntryDocumentSchema: Schema<RankingEntryPayload> = strictObjectSchema({
  rank: integerSchema({ minimum: 1, label: "ranking position" }),
  public_id: entityIDSchema,
  display_name: optionalSchema(
    stringSchema({ minimumLength: 1, maximumLength: 128 }),
  ),
  display_mode: displayModeSchema,
  rating: integerSchema({ minimum: 0, label: "rating" }),
  rank_title: optionalSchema(stringSchema({ maximumLength: 100 })),
  total_exp: integerSchema({ minimum: 0, label: "total experience" }),
  level_name: optionalSchema(stringSchema({ maximumLength: 100 })),
  level: optionalSchema(integerSchema({ minimum: 0, label: "cultivation level" })),
});

const rankingEntrySchema: Schema<RankingEntryPayload> = {
  parse(value: unknown, path = "$") {
    const entry = rankingEntryDocumentSchema.parse(value, path);
    const hasDisplayName = entry.display_name !== undefined;
    if ((entry.display_mode === "public") !== hasDisplayName) {
      throw new TypeError(`${path}: display name does not match privacy mode`);
    }
    return entry;
  },
};

const rankingPageDocumentSchema: Schema<RankingPagePayload> = strictObjectSchema({
  entries: arraySchema(rankingEntrySchema, { maximumLength: 100 }),
  next_cursor: optionalSchema(stringSchema({ minimumLength: 1, maximumLength: 1024 })),
  projection_revision: integerSchema({ minimum: 0, label: "projection revision" }),
});

export const rankingPageSchema: Schema<RankingPagePayload> = {
  parse(value: unknown, path = "$") {
    const page = rankingPageDocumentSchema.parse(value, path);
    const publicIDs = new Set<string>();
    let previousPosition = 0;
    for (const [index, entry] of page.entries.entries()) {
      if (publicIDs.has(entry.public_id)) {
        throw new TypeError(`${path}.entries[${index}]: duplicate public identity`);
      }
      if (entry.rank <= previousPosition) {
        throw new TypeError(`${path}.entries[${index}]: unstable ranking order`);
      }
      publicIDs.add(entry.public_id);
      previousPosition = entry.rank;
    }
    return page;
  },
};

export const rankingPrivacySchema: Schema<RankingPrivacy> = strictObjectSchema({
  visibility: rankingVisibilitySchema,
  version: integerSchema({ minimum: 1, label: "privacy version" }),
  projection_revision: integerSchema({ minimum: 1, label: "projection revision" }),
  replayed: optionalSchema({
    parse(value: unknown, path = "$") {
      if (typeof value !== "boolean") throw new TypeError(`${path}: expected boolean`);
      return value;
    },
  }),
});

export const rankingLimitSchema = integerSchema({
  minimum: 1,
  maximum: 100,
  label: "ranking limit",
});

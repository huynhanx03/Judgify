import assert from "node:assert/strict";
import { test } from "vitest";

import {
  elementSchema,
  levelListSchema,
  levelPageSchema,
  levelSchema,
  rankSchema,
  raritySchema,
} from "@/lib/cultivation/catalog-schema";

const ENTITY_ID = "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17";
const OTHER_ID = "019f6abb-8dd5-7581-8449-2b9ad77873e5";

const PAGINATION = {
  current_page: 1,
  page_size: 10,
  total_pages: 1,
  total_items: 1,
  has_next: false,
  has_prev: false,
};

test("cultivation catalog entities reject drift and unsafe numeric values", () => {
  assert.deepEqual(
    levelSchema.parse({
      id: ENTITY_ID,
      name: "Nhập môn",
      min_exp: 0,
    }),
    {
      id: ENTITY_ID,
      name: "Nhập môn",
      min_exp: 0,
      description: undefined,
    },
  );

  assert.throws(
    () =>
      levelSchema.parse({
        id: ENTITY_ID,
        name: "Nhập môn",
        min_exp: Number.MAX_SAFE_INTEGER + 1,
      }),
    /minimum experience/,
  );
  assert.throws(
    () =>
      rankSchema.parse({
        id: ENTITY_ID,
        name: "Đồng",
        min_rating: -1,
      }),
    /minimum rating/,
  );
  assert.throws(
    () =>
      raritySchema.parse({
        id: ENTITY_ID,
        name: "Hiếm",
        code: "rare",
        weight: 0,
      }),
    /rarity weight/,
  );
  assert.throws(
    () =>
      elementSchema.parse({
        id: ENTITY_ID,
        name: "Hỏa",
        code: "fire",
        version: 1,
        unexpected: true,
      }),
    /unknown field/,
  );
});

test("element versions and UUID identities are mandatory at the network boundary", () => {
  assert.equal(
    elementSchema.parse({
      id: ENTITY_ID,
      name: "Hỏa",
      code: "fire",
      version: 7,
    }).version,
    7,
  );
  assert.throws(
    () =>
      elementSchema.parse({
        id: ENTITY_ID,
        name: "Hỏa",
        code: "fire",
      }),
    /element version/,
  );
  assert.throws(
    () =>
      levelSchema.parse({ id: 42, name: "Nhập môn", min_exp: 0 }),
    /UUID/,
  );
});

test("catalog list and page responses are parsed recursively", () => {
  const item = {
    id: ENTITY_ID,
    name: "Nhập môn",
    min_exp: 0,
  };
  assert.equal(levelListSchema.parse([item])[0]?.id, ENTITY_ID);
  assert.equal(
    levelPageSchema.parse({
      records: [item],
      pagination: PAGINATION,
    }).pagination.total_items,
    1,
  );
  assert.throws(
    () =>
      levelPageSchema.parse({
        records: [{ ...item, id: OTHER_ID, name: "" }],
        pagination: PAGINATION,
      }),
    /level name/,
  );
  assert.throws(
    () => levelListSchema.parse([item, item]),
    /duplicate catalog identity/,
  );
  assert.throws(
    () =>
      levelPageSchema.parse({
        records: [item],
        pagination: {
          ...PAGINATION,
          total_items: 0,
        },
      }),
    /inconsistent catalog pagination/,
  );
});

import assert from "node:assert/strict";
import { test } from "vitest";

import {
  entityIDSchema,
  enumSchema,
  isoDateTimeSchema,
  paginationMetaSchema,
  parseApiSuccessEnvelope,
} from "@/lib/api/contracts";
import { parseApiErrorEnvelope } from "@/lib/api/error";
import { parseJsonDocument } from "@/lib/api/schema";

const CID = "019f6abb-8dd5-7581-8449-2b9ad77873e5";
const ENTITY_ID = "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17";

test("entity IDs remain canonical UUID strings and reject numeric or malformed values", () => {
  assert.equal(entityIDSchema.parse(ENTITY_ID), ENTITY_ID);
  assert.throws(() => entityIDSchema.parse(42), /UUID/);
  assert.throws(() => entityIDSchema.parse("42"), /UUID/);
  assert.throws(
    () => entityIDSchema.parse("018F5FBE-3D77-7E10-8FD1-8B6F1CA32E17"),
    /canonical UUID/,
  );
});

test("timestamps, enums, pagination, cursor, and CID are checked at the boundary", () => {
  assert.equal(
    isoDateTimeSchema.parse("2026-07-28T10:20:30.000Z"),
    "2026-07-28T10:20:30.000Z",
  );
  assert.throws(() => isoDateTimeSchema.parse("28/07/2026"), /timestamp/);

  const state = enumSchema(["queued", "running", "done"] as const);
  assert.equal(state.parse("running"), "running");
  assert.throws(() => state.parse("unknown"), /enum/);

  assert.deepEqual(
    paginationMetaSchema.parse({
      current_page: 2,
      page_size: 20,
      total_pages: 4,
      total_items: 65,
      has_next: true,
      has_prev: true,
    }),
    {
      current_page: 2,
      page_size: 20,
      total_pages: 4,
      total_items: 65,
      has_next: true,
      has_prev: true,
    },
  );
  assert.throws(
    () =>
      paginationMetaSchema.parse({
        current_page: 0,
        page_size: 20,
        total_pages: 4,
        total_items: 65,
        has_next: true,
        has_prev: false,
      }),
    /pagination/,
  );
});

test("JSON parsing rejects duplicate object keys instead of accepting last-write-wins drift", () => {
  assert.throws(
    () =>
      parseJsonDocument(
        `{"data":{"id":"${ENTITY_ID}","id":"${CID}"},"meta":{"cid":"${CID}"}}`,
      ),
    /duplicate JSON object key/,
  );
});

test("canonical and current go-common success envelopes normalize to one contract", () => {
  const schema = {
    parse(value: unknown) {
      const record = value as { id?: unknown };
      return { id: entityIDSchema.parse(record.id) };
    },
  };

  assert.deepEqual(
    parseApiSuccessEnvelope(
      {
        data: { id: ENTITY_ID },
        meta: { cid: CID },
      },
      schema,
    ),
    {
      data: { id: ENTITY_ID },
      meta: { cid: CID },
    },
  );

  assert.deepEqual(
    parseApiSuccessEnvelope(
      {
        code: 20000,
        message: "success",
        data: { id: ENTITY_ID },
      },
      schema,
      CID,
    ),
    {
      data: { id: ENTITY_ID },
      meta: { cid: CID },
    },
  );
  assert.throws(
    () =>
      parseApiSuccessEnvelope(
        { code: 20000, message: "success", data: { id: ENTITY_ID } },
        schema,
      ),
    /CID/,
  );
});

test("error envelopes require CID and reject unsafe or secret-bearing details", () => {
  const parsed = parseApiErrorEnvelope(
    {
      error: {
        code: "validation_failed",
        params: { limit: 128 },
        fields: { source_code: "too_large" },
      },
      meta: { cid: CID },
    },
    422,
  );

  assert.equal(parsed.cid, CID);
  assert.equal(parsed.code, "validation_failed");
  assert.deepEqual(parsed.fields, { source_code: "too_large" });

  assert.throws(
    () =>
      parseApiErrorEnvelope(
        {
          error: {
            code: "bad",
            params: { access_token: "secret" },
          },
          meta: { cid: CID },
        },
        400,
      ),
    /sensitive/,
  );
  assert.throws(
    () =>
      parseApiErrorEnvelope(
        { error: { code: "bad" }, meta: { cid: "" } },
        400,
      ),
    /CID/,
  );
});

test("legacy go-common scalar details remain machine-readable parameters", () => {
  const parsed = parseApiErrorEnvelope(
    {
      code: 40009,
      message: "catalog changed",
      data: {
        reason: "authoring_catalog_changed",
        current_revision: 12,
      },
      cid: CID,
    },
    409,
  );

  assert.deepEqual(parsed.params, {
    reason: "authoring_catalog_changed",
    current_revision: 12,
  });
});

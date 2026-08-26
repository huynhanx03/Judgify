import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("trait service parses every network response and preserves durable public offer semantics", async () => {
  const service = await source("services/cultivation.service.ts");
  assert.match(service, /from "@\/lib\/api\/client"/);
  assert.match(service, /schema: traitListSchema/);
  assert.match(service, /schema: traitPageSchema/);
  assert.equal(service.match(/schema: traitSchema/g)?.length, 2);
  assert.match(service, /schema: voidSchema/);
  assert.match(service, /schema: traitOfferSchema/);
  assert.match(service, /getAllTraits\(signal\?: AbortSignal\)/);
  assert.match(service, /findTraits\([\s\S]*signal\?: AbortSignal/);
  assert.match(service, /createTraitOffer\(signal\?: AbortSignal\)/);
  assert.match(service, /method: "POST"/);
  assert.match(service, /auth: "none"/);
  assert.match(service, /runIdempotentCommand/);
  assert.match(service, /entityIDSchema\.parse\(id\)/);
});

test("trait schema closes the reward language and reuses catalog identity invariants", async () => {
  const [schema, types] = await Promise.all([
    source("lib/cultivation/trait-schema.ts"),
    source("types/cultivation.ts"),
  ]);
  assert.match(schema, /multiplier_delta_bps: integerSchema/);
  assert.match(schema, /flat_bonus: integerSchema/);
  assert.match(schema, /all-scoped effect cannot target elements/);
  assert.match(schema, /element codes must be canonical/);
  assert.doesNotMatch(schema, /jsonValueSchema/);
  assert.doesNotMatch(types, /TraitMetadata/);
  assert.match(schema, /uniqueCatalogListSchema/);
  assert.match(schema, /uniqueCatalogPageSchema/);
  assert.match(schema, /expires_at: isoDateTimeSchema/);
  assert.match(schema, /offer_checksum: sha256ChecksumSchema/);
  assert.match(types, /type TraitEffectSpec = TraitMultiplierEffect \| TraitBonusEffect/);
  assert.match(types, /interface TraitResponse extends TraitPresentation/);
});

test("reward profile service parses reads, previews, and idempotent applies", async () => {
  const [service, schema] = await Promise.all([
    source("services/reward-profile.service.ts"),
    source("lib/cultivation/reward-profile-schema.ts"),
  ]);
  assert.equal(service.match(/schema: rewardProfileSchema/g)?.length, 3);
  assert.match(service, /schema: rewardProfilePreviewSchema/);
  assert.match(service, /commandPurpose\("reward-profile-assignment"/);
  assert.match(service, /runIdempotentCommand/);
  assert.match(service, /expected_revision/);
  assert.match(service, /preview_checksum/);
  assert.match(schema, /reward profile provenance is invalid/);
  assert.match(schema, /reward profile references a future effect/);
  assert.match(schema, /invalid reward profile slot topology/);
});

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("registration submits only the durable evidence returned by the active offer", async () => {
  const [flow, section, parser, rules, authTypes, cultivationTypes, cultivationService] = await Promise.all([
    source("modules/auth/RegisterFlow.tsx"),
    source("modules/auth/sections/trait-selection-section.tsx"),
    source("lib/auth/registration-trait-offer.ts"),
    source("constants/registration.ts"),
    source("types/auth.ts"),
    source("types/cultivation.ts"),
    source("services/cultivation.service.ts"),
  ]);

  assert.match(authTypes, /offer_id:\s*EntityID/);
  assert.match(authTypes, /catalog_checksum:\s*string/);
  assert.match(authTypes, /offer_checksum:\s*string/);
  assert.match(cultivationTypes, /interface TraitOfferResponse/);
  assert.match(cultivationTypes, /expires_at:\s*ISODateTime/);
  assert.match(cultivationTypes, /server_time:\s*ISODateTime/);
  assert.match(rules, /OFFERED_TALENT_COUNT:\s*6/);
  assert.match(rules, /SELECTED_TALENT_COUNT:\s*3/);
  assert.match(parser, /new Set\(response\.talents\.map\(\(trait\) => trait\.id\)\)/);
  assert.match(parser, /trait\.type !== "talent"/);
  assert.match(parser, /response\.root_bone\.type !== "root_bone"/);
  assert.match(parser, /serverExpiresAtEpochMs - serverTimeEpochMs/);
  assert.match(flow, /setTraitOffer\(offer\)/);
  assert.match(flow, /offer_id:\s*traitOffer\.id/);
  assert.match(flow, /offer_checksum:\s*traitOffer\.offerChecksum/);
  assert.match(flow, /REGISTRATION_TRAIT_RULES\.SELECTED_TALENT_COUNT/);
  assert.match(section, /REGISTRATION_TRAIT_RULES\.SELECTED_TALENT_COUNT/);
  assert.match(flow, /rolledTalents\.some\(\(talent\) => talent\.id === id\)/);
  assert.match(flow, /rollRequestSequence\.current/);
  assert.match(flow, /rollAbortControllerRef/);
  assert.doesNotMatch(flow, /hasAutoRolled|rollTraits\(\);\s*\}\s*,/);
  assert.doesNotMatch(`${flow}\n${parser}`, /(?:localStorage|sessionStorage|document\.cookie)/);
  assert.match(cultivationService, /api<TraitOfferResponse, never>\(ONBOARDING_TRAIT_API\.CREATE_OFFER/);
  assert.match(cultivationService, /method: "POST"/);
  assert.match(cultivationService, /auth: "none"/);
  assert.match(cultivationService, /idempotencyKey/);
  assert.match(cultivationService, /schema: traitOfferSchema/);
});

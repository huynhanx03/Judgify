import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("material lifecycle is an exact server preview then apply workflow", async () => {
  const [endpoints, service, actions, schema] = await Promise.all([
    source("constants/api/material.ts"),
    source("services/material.service.ts"),
    source("modules/admin/material-lifecycle-actions.tsx"),
    source("lib/materials/public-material-schema.ts"),
  ]);

  for (const action of ["PUBLISH", "ARCHIVE", "ROLLBACK"]) {
    assert.match(endpoints, new RegExp(`${action}_PREVIEW`));
    assert.match(service, new RegExp(`preview${titleCase(action)}`));
    assert.match(service, new RegExp(`apply${titleCase(action)}`));
  }
  assert.match(service, /idempotencyKey: commandID/);
  assert.match(service, /const confirmationToken = request\.confirmation_token/);
  assert.match(service, /confirmation_token: confirmationToken/);
  assert.match(schema, /materialLifecyclePreviewSchema/);
  assert.match(schema, /materialLifecycleReceiptSchema/);
  assert.match(schema, /render_artifact_id: entityIDSchema/);
  assert.match(schema, /source_checksum: stringSchema/);
  assert.match(schema, /published_revision_id: optionalSchema/);
  assert.match(schema, /published_revision_number: optionalSchema/);
  assert.match(actions, /if \(!preview\)[\s\S]*createLifecyclePreview/);
  assert.match(actions, /applyLifecyclePreview/);
  assert.match(actions, /setPreview\(null\)/);
  assert.match(actions, /setCommandId\(createIdempotencyKey\(\)\)/);
  assert.match(actions, /published_revision_id !== material\.revision_id/);
  assert.match(actions, /rollbackRevisionUnavailable/);
  assert.match(actions, /LifecycleProjectionCard/);
  assert.match(actions, /projection\.render_artifact_id/);
  assert.match(actions, /projection\.source_checksum/);
  assert.match(actions, /confirmation_expires_at/);
  assert.doesNotMatch(actions, />\s*(?:Xuất bản|Lưu trữ|Khôi phục)\s*</);
});

test("ordinary material writes carry durable UUID idempotency evidence", async () => {
  const [service, schema] = await Promise.all([
    source("services/material.service.ts"),
    source("lib/materials/public-material-schema.ts"),
  ]);

  assert.match(service, /optimisticMaterialMutation/);
  assert.match(service, /commandPurpose/);
  assert.match(service, /runIdempotentCommand/);
  assert.match(service, /command_id: commandID/);
  assert.match(service, /idempotencyKey: commandID/);
  assert.match(service, /materialMutationReceiptSchema/);
  assert.match(schema, /material\.draft_created\.v1/);
  assert.match(schema, /material\.draft_saved\.v1/);
  assert.match(schema, /material\.slug_changed\.v1/);
});

test("admin material lists stay bounded and edit loads the real current draft", async () => {
  const [endpoints, service, page] = await Promise.all([
    source("constants/api/material.ts"),
    source("services/material.service.ts"),
    source("modules/admin/materials/materials-page.tsx"),
  ]);

  assert.match(endpoints, /GET_ADMIN/);
  assert.match(service, /async getAdmin/);
  assert.match(page, /materialService\.getAdmin\(material\.id\)/);
  assert.match(page, /crud\.openEdit/);
});

function titleCase(value) {
  return value.toLowerCase().replace(/^./, (character) => character.toUpperCase());
}

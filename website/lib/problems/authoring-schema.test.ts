import assert from "node:assert/strict";
import { test } from "vitest";

import { problemAuthoringCatalogSchema } from "@/lib/problems/authoring-schema";

function validCatalog() {
  return {
    release_id: "judge-release.test",
    catalog_version:
      "v1.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    runtimes: [{
      key: "go",
      language: "go",
      display_name: "Go",
      profile_version: "1",
    }],
    checkers: [{
      key: "exact",
      version: "1",
      config_kind: "none",
    }],
    capabilities: [{
      runtime_key: "go",
      checker_key: "exact",
      checker_version: "1",
    }],
    policy: {
      slug_maximum_bytes: 128,
      title_maximum_bytes: 256,
      statement_maximum_bytes: 262_144,
      reason_maximum_bytes: 1_024,
      test_asset_maximum_bytes: 1_048_576,
      maximum_testcases: 128,
      maximum_groups: 32,
      maximum_tags: 16,
      maximum_runtimes: 8,
      cpu_time_ms: { default: 1_000, minimum: 1, maximum: 60_000 },
      wall_time_ms: { default: 2_000, minimum: 1, maximum: 120_000 },
      memory_limit_kb: {
        default: 262_144,
        minimum: 1,
        maximum: 1_048_576,
      },
      output_limit_bytes: {
        default: 1_048_576,
        minimum: 1,
        maximum: 16_777_216,
      },
      process_limit: { default: 32, minimum: 1, maximum: 256 },
    },
  };
}

test("authoring catalog accepts an explicit executable capability", () => {
  const catalog = problemAuthoringCatalogSchema.parse(validCatalog());
  assert.deepEqual(catalog.capabilities, [{
    runtime_key: "go",
    checker_key: "exact",
    checker_version: "1",
  }]);
});

test("authoring catalog requires a bounded opaque version token", () => {
  const missing = validCatalog() as Record<string, unknown>;
  delete missing.catalog_version;
  assert.throws(
    () => problemAuthoringCatalogSchema.parse(missing),
    /catalog_version/,
  );

  const malformed = validCatalog();
  malformed.catalog_version = "sha256:secret-implementation-checksum";
  assert.throws(
    () => problemAuthoringCatalogSchema.parse(malformed),
    /catalog version/,
  );
});

test("authoring catalog rejects capabilities outside active profiles", () => {
  const catalog = validCatalog();
  catalog.capabilities[0] = {
    ...catalog.capabilities[0],
    runtime_key: "cpp",
  };
  assert.throws(
    () => problemAuthoringCatalogSchema.parse(catalog),
    /unknown profile/,
  );
});

test("authoring catalog rejects active profiles with no safe tuple", () => {
  const catalog = validCatalog();
  catalog.runtimes.push({
    key: "cpp",
    language: "cpp",
    display_name: "C++",
    profile_version: "1",
  });
  assert.throws(
    () => problemAuthoringCatalogSchema.parse(catalog),
    /every active profile must be usable/,
  );
});

test("authoring catalog rejects a capability projection above its wire bound", () => {
  const catalog = validCatalog();
  catalog.runtimes = Array.from({ length: 65 }, (_, index) => ({
    key: `runtime-${index}`,
    language: "test",
    display_name: `Runtime ${index}`,
    profile_version: "1",
  }));
  catalog.capabilities = catalog.runtimes.map((runtime) => ({
    runtime_key: runtime.key,
    checker_key: "exact",
    checker_version: "1",
  }));

  assert.throws(
    () => problemAuthoringCatalogSchema.parse(catalog),
    /array exceeds boundary/,
  );
});

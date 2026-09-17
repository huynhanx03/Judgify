import { describe, expect, it } from "vitest";

import {
  authoringCatalogsMatch,
  checkerIdentity,
  compatibleCheckerIdentities,
  compatibleRuntimeKeys,
  isCompatibleAuthoringSelection,
  splitCheckerIdentity,
} from "@/lib/problems/authoring-capabilities";
import type { ProblemAuthoringCatalog } from "@/types/problem";

const catalog = {
  release_id: "judge-release.test",
  catalog_version:
    "v1.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  runtimes: [
    {
      key: "go",
      language: "go",
      display_name: "Go",
      profile_version: "1",
    },
    {
      key: "cpp",
      language: "cpp",
      display_name: "C++",
      profile_version: "1",
    },
  ],
  checkers: [
    { key: "exact", version: "1", config_kind: "none" },
    { key: "token", version: "1", config_kind: "none" },
  ],
  capabilities: [
    { runtime_key: "go", checker_key: "exact", checker_version: "1" },
    { runtime_key: "cpp", checker_key: "exact", checker_version: "1" },
    { runtime_key: "go", checker_key: "token", checker_version: "1" },
  ],
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
    memory_limit_kb: { default: 262_144, minimum: 1, maximum: 1_048_576 },
    output_limit_bytes: { default: 1_048_576, minimum: 1, maximum: 16_777_216 },
    process_limit: { default: 32, minimum: 1, maximum: 256 },
  },
} satisfies ProblemAuthoringCatalog;

describe("problem authoring capabilities", () => {
  it("round-trips checker identities without delimiter ambiguity", () => {
    const identity = checkerIdentity("exact", "1");
    expect(splitCheckerIdentity(identity)).toEqual(["exact", "1"]);
    expect(splitCheckerIdentity("invalid")).toEqual(["", ""]);
  });

  it("requires one checker to support every selected runtime", () => {
    expect(
      [...compatibleCheckerIdentities(catalog, ["go", "cpp"])],
    ).toEqual([checkerIdentity("exact", "1")]);
    expect(
      [...compatibleCheckerIdentities(catalog, ["go"])],
    ).toEqual([
      checkerIdentity("exact", "1"),
      checkerIdentity("token", "1"),
    ]);
  });

  it("returns compatible runtimes in catalog display order", () => {
    expect(
      compatibleRuntimeKeys(catalog, checkerIdentity("token", "1")),
    ).toEqual(["go"]);
  });

  it("accepts only a complete active runtime and checker capability selection", () => {
    expect(
      isCompatibleAuthoringSelection(
        catalog,
        ["go", "cpp"],
        checkerIdentity("exact", "1"),
      ),
    ).toBe(true);
    expect(
      isCompatibleAuthoringSelection(
        catalog,
        ["go", "cpp"],
        checkerIdentity("token", "1"),
      ),
    ).toBe(false);
    expect(
      isCompatibleAuthoringSelection(
        catalog,
        ["go", "unknown"],
        checkerIdentity("exact", "1"),
      ),
    ).toBe(false);
    expect(
      isCompatibleAuthoringSelection(
        catalog,
        ["go", "go"],
        checkerIdentity("exact", "1"),
      ),
    ).toBe(false);
  });

  it("detects release, policy, and exact capability catalog drift", () => {
    expect(authoringCatalogsMatch(catalog, structuredClone(catalog))).toBe(true);

    const changedRelease = structuredClone(catalog);
    changedRelease.release_id = "judge-release.next";
    expect(authoringCatalogsMatch(catalog, changedRelease)).toBe(false);

    const changedVersion = structuredClone(catalog);
    changedVersion.catalog_version =
      "v1.bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb";
    expect(authoringCatalogsMatch(catalog, changedVersion)).toBe(false);

    const changedCapability = structuredClone(catalog);
    changedCapability.capabilities = changedCapability.capabilities.slice(0, 2);
    expect(authoringCatalogsMatch(catalog, changedCapability)).toBe(false);

    const changedPolicy = structuredClone(catalog);
    changedPolicy.policy.maximum_runtimes = 1;
    expect(authoringCatalogsMatch(catalog, changedPolicy)).toBe(false);
  });
});

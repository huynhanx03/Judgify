import {
  arraySchema,
  assertOnlyKeys,
  booleanSchema,
  enumSchema,
  integerSchema,
  isRecord,
  optionalSchema,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import {
  entityIDSchema,
  isoDateTimeSchema,
} from "@/lib/api/contracts";
import type {
  ProblemAuthoringCatalog,
  ProblemAuthoringCapabilityOption,
  ProblemAuthoringCheckerOption,
  ProblemAuthoringDraft,
  ProblemAuthoringNumericPolicy,
  ProblemAuthoringPolicy,
  ProblemAuthoringRuntimeOption,
  ProblemAuthoringTestcase,
  ProblemAuthoringTestcaseGroup,
  ProblemDraftReceipt,
  ProblemPublicationReceipt,
} from "@/types/problem";

const protocolIdentifierSchema = stringSchema({
  minimumLength: 1,
  maximumLength: 128,
  pattern: /^[a-z0-9][a-z0-9._-]*$/,
  label: "protocol identifier",
});
const authoringCatalogVersionSchema = stringSchema({
  minimumLength: 67,
  maximumLength: 67,
  pattern: /^v1\.[0-9a-f]{64}$/,
  label: "authoring catalog version",
});
const MAXIMUM_AUTHORING_CAPABILITIES = 64;
const artifactStatusSchema = enumSchema(["ready", "rejected"] as const);
const artifactDiagnosticSchema = enumSchema([
  "invalid_utf8",
  "ambiguous_encoding",
  "source_too_large",
  "output_too_large",
  "nesting_too_deep",
  "render_rejected",
] as const);
const artifactDiagnosticsSchema = arraySchema(artifactDiagnosticSchema, {
  maximumLength: 8,
});

const toleranceSchema: Schema<number> = {
  parse(value: unknown, path = "$"): number {
    if (
      typeof value !== "number" ||
      !Number.isFinite(value) ||
      value < 0 ||
      value > 1
    ) {
      throw new TypeError(`${path}: invalid checker tolerance`);
    }
    return value;
  },
};

const checkerConfigSchema: Schema<Record<string, unknown>> = {
  parse(value: unknown, path = "$"): Record<string, unknown> {
    if (!isRecord(value)) {
      throw new TypeError(`${path}: expected checker config object`);
    }
    assertOnlyKeys(value, ["abs_tol", "rel_tol"], path);
    const keys = Object.keys(value);
    if (keys.length === 0) return {};
    if (
      keys.length !== 2 ||
      !Object.hasOwn(value, "abs_tol") ||
      !Object.hasOwn(value, "rel_tol")
    ) {
      throw new TypeError(
        `${path}: checker tolerances must be supplied together`,
      );
    }
    return {
      abs_tol: toleranceSchema.parse(value.abs_tol, `${path}.abs_tol`),
      rel_tol: toleranceSchema.parse(value.rel_tol, `${path}.rel_tol`),
    };
  },
};

const rawNumericPolicySchema = strictObjectSchema({
  default: integerSchema({ minimum: 1, label: "policy default" }),
  minimum: integerSchema({ minimum: 1, label: "policy minimum" }),
  maximum: integerSchema({ minimum: 1, label: "policy maximum" }),
});

const numericPolicySchema: Schema<ProblemAuthoringNumericPolicy> = {
  parse(value: unknown, path = "$"): ProblemAuthoringNumericPolicy {
    const policy = rawNumericPolicySchema.parse(value, path);
    if (
      policy.minimum > policy.default ||
      policy.default > policy.maximum
    ) {
      throw new TypeError(`${path}: inconsistent numeric policy`);
    }
    return policy;
  },
};

const authoringPolicySchema = strictObjectSchema({
  slug_maximum_bytes: integerSchema({
    minimum: 1,
    label: "slug byte boundary",
  }),
  title_maximum_bytes: integerSchema({
    minimum: 1,
    label: "title byte boundary",
  }),
  statement_maximum_bytes: integerSchema({
    minimum: 1,
    label: "statement byte boundary",
  }),
  reason_maximum_bytes: integerSchema({
    minimum: 1,
    label: "reason byte boundary",
  }),
  test_asset_maximum_bytes: integerSchema({
    minimum: 1,
    label: "test asset byte boundary",
  }),
  maximum_testcases: integerSchema({
    minimum: 1,
    label: "testcase boundary",
  }),
  maximum_groups: integerSchema({
    minimum: 1,
    label: "testcase group boundary",
  }),
  maximum_tags: integerSchema({
    minimum: 1,
    label: "tag boundary",
  }),
  maximum_runtimes: integerSchema({
    minimum: 1,
    label: "runtime boundary",
  }),
  cpu_time_ms: numericPolicySchema,
  wall_time_ms: numericPolicySchema,
  memory_limit_kb: numericPolicySchema,
  output_limit_bytes: numericPolicySchema,
  process_limit: numericPolicySchema,
}) as Schema<ProblemAuthoringPolicy>;

const runtimeOptionSchema = strictObjectSchema({
  key: protocolIdentifierSchema,
  language: stringSchema({ minimumLength: 1, maximumLength: 128 }),
  display_name: stringSchema({ minimumLength: 1, maximumLength: 256 }),
  profile_version: protocolIdentifierSchema,
}) as Schema<ProblemAuthoringRuntimeOption>;

const checkerOptionSchema = strictObjectSchema({
  key: protocolIdentifierSchema,
  version: protocolIdentifierSchema,
  config_kind: enumSchema(["none", "float_tolerance"] as const),
}) as Schema<ProblemAuthoringCheckerOption>;

const capabilityOptionSchema = strictObjectSchema({
  runtime_key: protocolIdentifierSchema,
  checker_key: protocolIdentifierSchema,
  checker_version: protocolIdentifierSchema,
}) as Schema<ProblemAuthoringCapabilityOption>;

const rawAuthoringCatalogSchema = strictObjectSchema({
  release_id: protocolIdentifierSchema,
  catalog_version: authoringCatalogVersionSchema,
  runtimes: arraySchema(runtimeOptionSchema, {
    maximumLength: MAXIMUM_AUTHORING_CAPABILITIES,
  }),
  checkers: arraySchema(checkerOptionSchema, {
    maximumLength: MAXIMUM_AUTHORING_CAPABILITIES,
  }),
  capabilities: arraySchema(capabilityOptionSchema, {
    maximumLength: MAXIMUM_AUTHORING_CAPABILITIES,
  }),
  policy: authoringPolicySchema,
});

export const problemAuthoringCatalogSchema: Schema<
  ProblemAuthoringCatalog
> = {
  parse(value: unknown, path = "$"): ProblemAuthoringCatalog {
    const catalog = rawAuthoringCatalogSchema.parse(value, path);
    if (
      catalog.runtimes.length === 0 ||
      catalog.checkers.length === 0 ||
      catalog.capabilities.length === 0
    ) {
      throw new TypeError(`${path}: authoring catalog must not be empty`);
    }
    if (
      new Set(catalog.runtimes.map((runtime) => runtime.key)).size !==
      catalog.runtimes.length
    ) {
      throw new TypeError(`${path}.runtimes: duplicate runtime key`);
    }
    if (
      new Set(
        catalog.checkers.map(
          (checker) => `${checker.key}\u0000${checker.version}`,
        ),
      ).size !== catalog.checkers.length
    ) {
      throw new TypeError(`${path}.checkers: duplicate checker identity`);
    }
    const runtimeKeys = new Set(
      catalog.runtimes.map((runtime) => runtime.key),
    );
    const checkerIDs = new Set(
      catalog.checkers.map(
        (checker) => `${checker.key}\u0000${checker.version}`,
      ),
    );
    const capabilityIDs = catalog.capabilities.map(
      (capability) =>
        `${capability.runtime_key}\u0000${capability.checker_key}\u0000${capability.checker_version}`,
    );
    if (new Set(capabilityIDs).size !== capabilityIDs.length) {
      throw new TypeError(`${path}.capabilities: duplicate capability`);
    }
    for (const capability of catalog.capabilities) {
      if (
        !runtimeKeys.has(capability.runtime_key) ||
        !checkerIDs.has(
          `${capability.checker_key}\u0000${capability.checker_version}`,
        )
      ) {
        throw new TypeError(
          `${path}.capabilities: capability references an unknown profile`,
        );
      }
    }
    const coveredRuntimeKeys = new Set(
      catalog.capabilities.map((capability) => capability.runtime_key),
    );
    const coveredCheckerIDs = new Set(
      catalog.capabilities.map((capability) =>
        `${capability.checker_key}\u0000${capability.checker_version}`
      ),
    );
    if (
      [...runtimeKeys].some((runtimeKey) => !coveredRuntimeKeys.has(runtimeKey)) ||
      [...checkerIDs].some((checkerID) => !coveredCheckerIDs.has(checkerID))
    ) {
      throw new TypeError(
        `${path}.capabilities: every active profile must be usable`,
      );
    }
    return catalog;
  },
};

const authoringTestcaseSchema = strictObjectSchema({
  test_case_id: entityIDSchema,
  kind: enumSchema(["sample", "hidden"] as const),
  input: stringSchema(),
  expected_output: stringSchema(),
}) as Schema<ProblemAuthoringTestcase>;

const authoringTestcaseGroupSchema = strictObjectSchema({
  stop_on_failure: booleanSchema,
  cases: arraySchema(authoringTestcaseSchema),
}) as Schema<ProblemAuthoringTestcaseGroup>;

const rawAuthoringDraftSchema = strictObjectSchema({
  problem_id: entityIDSchema,
  slug: stringSchema({ minimumLength: 1 }),
  lifecycle: enumSchema(["draft", "published", "archived"] as const),
  version: integerSchema({ minimum: 1, label: "problem version" }),
  revision_id: entityIDSchema,
  testset_revision_id: entityIDSchema,
  title: stringSchema({ minimumLength: 1 }),
  statement_markdown: stringSchema({ minimumLength: 1 }),
  difficulty_id: entityIDSchema,
  tag_ids: arraySchema(entityIDSchema, { unique: true }),
  cpu_time_ms: integerSchema({ minimum: 1, label: "CPU limit" }),
  wall_time_ms: integerSchema({ minimum: 1, label: "wall limit" }),
  memory_limit_kb: integerSchema({ minimum: 1, label: "memory limit" }),
  output_limit_bytes: integerSchema({ minimum: 1, label: "output limit" }),
  process_limit: integerSchema({ minimum: 1, label: "process limit" }),
  checker_key: protocolIdentifierSchema,
  checker_version: protocolIdentifierSchema,
  checker_config: checkerConfigSchema,
  allowed_runtime_keys: arraySchema(protocolIdentifierSchema, {
    unique: true,
  }),
  artifact_status: artifactStatusSchema,
  artifact_diagnostics: artifactDiagnosticsSchema,
  saved_at: isoDateTimeSchema,
  groups: arraySchema(authoringTestcaseGroupSchema),
});

function assertArtifactState(
  status: "ready" | "rejected",
  diagnostics: readonly string[],
  path: string,
): void {
  if (
    (status === "ready" && diagnostics.length !== 0) ||
    (status === "rejected" && diagnostics.length === 0)
  ) {
    throw new TypeError(`${path}: inconsistent render artifact state`);
  }
}

export const problemAuthoringDraftSchema: Schema<ProblemAuthoringDraft> = {
  parse(value: unknown, path = "$"): ProblemAuthoringDraft {
    const draft = rawAuthoringDraftSchema.parse(value, path);
    if (
      draft.groups.length === 0 ||
      draft.groups.some((group) => group.cases.length === 0)
    ) {
      throw new TypeError(`${path}.groups: testcase groups must be non-empty`);
    }
    if (draft.allowed_runtime_keys.length === 0) {
      throw new TypeError(
        `${path}.allowed_runtime_keys: runtime selection must be non-empty`,
      );
    }
    const testcases = draft.groups.flatMap((group) => group.cases);
    if (
      new Set(testcases.map((testcase) => testcase.test_case_id)).size !==
      testcases.length
    ) {
      throw new TypeError(`${path}.groups: duplicate testcase identity`);
    }
    assertArtifactState(
      draft.artifact_status,
      draft.artifact_diagnostics,
      path,
    );
    return draft;
  },
};

const rawProblemDraftReceiptSchema = strictObjectSchema({
  problem_id: entityIDSchema,
  revision_id: entityIDSchema,
  testset_revision_id: entityIDSchema,
  version: integerSchema({ minimum: 1, label: "problem version" }),
  artifact_status: artifactStatusSchema,
  artifact_diagnostics: artifactDiagnosticsSchema,
  saved_at: isoDateTimeSchema,
});

export const problemDraftReceiptSchema: Schema<ProblemDraftReceipt> = {
  parse(value: unknown, path = "$"): ProblemDraftReceipt {
    const receipt = rawProblemDraftReceiptSchema.parse(value, path);
    assertArtifactState(
      receipt.artifact_status,
      receipt.artifact_diagnostics,
      path,
    );
    return receipt;
  },
};

const rawProblemPublicationReceiptSchema = strictObjectSchema({
  problem_id: entityIDSchema,
  revision_id: optionalSchema(entityIDSchema),
  testset_revision_id: optionalSchema(entityIDSchema),
  version: integerSchema({ minimum: 1, label: "problem version" }),
  published_at: optionalSchema(isoDateTimeSchema),
  lifecycle: enumSchema(["published", "archived"] as const),
});

export const problemPublicationReceiptSchema: Schema<
  ProblemPublicationReceipt
> = {
  parse(value: unknown, path = "$"): ProblemPublicationReceipt {
    const receipt = rawProblemPublicationReceiptSchema.parse(value, path);
    const hasPublicationSelection =
      receipt.revision_id !== undefined &&
      receipt.testset_revision_id !== undefined &&
      receipt.published_at !== undefined;
    if (
      (receipt.lifecycle === "published" && !hasPublicationSelection) ||
      (receipt.lifecycle === "archived" &&
        (receipt.revision_id !== undefined ||
          receipt.testset_revision_id !== undefined ||
          receipt.published_at !== undefined))
    ) {
      throw new TypeError(`${path}: inconsistent publication receipt`);
    }
    return receipt;
  },
};

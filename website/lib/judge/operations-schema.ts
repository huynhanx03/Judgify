import {
  arraySchema,
  booleanSchema,
  enumSchema,
  integerSchema,
  optionalSchema,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import { entityIDSchema, isoDateTimeSchema } from "@/lib/api/contracts";
import type {
  JudgeActivationSnapshot,
  JudgeOperationsOverview,
  JudgeQueueSnapshot,
  JudgeWorkerSnapshot,
} from "@/types/judge-operations";

const MAXIMUM_SNAPSHOTS = 100;
const boundedIdentifier = stringSchema({
  minimumLength: 1,
  maximumLength: 512,
  label: "judge operations identifier",
});
const count = integerSchema({ minimum: 0, label: "judge operations count" });

const queueSchema: Schema<JudgeQueueSnapshot> = strictObjectSchema({
  ready: count,
  leased: count,
  retry_wait: count,
  done: count,
  cancelled: count,
  dead: count,
  oldest_ready_at: optionalSchema(isoDateTimeSchema),
});

const workerSchema: Schema<JudgeWorkerSnapshot> = strictObjectSchema({
  id: entityIDSchema,
  worker_key: boundedIdentifier,
  boot_id: entityIDSchema,
  boot_started_at: isoDateTimeSchema,
  heartbeat_at: isoDateTimeSchema,
  claims_enabled: booleanSchema,
  total_capacity: integerSchema({
    minimum: 1,
    label: "judge worker capacity",
  }),
  free_capacity: count,
  evidence_count: count,
  latest_probed_at: optionalSchema(isoDateTimeSchema),
});

const activationSchema: Schema<JudgeActivationSnapshot> = strictObjectSchema({
  id: entityIDSchema,
  release_id: boundedIdentifier,
  approved_manifest_checksum: boundedIdentifier,
  runtime_key: boundedIdentifier,
  runtime_profile_version: boundedIdentifier,
  checker_key: boundedIdentifier,
  checker_version: boundedIdentifier,
  checker_implementation_checksum: boundedIdentifier,
  capability_profile_hash: boundedIdentifier,
  state: enumSchema(["active", "retired"] as const),
  activated_at: isoDateTimeSchema,
  retired_at: optionalSchema(isoDateTimeSchema),
});

export const judgeOperationsOverviewSchema: Schema<JudgeOperationsOverview> =
  strictObjectSchema({
    generated_at: isoDateTimeSchema,
    worker_heartbeat_freshness_ms: integerSchema({
      minimum: 1,
      label: "worker heartbeat freshness",
    }),
    queue: queueSchema,
    workers: arraySchema(workerSchema, { maximumLength: MAXIMUM_SNAPSHOTS }),
    activations: arraySchema(activationSchema, {
      maximumLength: MAXIMUM_SNAPSHOTS,
    }),
  });

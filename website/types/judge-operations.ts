import type { EntityID, ISODateTime } from "@/types/api";

export interface JudgeQueueSnapshot {
  ready: number;
  leased: number;
  retry_wait: number;
  done: number;
  cancelled: number;
  dead: number;
  oldest_ready_at?: ISODateTime;
}

export interface JudgeWorkerSnapshot {
  id: EntityID;
  worker_key: string;
  boot_id: EntityID;
  boot_started_at: ISODateTime;
  heartbeat_at: ISODateTime;
  claims_enabled: boolean;
  total_capacity: number;
  free_capacity: number;
  evidence_count: number;
  latest_probed_at?: ISODateTime;
}

export type JudgeActivationState = "active" | "retired";

export interface JudgeActivationSnapshot {
  id: EntityID;
  release_id: string;
  approved_manifest_checksum: string;
  runtime_key: string;
  runtime_profile_version: string;
  checker_key: string;
  checker_version: string;
  checker_implementation_checksum: string;
  capability_profile_hash: string;
  state: JudgeActivationState;
  activated_at: ISODateTime;
  retired_at?: ISODateTime;
}

export interface JudgeOperationsOverview {
  generated_at: ISODateTime;
  worker_heartbeat_freshness_ms: number;
  queue: JudgeQueueSnapshot;
  workers: JudgeWorkerSnapshot[];
  activations: JudgeActivationSnapshot[];
}

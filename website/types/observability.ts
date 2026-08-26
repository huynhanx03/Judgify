import type { EntityID, ISODateTime } from "@/types/api";

export interface DebugWindow {
  active: boolean;
  key?: EntityID;
  component?: string;
  level?: "debug";
  started_at?: ISODateTime;
  expires_at?: ISODateTime;
}

export interface ActivateDebugWindowCommand {
  duration_seconds: number;
  reason: string;
}

export interface DeactivateDebugWindowCommand {
  reason: string;
}

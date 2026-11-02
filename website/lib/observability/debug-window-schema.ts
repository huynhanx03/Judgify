import {
  booleanSchema,
  ContractError,
  literalSchema,
  optionalSchema,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import {
  compareISODateTime,
  entityIDSchema,
  isoDateTimeSchema,
} from "@/lib/api/contracts";
import type { DebugWindow } from "@/types/observability";

const rawDebugWindowSchema = strictObjectSchema({
  active: booleanSchema,
  key: optionalSchema(entityIDSchema),
  component: optionalSchema(
    stringSchema({
      minimumLength: 1,
      maximumLength: 64,
      pattern: /^[a-z][a-z0-9._-]*$/,
      label: "debug-window component",
    }),
  ),
  level: optionalSchema(literalSchema("debug")),
  started_at: optionalSchema(isoDateTimeSchema),
  expires_at: optionalSchema(isoDateTimeSchema),
});

export const debugWindowSchema: Schema<DebugWindow> = {
  parse(value: unknown, path = "$"): DebugWindow {
    const window = rawDebugWindowSchema.parse(value, path);
    const leaseFields = [
      window.key,
      window.component,
      window.level,
      window.started_at,
      window.expires_at,
    ];
    const populated = leaseFields.filter(
      (field) => field !== undefined,
    ).length;
    if (window.active && populated !== leaseFields.length) {
      throw new ContractError(
        "active debug window is incomplete",
        path,
      );
    }
    if (populated !== 0 && populated !== leaseFields.length) {
      throw new ContractError(
        "debug-window lease projection is incomplete",
        path,
      );
    }
    if (
      window.started_at &&
      window.expires_at &&
      compareISODateTime(window.expires_at, window.started_at) <= 0
    ) {
      throw new ContractError(
        "debug-window expiry is not after its start",
        path,
      );
    }
    return window;
  },
};

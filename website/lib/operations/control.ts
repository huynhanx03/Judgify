import {
  OPERATION_CONTROL_ACTION,
  OPERATION_CONTROL_REASON_LIMITS,
} from "@/constants/operation";
import { commandPurpose } from "@/lib/api/command-purpose";
import { entityIDSchema } from "@/lib/api/contracts";
import type {
  OperationControlAction,
  OperationControlCommand,
} from "@/types/operation";

const textEncoder = new TextEncoder();

export function normalizeOperationControl(
  command: OperationControlCommand,
): OperationControlCommand {
  if (
    !Object.values(OPERATION_CONTROL_ACTION).includes(
      command.action as OperationControlAction,
    ) ||
    !Number.isSafeInteger(command.expected_version) ||
    command.expected_version < 1
  ) {
    throw new TypeError("invalid operation control command");
  }
  const reason = command.reason.trim();
  if (
    reason.length < OPERATION_CONTROL_REASON_LIMITS.MINIMUM_CHARACTERS ||
    textEncoder.encode(reason).byteLength >
      OPERATION_CONTROL_REASON_LIMITS.MAXIMUM_BYTES ||
    /[\p{C}]/u.test(reason)
  ) {
    throw new TypeError("invalid operation control reason");
  }
  return { ...command, reason };
}

export async function operationControlPurpose(
  operationID: string,
  command: OperationControlCommand,
): Promise<string> {
  return commandPurpose("operation-control", [
    entityIDSchema.parse(operationID),
    command.action,
    command.expected_version,
    command.reason,
  ]);
}


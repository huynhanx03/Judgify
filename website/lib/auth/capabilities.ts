import type {
  CapabilityRequirement,
  EffectiveCapabilitiesResponse,
} from "@/types/auth";

const CAPABILITY_SEPARATOR = "\u0000";
const SESSION_CAPABILITY_SEPARATOR = ":";

export interface CapabilityIndex {
  revision: number | null;
  keys: ReadonlySet<string>;
}

export const EMPTY_CAPABILITY_INDEX: CapabilityIndex = {
  revision: null,
  keys: new Set<string>(),
};

function capabilityKey(resource: string, action: string): string {
  return `${resource}${CAPABILITY_SEPARATOR}${action}`;
}

/**
 * Builds an immutable lookup snapshot. Wildcards are intentionally rejected:
 * the browser may only consume concrete capabilities projected by the API.
 */
export function createCapabilityIndex(
  response: EffectiveCapabilitiesResponse,
): CapabilityIndex {
  const keys = new Set<string>();

  for (const grant of response.capabilities) {
    const resource = grant.resource.trim();
    if (!resource || resource === "*") continue;

    for (const rawAction of grant.actions) {
      const action = rawAction.trim();
      if (!action || action === "*") continue;
      keys.add(capabilityKey(resource, action));
    }
  }

  return { revision: response.revision, keys };
}

/** Build the compatibility index from canonical `/session/me` keys. */
export function createCapabilityIndexFromKeys(
  capabilities: readonly string[],
  revision: number,
): CapabilityIndex {
  const keys = new Set<string>();
  for (const capability of capabilities) {
    const separator = capability.indexOf(SESSION_CAPABILITY_SEPARATOR);
    if (
      separator <= 0 ||
      separator !== capability.lastIndexOf(SESSION_CAPABILITY_SEPARATOR)
    ) {
      continue;
    }
    const resource = capability.slice(0, separator);
    const action = capability.slice(separator + 1);
    if (!resource || !action || resource === "*" || action === "*") continue;
    keys.add(capabilityKey(resource, action));
  }
  return { revision, keys };
}

export function hasCapability(
  index: CapabilityIndex,
  resource: string,
  action: string,
): boolean {
  if (!resource || !action || resource === "*" || action === "*") return false;
  return index.keys.has(capabilityKey(resource, action));
}

export function hasAnyRequirement(
  index: CapabilityIndex,
  requirements: readonly CapabilityRequirement[],
): boolean {
  return requirements.some(({ resource, action }) =>
    hasCapability(index, resource, action),
  );
}

export function hasAnyCapability(index: CapabilityIndex): boolean {
  return index.keys.size > 0;
}

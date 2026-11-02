import {
  AUTHORIZATION_ACTION,
  AUTHORIZATION_RESOURCE,
} from "@/constants/authorization";
import {
  hasAnyCapability,
  hasAnyRequirement,
  hasCapability,
  type CapabilityIndex,
} from "@/lib/auth/capabilities";
import type { CapabilityRequirement } from "@/types/auth";
import { APP_ROUTES } from "@/constants/routes";

export interface CapabilityBoundItem {
  requirements?: readonly CapabilityRequirement[];
  requiresAnyCapability?: boolean;
}

interface AdminRoutePolicy {
  pattern: RegExp;
  anyOf?: readonly CapabilityRequirement[];
  allOf?: readonly CapabilityRequirement[];
}

const requirement = (
  resource: string,
  action: string,
): CapabilityRequirement => ({ resource, action });

const read = (resource: string): readonly CapabilityRequirement[] => [
  requirement(resource, AUTHORIZATION_ACTION.READ),
];

/** Ordered from most specific to least specific. */
const ADMIN_ROUTE_POLICIES: readonly AdminRoutePolicy[] = [
  {
    pattern: /^\/admin\/problems\/create$/,
    allOf: [
      requirement(AUTHORIZATION_RESOURCE.PROBLEM, AUTHORIZATION_ACTION.CREATE),
    ],
  },
  {
    pattern: /^\/admin\/problems\/[^/]+\/edit$/,
    allOf: [
      requirement(AUTHORIZATION_RESOURCE.PROBLEM, AUTHORIZATION_ACTION.READ),
      requirement(AUTHORIZATION_RESOURCE.PROBLEM, AUTHORIZATION_ACTION.UPDATE),
    ],
  },
  {
    pattern: /^\/admin\/materials\/categories$/,
    allOf: read(AUTHORIZATION_RESOURCE.MATERIAL_CATEGORY),
  },
  {
    pattern: /^\/admin\/materials$/,
    allOf: read(AUTHORIZATION_RESOURCE.MATERIAL),
  },
  {
    pattern: /^\/admin\/problems$/,
    allOf: read(AUTHORIZATION_RESOURCE.PROBLEM),
  },
  {
    pattern: /^\/admin\/tags$/,
    allOf: read(AUTHORIZATION_RESOURCE.TAG),
  },
  {
    pattern: /^\/admin\/difficulties$/,
    allOf: read(AUTHORIZATION_RESOURCE.DIFFICULTY),
  },
  {
    pattern: /^\/admin\/contests\/[^/]+\/clarifications$/,
    allOf: read(AUTHORIZATION_RESOURCE.CONTEST),
  },
  {
    pattern: /^\/admin\/contests\/[^/]+$/,
    allOf: read(AUTHORIZATION_RESOURCE.CONTEST),
  },
  {
    pattern: /^\/admin\/contests$/,
    allOf: read(AUTHORIZATION_RESOURCE.CONTEST),
  },
  {
    pattern: /^\/admin\/audit$/,
    allOf: read(AUTHORIZATION_RESOURCE.AUDIT),
  },
  {
    pattern: /^\/admin\/attribute-definitions$/,
    allOf: read(AUTHORIZATION_RESOURCE.ATTRIBUTE_DEFINITION),
  },
  {
    pattern: /^\/admin\/submissions$/,
    allOf: read(AUTHORIZATION_RESOURCE.SUBMISSION),
  },
  {
    pattern: /^\/admin\/judge$/,
    allOf: read(AUTHORIZATION_RESOURCE.JUDGE),
  },
  {
    pattern: /^\/admin\/operations$/,
    allOf: read(AUTHORIZATION_RESOURCE.OPERATION),
  },
  {
    pattern: /^\/admin\/observability$/,
    allOf: read(AUTHORIZATION_RESOURCE.OBSERVABILITY),
  },
  {
    pattern: /^\/admin\/notification-campaigns$/,
    allOf: [
      requirement(
        AUTHORIZATION_RESOURCE.NOTIFICATION,
        AUTHORIZATION_ACTION.COMPOSE,
      ),
    ],
  },
  {
    pattern: /^\/admin\/elements$/,
    allOf: read(AUTHORIZATION_RESOURCE.ELEMENT),
  },
  {
    pattern: /^\/admin\/rarities$/,
    allOf: read(AUTHORIZATION_RESOURCE.RARITY),
  },
  {
    pattern: /^\/admin\/traits$/,
    allOf: read(AUTHORIZATION_RESOURCE.TRAIT),
  },
  {
    pattern: /^\/admin\/levels$/,
    allOf: read(AUTHORIZATION_RESOURCE.LEVEL),
  },
  {
    pattern: /^\/admin\/ranks$/,
    allOf: read(AUTHORIZATION_RESOURCE.RANK),
  },
  {
    pattern: /^\/admin\/user-traits$/,
    allOf: [
      requirement(
        AUTHORIZATION_RESOURCE.CULTIVATION_REWARD_RULE,
        AUTHORIZATION_ACTION.READ,
      ),
      requirement(AUTHORIZATION_RESOURCE.USER, AUTHORIZATION_ACTION.READ),
    ],
  },
  {
	pattern: /^\/admin\/progression$/,
	anyOf: [
	  requirement(AUTHORIZATION_RESOURCE.CULTIVATION_LEDGER, AUTHORIZATION_ACTION.READ),
	  requirement(AUTHORIZATION_RESOURCE.CULTIVATION_REWARD_RULE, AUTHORIZATION_ACTION.READ),
	],
	  },
  {
    pattern: /^\/admin\/users$/,
    allOf: read(AUTHORIZATION_RESOURCE.USER),
  },
  {
    pattern: /^\/admin\/roles$/,
    anyOf: [
      requirement(AUTHORIZATION_RESOURCE.ROLE, AUTHORIZATION_ACTION.READ),
      requirement(
        AUTHORIZATION_RESOURCE.AUTHORIZATION,
        AUTHORIZATION_ACTION.MANAGE,
      ),
    ],
  },
];

function normalizedPathname(pathname: string): string {
  const path = pathname.split(/[?#]/, 1)[0] || "/";
  return path.length > 1 ? path.replace(/\/+$/, "") : path;
}

export function adminRouteAllowed(
  pathname: string,
  index: CapabilityIndex,
): boolean {
  const normalized = normalizedPathname(pathname);
  if (normalized === APP_ROUTES.ADMIN) return hasAnyCapability(index);

  const policy = ADMIN_ROUTE_POLICIES.find(({ pattern }) =>
    pattern.test(normalized),
  );
  if (!policy) return false;
  const satisfiesAll =
    !policy.allOf?.length ||
    policy.allOf.every(({ resource, action }) =>
      hasCapability(index, resource, action),
    );
  const satisfiesAny =
    !policy.anyOf?.length || hasAnyRequirement(index, policy.anyOf);
  return Boolean(policy.allOf?.length || policy.anyOf?.length) &&
    satisfiesAll && satisfiesAny;
}

export function adminItemAllowed(
  item: CapabilityBoundItem,
  index: CapabilityIndex,
): boolean {
  if (item.requiresAnyCapability) return hasAnyCapability(index);
  if (!item.requirements?.length) return false;
  return hasAnyRequirement(index, item.requirements);
}

export function filterAuthorizedItems<T extends CapabilityBoundItem>(
  items: readonly T[],
  index: CapabilityIndex,
): T[] {
  return items.filter((item) => adminItemAllowed(item, index));
}

export function filterAuthorizedSections<
  S extends { items: readonly CapabilityBoundItem[] },
>(
  sections: readonly S[],
  index: CapabilityIndex,
): Array<S & { items: Array<S["items"][number]> }> {
  return sections.flatMap((section) => {
    const items = filterAuthorizedItems(section.items, index) as Array<
      S["items"][number]
    >;
    return items.length ? [{ ...section, items }] : [];
  });
}

export const VITAL_NAMES = ["LCP", "INP", "CLS", "TTFB", "client_error"] as const;
export type VitalName = (typeof VITAL_NAMES)[number];

export const ROUTE_KEYS = [
  "home", "admin", "arena", "auth", "contest", "material", "profile",
  "ranking", "other",
] as const;
export type RouteKey = (typeof ROUTE_KEYS)[number];

export function routeKey(pathname: string): RouteKey {
  if (pathname === "/") return "home";
  const first = pathname.split("/")[1];
  switch (first) {
    case "admin": return "admin";
    case "arena": return "arena";
    case "contest": return "contest";
    case "materials": return "material";
    case "profile":
    case "u": return "profile";
    case "ranking": return "ranking";
    case "login":
    case "register":
    case "forgot-password":
    case "reset-password":
    case "verify-recovery-contact":
    case "accept-invitation":
    case "auth": return "auth";
    default: return "other";
  }
}

export type VitalSignal = { route: RouteKey; name: VitalName; value: number };

export function parseVitalSignal(value: unknown): VitalSignal | null {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return null;
  const record = value as Record<string, unknown>;
  if (Object.keys(record).length !== 3 ||
      !Object.hasOwn(record, "route") ||
      !Object.hasOwn(record, "name") ||
      !Object.hasOwn(record, "value")) return null;
  if (!ROUTE_KEYS.includes(record.route as RouteKey) ||
      !VITAL_NAMES.includes(record.name as VitalName) ||
      typeof record.value !== "number" ||
      !Number.isFinite(record.value) ||
      record.value < 0 || record.value > 1_000_000) return null;
  return record as VitalSignal;
}

import matrix from "@/architecture/browser-matrix.json";

export type MatrixRoute = (typeof matrix.routes)[number];
type FixtureKey =
  | "publicProblemId"
  | "publicContestId"
  | "publicMaterialSlug"
  | "publicUsername"
  | "adminProblemId"
  | "adminContestId";

function optional(name: string): string | undefined {
  const value = process.env[name]?.trim();
  return value || undefined;
}

function enabled(name: string): boolean {
  return optional(name) === "1";
}

export const acceptanceEnvironment = Object.freeze({
  requireFull: enabled("JUDGIFY_E2E_REQUIRE_FULL"),
  enforcePerformance: enabled("JUDGIFY_E2E_ENFORCE_PERFORMANCE"),
  member: {
    username: optional("JUDGIFY_E2E_USER_USERNAME"),
    password: optional("JUDGIFY_E2E_USER_PASSWORD"),
  },
  admin: {
    username: optional("JUDGIFY_E2E_ADMIN_USERNAME"),
    password: optional("JUDGIFY_E2E_ADMIN_PASSWORD"),
  },
  fixtures: {
    publicProblemId: optional("JUDGIFY_E2E_PUBLIC_PROBLEM_ID"),
    publicContestId: optional("JUDGIFY_E2E_PUBLIC_CONTEST_ID"),
    publicMaterialSlug: optional("JUDGIFY_E2E_PUBLIC_MATERIAL_SLUG"),
    publicUsername: optional("JUDGIFY_E2E_PUBLIC_USERNAME"),
    adminProblemId: optional("JUDGIFY_E2E_ADMIN_PROBLEM_ID"),
    adminContestId: optional("JUDGIFY_E2E_ADMIN_CONTEST_ID"),
    policyRoleKey: optional("JUDGIFY_E2E_POLICY_ROLE_KEY"),
    policyResource: optional("JUDGIFY_E2E_POLICY_RESOURCE"),
    policyAction: optional("JUDGIFY_E2E_POLICY_ACTION"),
  },
});

export function hasMemberCredentials(): boolean {
  return Boolean(
    acceptanceEnvironment.member.username &&
      acceptanceEnvironment.member.password,
  );
}

export function hasAdminCredentials(): boolean {
  return Boolean(
    acceptanceEnvironment.admin.username &&
      acceptanceEnvironment.admin.password,
  );
}

export function resolveMatrixRoute(entry: MatrixRoute): string | null {
  const fixture = (entry as MatrixRoute & { fixture?: FixtureKey }).fixture;
  if (!fixture) return entry.route;
  const value = acceptanceEnvironment.fixtures[fixture];
  if (!value) return null;
  return entry.route.replace(/\[[^\]]+\]/, encodeURIComponent(value));
}

export function requireAcceptanceEnvironment(): void {
  if (!acceptanceEnvironment.requireFull) return;

  const required: Array<[string, unknown]> = [
    ["JUDGIFY_E2E_USER_USERNAME", acceptanceEnvironment.member.username],
    ["JUDGIFY_E2E_USER_PASSWORD", acceptanceEnvironment.member.password],
    ["JUDGIFY_E2E_ADMIN_USERNAME", acceptanceEnvironment.admin.username],
    ["JUDGIFY_E2E_ADMIN_PASSWORD", acceptanceEnvironment.admin.password],
    ["JUDGIFY_E2E_PUBLIC_PROBLEM_ID", acceptanceEnvironment.fixtures.publicProblemId],
    ["JUDGIFY_E2E_PUBLIC_CONTEST_ID", acceptanceEnvironment.fixtures.publicContestId],
    ["JUDGIFY_E2E_PUBLIC_MATERIAL_SLUG", acceptanceEnvironment.fixtures.publicMaterialSlug],
    ["JUDGIFY_E2E_PUBLIC_USERNAME", acceptanceEnvironment.fixtures.publicUsername],
    ["JUDGIFY_E2E_ADMIN_PROBLEM_ID", acceptanceEnvironment.fixtures.adminProblemId],
    ["JUDGIFY_E2E_ADMIN_CONTEST_ID", acceptanceEnvironment.fixtures.adminContestId],
    ["JUDGIFY_E2E_POLICY_ROLE_KEY", acceptanceEnvironment.fixtures.policyRoleKey],
    ["JUDGIFY_E2E_POLICY_RESOURCE", acceptanceEnvironment.fixtures.policyResource],
    ["JUDGIFY_E2E_POLICY_ACTION", acceptanceEnvironment.fixtures.policyAction],
  ];
  const missing = required.filter(([, value]) => !value).map(([name]) => name);
  if (missing.length > 0) {
    throw new Error(
      `Full browser acceptance requires environment variables: ${missing.join(", ")}`,
    );
  }
}

requireAcceptanceEnvironment();

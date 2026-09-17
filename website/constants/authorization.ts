export const POLICY_REASON_LIMITS = {
  MIN_LENGTH: 3,
  MAX_LENGTH: 500,
} as const;

export const AUTH_BOOTSTRAP_STATUS = {
  LOADING: "loading",
  READY: "ready",
  ERROR: "error",
} as const;

export const CAPABILITY_STATUS = {
  IDLE: "idle",
  LOADING: "loading",
  READY: "ready",
  ERROR: "error",
} as const;

/** Code-owned capability keys shared by route, navigation, and action policy. */
export const AUTHORIZATION_RESOURCE = {
  ATTRIBUTE_DEFINITION: "attribute_definition",
  AUDIT: "audit",
  AUTHORIZATION: "authorization",
  CONTEST: "contest",
  CULTIVATION_LEDGER: "cultivation_ledger",
  CULTIVATION_REWARD_RULE: "cultivation_reward_rule",
  DIFFICULTY: "difficulty",
  ELEMENT: "element",
  LEVEL: "level",
  JUDGE: "judge",
  MATERIAL: "material",
  MATERIAL_CATEGORY: "material_category",
  NOTIFICATION: "notification",
  OBSERVABILITY: "observability",
  OPERATION: "operation",
  PROBLEM: "problem",
  SUBMISSION: "submission",
  RANK: "rank",
  RARITY: "rarity",
  ROLE: "role",
  TAG: "tag",
  TEST_CASE: "test_case",
  TRAIT: "trait",
  USER: "user",
  USER_TRAIT: "user_trait",
} as const;

export const AUTHORIZATION_ACTION = {
  CREATE: "create",
  INSPECT: "inspect",
  READ: "read",
  UPDATE: "update",
  DELETE: "delete",
  MANAGE: "manage",
  PUBLISH: "publish",
  ARCHIVE: "archive",
  ROLLBACK: "rollback",
  EXECUTE: "execute",
  ADJUST: "adjust",
  REVERSE: "reverse",
  REBUILD: "rebuild",
  BROADCAST_ALL_ACTIVE: "broadcast_all_active",
  BROADCAST_ROLE: "broadcast_role",
  COMPOSE: "compose",
  READ_DELIVERY: "read_delivery",
} as const;

import {
  arraySchema,
  booleanSchema,
  integerSchema,
  nullableSchema,
  optionalSchema,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import {
  cursorSchema,
  entityIDSchema,
  isoDateTimeSchema,
} from "@/lib/api/contracts";
import type {
  AuthSession,
  AuthSessionListResponse,
  AuthSessionMutationResponse,
  SessionView,
} from "@/types/session";

const capabilitySchema = stringSchema({
  minimumLength: 3,
  maximumLength: 129,
  pattern: /^[a-z][a-z0-9_.-]{0,63}:[a-z][a-z0-9_.-]{0,63}$/,
  label: "concrete capability key",
});

const rawSessionViewSchema = strictObjectSchema({
  user: strictObjectSchema({
    id: entityIDSchema,
    username: stringSchema({ minimumLength: 1, maximumLength: 50 }),
    first_name: optionalSchema(
      stringSchema({ minimumLength: 1, maximumLength: 100 }),
    ),
    last_name: optionalSchema(
      stringSchema({ minimumLength: 1, maximumLength: 100 }),
    ),
  }),
  session: strictObjectSchema({
    id: entityIDSchema,
    expires_at: isoDateTimeSchema,
    reauthenticated_until: optionalSchema(
      nullableSchema(isoDateTimeSchema),
    ),
  }),
  capabilities: arraySchema(capabilitySchema, {
    maximumLength: 512,
    unique: true,
  }),
  authorization_revision: integerSchema({
    minimum: 0,
    label: "authorization revision",
  }),
});

export const sessionViewSchema: Schema<SessionView> = {
  parse(value: unknown, path = "$"): SessionView {
    const parsed = rawSessionViewSchema.parse(value, path);
    return {
      user: {
        id: parsed.user.id,
        username: parsed.user.username,
        ...(parsed.user.first_name
          ? { first_name: parsed.user.first_name }
          : {}),
        ...(parsed.user.last_name ? { last_name: parsed.user.last_name } : {}),
      },
      session: {
        id: parsed.session.id,
        expires_at: parsed.session.expires_at,
        ...(parsed.session.reauthenticated_until === undefined
          ? {}
          : {
              reauthenticated_until:
                parsed.session.reauthenticated_until,
            }),
      },
      capabilities: parsed.capabilities,
      authorization_revision: parsed.authorization_revision,
    };
  },
};

const rawAuthSessionSchema = strictObjectSchema({
  id: entityIDSchema,
  current: booleanSchema,
  status: stringSchema({
    pattern: /^(?:active|revoked|expired)$/,
    label: "session status",
  }),
  created_at: isoDateTimeSchema,
  last_seen_at: isoDateTimeSchema,
  last_reauthenticated_at: isoDateTimeSchema,
  idle_expires_at: isoDateTimeSchema,
  absolute_expires_at: isoDateTimeSchema,
  revoked_at: optionalSchema(nullableSchema(isoDateTimeSchema)),
  user_agent: optionalSchema(
    stringSchema({ minimumLength: 1, maximumLength: 512 }),
  ),
});

const authSessionSchema: Schema<AuthSession> = {
  parse(value: unknown, path = "$"): AuthSession {
    const parsed = rawAuthSessionSchema.parse(value, path);
    return parsed as AuthSession;
  },
};

const rawSessionListSchema = strictObjectSchema({
  items: arraySchema(authSessionSchema, { maximumLength: 50 }),
  next_cursor: optionalSchema(nullableSchema(cursorSchema)),
});

export const authSessionListSchema: Schema<AuthSessionListResponse> = {
  parse(value: unknown, path = "$"): AuthSessionListResponse {
    const parsed = rawSessionListSchema.parse(value, path);
    return {
      items: parsed.items,
      ...(parsed.next_cursor === undefined
        ? {}
        : { next_cursor: parsed.next_cursor }),
    };
  },
};

export const sessionMutationSchema: Schema<AuthSessionMutationResponse> = {
  parse(value: unknown, path = "$"): AuthSessionMutationResponse {
    const parsed = strictObjectSchema({ success: booleanSchema }).parse(
      value,
      path,
    );
    if (!parsed.success) {
      throw new TypeError(`${path}.success: command did not succeed`);
    }
    return parsed;
  },
};

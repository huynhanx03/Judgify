package db_test

import (
	"slices"
	"strings"
	"testing"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschemaapi "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"github.com/stretchr/testify/require"

	entschema "github.com/huynhanx03/judgify/internal/ent/schema"
)

type fieldSchema interface{ Fields() []ent.Field }
type indexSchema interface{ Indexes() []ent.Index }
type annotationSchema interface {
	Annotations() []entschemaapi.Annotation
}

func TestSchemaInvariants(t *testing.T) {
	t.Run("closed security states", testClosedSecurityStates)
	t.Run("opaque secret storage", testOpaqueSecretStorage)
	t.Run("refresh rotation identity", testRefreshRotationIdentity)
	t.Run("recoverable issuance identity", testIssuanceIdentity)
	t.Run("bounded recovery contacts", testRecoveryContactCardinality)
	t.Run("oauth provider identity", testOAuthIdentity)
	t.Run("authorization singleton and catalog", testAuthorizationState)
	t.Run("profile revisions", testProfileRevisions)
	t.Run("operational polling indexes", testOperationalIndexes)
}

func testClosedSecurityStates(t *testing.T) {
	t.Helper()
	requireEnum(t, entschema.User{}, "status", "invited", "active", "suspended", "deleted")
	requireEnum(t, entschema.AuthSession{}, "status", "active", "revoked", "expired")
	requireEnum(t, entschema.AuthRefreshToken{}, "status", "issued", "consumed", "revoked")
	requireEnum(t, entschema.UserRecoveryContact{}, "status", "pending", "verified", "revoked")
	requireEnum(t, entschema.UserRecoveryContact{}, "purpose", "registration", "replacement", "invitation")
	requireEnum(t, entschema.RecoveryContactVerificationToken{}, "status", "issued", "consumed", "superseded", "expired")
	requireEnum(t, entschema.PasswordResetToken{}, "status", "issued", "consumed", "superseded", "expired")
	requireEnum(t, entschema.SecurityEmailDelivery{}, "status", "ready", "leased", "sent", "dead", "cancelled")
	requireEnum(t, entschema.OAuthFlow{}, "status", "pending", "consumed", "expired")
	requireEnum(t, entschema.OAuthCompletionTicket{}, "status", "issued", "consumed", "expired")
	requireEnum(t, entschema.RecoveryContactCommandReceipt{}, "kind", "replace", "resend", "cancel_pending", "remove")
	requireEnum(t, entschema.AuthSessionCommandReceipt{}, "kind", "revoke", "logout_all")

	requireNamedCheck(t, entschema.User{}, "user_status_closed")
	requireNamedCheck(t, entschema.AuthSession{}, "auth_session_status_closed")
	requireNamedCheck(t, entschema.AuthRefreshToken{}, "auth_refresh_status_closed")
	requireNamedCheck(t, entschema.UserRecoveryContact{}, "recovery_contact_status_closed")
	requireNamedCheck(t, entschema.RecoveryContactVerificationToken{}, "recovery_verification_status_closed")
	requireNamedCheck(t, entschema.PasswordResetToken{}, "password_reset_status_closed")
	requireNamedCheck(t, entschema.SecurityEmailDelivery{}, "security_email_status_closed")
	requireNamedCheck(t, entschema.SecurityEmailDelivery{}, "security_email_state_shape")
	requireNamedCheck(t, entschema.OAuthFlow{}, "oauth_flow_status_closed")
	requireNamedCheck(t, entschema.OAuthCompletionTicket{}, "oauth_ticket_status_closed")
	requireNamedCheck(t, entschema.Role{}, "role_status_closed")
	requireNamedCheck(t, entschema.RecoveryContactCommandReceipt{}, "recovery_contact_command_kind_closed")
	requireNamedCheck(t, entschema.AuthSessionCommandReceipt{}, "auth_session_command_shape")
}

func testOpaqueSecretStorage(t *testing.T) {
	t.Helper()
	schemas := []fieldSchema{
		entschema.AuthRefreshToken{},
		entschema.AuthRefreshReceipt{},
		entschema.AuthSessionIssuanceReceipt{},
		entschema.UserRecoveryContact{},
		entschema.RecoveryContactVerificationToken{},
		entschema.PasswordResetToken{},
		entschema.SecurityEmailDelivery{},
		entschema.OAuthFlow{},
		entschema.OAuthCompletionTicket{},
	}
	for _, schema := range schemas {
		for _, descriptor := range fields(schema) {
			name := descriptor.Name
			for _, forbidden := range []string{
				"raw_token", "raw_secret", "refresh_token", "reset_token", "verification_token",
				"email_address", "recipient_address", "ip_address", "pkce_verifier",
			} {
				require.NotEqualf(t, forbidden, name, "%T persists forbidden plaintext field %q", schema, name)
			}
			if strings.HasSuffix(name, "_hash") || strings.HasSuffix(name, "_ciphertext") {
				require.Truef(t, descriptor.Sensitive, "%T.%s must be redacted by Ent", schema, name)
			}
		}
	}

	requireUniqueField(t, entschema.AuthRefreshToken{}, "token_hash")
	requireField(t, entschema.RecoveryContactVerificationToken{}, "hash_key_id")
	requireField(t, entschema.PasswordResetToken{}, "hash_key_id")
	requireUniqueIndex(t, entschema.RecoveryContactVerificationToken{}, "hash_key_id", "token_hash")
	requireUniqueIndex(t, entschema.PasswordResetToken{}, "hash_key_id", "token_hash")
	requireUniqueField(t, entschema.OAuthFlow{}, "state_hash")
	requireUniqueField(t, entschema.OAuthCompletionTicket{}, "ticket_hash")
}

func testRefreshRotationIdentity(t *testing.T) {
	t.Helper()
	requireUniqueIndex(t, entschema.AuthRefreshReceipt{}, "session_id", "presented_token_id", "idempotency_key")
	requireIndex(t, entschema.AuthSession{}, "user_id", "status")
	requireIndex(t, entschema.AuthSession{}, "family_id")
	requireIndex(t, entschema.AuthSession{}, "status", "idle_expires_at")
	requireIndex(t, entschema.AuthSession{}, "status", "absolute_expires_at")
	requireIndex(t, entschema.AuthSession{}, "user_id", "created_at", "id")
	requireUniqueIndex(t, entschema.AuthSessionCommandReceipt{}, "user_id", "idempotency_key")
	requireIndex(t, entschema.AuthSessionCommandReceipt{}, "expires_at")
}

func testIssuanceIdentity(t *testing.T) {
	t.Helper()
	requirePartialUniqueIndex(
		t,
		entschema.AuthSessionIssuanceReceipt{},
		"purpose = 'password_login'",
		"user_id", "pre_session_id", "idempotency_key",
	)
	requirePartialUniqueIndex(
		t,
		entschema.AuthSessionIssuanceReceipt{},
		"purpose <> 'password_login'",
		"purpose", "request_or_ticket_id", "idempotency_key",
	)
}

func testRecoveryContactCardinality(t *testing.T) {
	t.Helper()
	requirePartialUniqueIndex(t, entschema.UserRecoveryContact{}, "status = 'verified'", "user_id", "kind")
	requirePartialUniqueIndex(t, entschema.UserRecoveryContact{}, "status = 'pending'", "user_id", "kind")
	requireIndex(t, entschema.UserRecoveryContact{}, "fingerprint_hash")
	requireIndex(t, entschema.UserRecoveryContact{}, "lookup_hash")
	requireIndex(t, entschema.RecoveryContactVerificationToken{}, "contact_id", "contact_version", "status")
	requireUniqueIndex(t, entschema.RecoveryContactCommandReceipt{}, "user_id", "idempotency_key")
	requireIndex(t, entschema.RecoveryContactCommandReceipt{}, "expires_at")
}

func testOAuthIdentity(t *testing.T) {
	t.Helper()
	requireUniqueIndex(t, entschema.FederatedIdentity{}, "provider", "provider_subject")
	requireUniqueField(t, entschema.OAuthCompletionTicket{}, "flow_id")
}

func testAuthorizationState(t *testing.T) {
	t.Helper()
	stateFields := fields(entschema.AuthorizationState{})
	require.Contains(t, stateFields, "id")
	require.Equal(t, "default", stateFields["id"].Default)
	requireUniqueField(t, entschema.AuthorizationResource{}, "key")
	requireUniqueIndex(t, entschema.AuthorizationAction{}, "resource_id", "key")
}

func testProfileRevisions(t *testing.T) {
	t.Helper()
	requireUniqueIndex(t, entschema.AttributeDefinitionRevision{}, "definition_id", "revision")
	requireEnum(t, entschema.AttributeDefinitionRevision{}, "data_type", "string", "number", "boolean", "date")
	requireEnum(t, entschema.AttributeDefinitionRevision{}, "visibility", "public", "private", "admin")
	definitionFields := fields(entschema.AttributeDefinition{})
	require.True(t, definitionFields["key"].Immutable)
	require.True(t, definitionFields["kind"].Immutable)
	require.Contains(t, definitionFields, "required_on_onboarding")
	require.Contains(t, definitionFields, "active_revision_id")
	require.Contains(t, definitionFields, "draft_revision_id")
}

func testOperationalIndexes(t *testing.T) {
	t.Helper()
	requireIndex(t, entschema.SecurityEmailDelivery{}, "status", "next_attempt_at", "created_at", "id")
	requireIndex(t, entschema.SecurityEmailDelivery{}, "status", "leased_until")
	requireIndex(t, entschema.OAuthFlow{}, "status", "expires_at")
	requireIndex(t, entschema.OAuthCompletionTicket{}, "status", "expires_at")
	requireIndex(t, entschema.AuthRefreshReceipt{}, "expires_at")
}

func fields(schema fieldSchema) map[string]*field.Descriptor {
	result := make(map[string]*field.Descriptor)
	for _, schemaField := range schema.Fields() {
		descriptor := schemaField.Descriptor()
		result[descriptor.Name] = descriptor
	}
	return result
}

func requireEnum(t *testing.T, schema fieldSchema, name string, expected ...string) {
	t.Helper()
	descriptor, ok := fields(schema)[name]
	require.Truef(t, ok, "%T is missing field %q", schema, name)
	actual := make([]string, 0, len(descriptor.Enums))
	for _, enum := range descriptor.Enums {
		actual = append(actual, enum.V)
	}
	require.Equal(t, expected, actual, "%T.%s enum drifted", schema, name)
}

func requireUniqueIndex(t *testing.T, schema indexSchema, expected ...string) {
	t.Helper()
	requireIndexMatching(t, schema, true, "", expected...)
}

func requireUniqueField(t *testing.T, schema fieldSchema, name string) {
	t.Helper()
	descriptor, ok := fields(schema)[name]
	require.Truef(t, ok, "%T is missing field %q", schema, name)
	require.Truef(t, descriptor.Unique, "%T.%s must be unique", schema, name)
}

func requireField(t *testing.T, schema fieldSchema, name string) {
	t.Helper()
	_, ok := fields(schema)[name]
	require.Truef(t, ok, "%T is missing field %q", schema, name)
}

func requireIndex(t *testing.T, schema indexSchema, expected ...string) {
	t.Helper()
	requireIndexMatching(t, schema, false, "", expected...)
}

func requirePartialUniqueIndex(t *testing.T, schema indexSchema, predicate string, expected ...string) {
	t.Helper()
	requireIndexMatching(t, schema, true, predicate, expected...)
}

func requireIndexMatching(t *testing.T, schema indexSchema, unique bool, predicate string, expected ...string) {
	t.Helper()
	for _, schemaIndex := range schema.Indexes() {
		descriptor := schemaIndex.Descriptor()
		if !slices.Equal(descriptor.Fields, expected) || unique && !descriptor.Unique {
			continue
		}
		if predicate == "" {
			return
		}
		for _, annotation := range descriptor.Annotations {
			if sqlAnnotation, ok := annotation.(*entsql.IndexAnnotation); ok && sqlAnnotation.Where == predicate {
				return
			}
		}
	}
	require.Failf(t, "missing index", "%T needs unique=%v predicate=%q fields=%v", schema, unique, predicate, expected)
}

func requireNamedCheck(t *testing.T, schema annotationSchema, name string) {
	t.Helper()
	for _, annotation := range schema.Annotations() {
		sqlAnnotation, ok := annotation.(*entsql.Annotation)
		if !ok {
			continue
		}
		if expression := strings.TrimSpace(sqlAnnotation.Checks[name]); expression != "" {
			return
		}
	}
	require.Failf(t, "missing database check", "%T needs named CHECK constraint %q", schema, name)
}

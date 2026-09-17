package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
	rendercontract "github.com/huynhanx03/judgify/internal/platform/contentrender/contract"
)

// NotificationCampaignConfirmation stores only a nonce hash and the exact
// reviewed schedule tuple. The opaque nonce exists only in client memory.
type NotificationCampaignConfirmation struct{ ent.Schema }

func (NotificationCampaignConfirmation) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}}
}

func (NotificationCampaignConfirmation) Fields() []ent.Field {
	return []ent.Field{
		field.Bytes("nonce_hash").NotEmpty().Immutable().Sensitive(),
		field.UUID("actor_id", uuid.UUID{}).Immutable(),
		field.UUID("session_id", uuid.UUID{}).Immutable(),
		field.UUID("campaign_id", uuid.UUID{}).Immutable(),
		field.UUID("campaign_revision_id", uuid.UUID{}).Immutable(),
		field.UUID("render_source_id", uuid.UUID{}).Immutable(),
		field.Enum("render_source_kind").
			Values(rendercontract.SourceKindNotificationCampaignBody).
			Immutable(),
		field.Bytes("normalized_source_checksum").NotEmpty().Immutable().Sensitive(),
		field.UUID("render_artifact_id", uuid.UUID{}).Immutable(),
		field.Bytes("audience_fingerprint").NotEmpty().Immutable().Sensitive(),
		field.Int64("expected_campaign_version").Positive().Immutable(),
		field.Time("scheduled_for").Optional().Nillable().Immutable(),
		field.Time("dry_run_estimated_at").Immutable(),
		field.Int64("dry_run_recipient_count").NonNegative().Immutable(),
		field.Time("issued_at").Immutable(),
		field.Time("expires_at").Immutable(),
		field.Time("consumed_at").Optional().Nillable(),
		field.UUID("consumed_by_idempotency_key", uuid.UUID{}).Optional().Nillable(),
		field.Bytes("schedule_request_hash").Optional().Nillable().Sensitive(),
		field.UUID("operation_id", uuid.UUID{}).Optional().Nillable(),
	}
}

func (NotificationCampaignConfirmation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("campaign", NotificationCampaign.Type).
			Ref("confirmations").
			Field("campaign_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (NotificationCampaignConfirmation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("nonce_hash").Unique(),
		index.Fields("campaign_id", "issued_at", "id"),
		index.Fields("expires_at", "consumed_at"),
		index.Fields("operation_id").Unique(),
	}
}

func (NotificationCampaignConfirmation) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"notification_campaign_confirmation_nonce_size":                "octet_length(nonce_hash) = 32",
			"notification_campaign_confirmation_source_checksum_size":      "octet_length(normalized_source_checksum) = 32",
			"notification_campaign_confirmation_audience_fingerprint_size": "octet_length(audience_fingerprint) = 32",
			"notification_campaign_confirmation_request_hash_size":         "schedule_request_hash IS NULL OR octet_length(schedule_request_hash) = 32",
			"notification_campaign_confirmation_expiry_order":              "expires_at > issued_at AND dry_run_estimated_at <= issued_at",
			"notification_campaign_confirmation_consumption_shape": `(
				consumed_at IS NULL
				AND consumed_by_idempotency_key IS NULL
				AND schedule_request_hash IS NULL
				AND operation_id IS NULL
			) OR (
				consumed_at IS NOT NULL
				AND consumed_by_idempotency_key IS NOT NULL
				AND schedule_request_hash IS NOT NULL
				AND operation_id IS NOT NULL
			)`,
			"notification_campaign_confirmation_source_kind": "render_source_kind = 'notification_campaign_body'",
		}),
	}
}

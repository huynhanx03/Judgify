package schema

import (
	"fmt"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	contract "github.com/huynhanx03/judgify/internal/notification/contract"
	rendercontract "github.com/huynhanx03/judgify/internal/platform/contentrender/contract"
)

// NotificationCampaign is the versioned administrative aggregate. Authored
// content lives in immutable revisions; scheduling freezes exact render
// identities and delegates bounded delivery to a generic durable operation.
type NotificationCampaign struct{ ent.Schema }

func (NotificationCampaign) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (NotificationCampaign) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("status").Values(contract.CampaignStatuses()...).Default(contract.CampaignStatusDraft),
		field.UUID("current_draft_revision_id", uuid.UUID{}),
		field.UUID("scheduled_revision_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("scheduled_render_source_id", uuid.UUID{}).Optional().Nillable(),
		field.Enum("scheduled_render_source_kind").
			Values(rendercontract.SourceKindNotificationCampaignBody).
			Optional().
			Nillable(),
		field.Bytes("scheduled_normalized_source_checksum").Optional().Nillable().Sensitive(),
		field.UUID("scheduled_render_artifact_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("operation_id", uuid.UUID{}).Optional().Nillable(),
		field.Int64("version").Min(1).Default(1),
		field.Time("scheduled_for").Optional().Nillable(),
		field.String("last_failure_code").
			MaxLen(contract.MaximumCampaignFailureCodeBytes).
			Optional().
			Nillable(),
		field.UUID("created_by", uuid.UUID{}).Immutable(),
		field.UUID("scheduled_by", uuid.UUID{}).Optional().Nillable(),
		field.UUID("cancelled_by", uuid.UUID{}).Optional().Nillable(),
		field.String("schedule_reason").
			MaxLen(contract.MaximumCampaignReasonBytes).
			Optional().
			Nillable().
			Sensitive(),
		field.String("cancel_reason").
			MaxLen(contract.MaximumCampaignReasonBytes).
			Optional().
			Nillable().
			Sensitive(),
		field.Time("scheduled_at").Optional().Nillable(),
		field.Time("started_at").Optional().Nillable(),
		field.Time("completed_at").Optional().Nillable(),
		field.Time("cancelled_at").Optional().Nillable(),
	}
}

func (NotificationCampaign) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("creator", User.Type).
			Ref("created_notification_campaigns").
			Field("created_by").
			Unique().
			Required().
			Immutable(),
		edge.To("revisions", NotificationCampaignRevision.Type),
		edge.To("confirmations", NotificationCampaignConfirmation.Type),
		edge.To("recipients", NotificationCampaignRecipient.Type),
	}
}

func (NotificationCampaign) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "scheduled_for", "created_at", "id"),
		index.Fields("created_by", "created_at", "id"),
		index.Fields("operation_id").Unique(),
		index.Fields("scheduled_revision_id"),
	}
}

func (NotificationCampaign) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"notification_campaign_positive_version": "version >= 1",
			"notification_campaign_schedule_checksum": fmt.Sprintf(
				"scheduled_normalized_source_checksum IS NULL OR octet_length(scheduled_normalized_source_checksum) = %d",
				32,
			),
			"notification_campaign_schedule_shape": `(
				status = 'draft'
				AND scheduled_revision_id IS NULL
				AND scheduled_render_source_id IS NULL
				AND scheduled_render_source_kind IS NULL
				AND scheduled_normalized_source_checksum IS NULL
				AND scheduled_render_artifact_id IS NULL
				AND operation_id IS NULL
				AND scheduled_by IS NULL
				AND schedule_reason IS NULL
				AND scheduled_at IS NULL
				AND started_at IS NULL
				AND completed_at IS NULL
			) OR (
				status <> 'draft'
				AND scheduled_revision_id IS NOT NULL
				AND scheduled_render_source_id IS NOT NULL
				AND scheduled_render_source_kind = 'notification_campaign_body'
				AND scheduled_normalized_source_checksum IS NOT NULL
				AND scheduled_render_artifact_id IS NOT NULL
				AND operation_id IS NOT NULL
				AND scheduled_by IS NOT NULL
				AND schedule_reason IS NOT NULL
				AND scheduled_at IS NOT NULL
			)`,
			"notification_campaign_cancel_shape": `(
				status = 'cancelled'
				AND cancelled_by IS NOT NULL
				AND cancel_reason IS NOT NULL
				AND cancelled_at IS NOT NULL
			) OR (
				status <> 'cancelled'
				AND cancelled_by IS NULL
				AND cancel_reason IS NULL
				AND cancelled_at IS NULL
			)`,
			"notification_campaign_completion_shape": `(
				status IN ('completed', 'failed') AND completed_at IS NOT NULL
			) OR (
				status NOT IN ('completed', 'failed') AND completed_at IS NULL
			)`,
			"notification_campaign_failure_shape": `(
				status = 'failed' AND last_failure_code IS NOT NULL
			) OR (
				status <> 'failed'
			)`,
		}),
	}
}

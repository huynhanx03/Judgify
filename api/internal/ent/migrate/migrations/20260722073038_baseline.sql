-- Create "activated_judge_capabilities" table
CREATE TABLE "activated_judge_capabilities" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "release_id" character varying(128) NOT NULL,
  "approved_release_manifest_checksum" character varying(71) NOT NULL,
  "actor_id" uuid NOT NULL,
  "audit_id" uuid NOT NULL,
  "reason" character varying(500) NOT NULL,
  "judge_spec_protocol" character varying(128) NOT NULL,
  "runtime_key" character varying(64) NOT NULL,
  "runtime_profile_version" character varying(128) NOT NULL,
  "compiler_version" character varying(128) NOT NULL,
  "runtime_bootstrap_version" character varying(128) NOT NULL,
  "compile_command_id" character varying(128) NOT NULL,
  "run_command_id" character varying(128) NOT NULL,
  "image_digest" character varying(71) NOT NULL,
  "manifest_version" character varying(128) NOT NULL,
  "manifest_checksum" character varying(71) NOT NULL,
  "supervisor_protocol" character varying(128) NOT NULL,
  "checker_key" character varying(64) NOT NULL,
  "checker_version" character varying(64) NOT NULL,
  "capability_profile_hash" character varying(71) NOT NULL,
  "checker_implementation_checksum" character varying(71) NOT NULL,
  "smoke_checksum" character varying(71) NOT NULL,
  "golden_checksum" character varying(71) NOT NULL,
  "proof_worker_instance_id" uuid NOT NULL,
  "proof_boot_id" uuid NOT NULL,
  "state" character varying(16) NOT NULL DEFAULT 'active',
  "activated_at" timestamptz NOT NULL,
  "retired_at" timestamptz NULL,
  "retired_by" uuid NULL,
  "retirement_audit_id" uuid NULL,
  "retirement_reason" character varying(500) NULL,
  "proof_evidence_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "activated_judge_capability_identifiers" CHECK (((release_id)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((judge_spec_protocol)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((runtime_key)::text ~ '^[a-z0-9][a-z0-9._-]{0,63}$'::text) AND ((runtime_profile_version)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((compiler_version)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((runtime_bootstrap_version)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((compile_command_id)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((run_command_id)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((manifest_version)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((supervisor_protocol)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((checker_key)::text ~ '^[a-z0-9][a-z0-9._-]{0,63}$'::text) AND ((checker_version)::text ~ '^[a-z0-9][a-z0-9._-]{0,63}$'::text)),
  CONSTRAINT "activated_judge_capability_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (actor_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (audit_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (proof_worker_instance_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (proof_boot_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (proof_evidence_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "activated_judge_capability_pinned_digests" CHECK (((image_digest)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((manifest_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((approved_release_manifest_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((capability_profile_hash)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((checker_implementation_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((smoke_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((golden_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text)),
  CONSTRAINT "activated_judge_capability_state_times" CHECK ((((state)::text = 'active'::text) AND (retired_at IS NULL) AND (retired_by IS NULL) AND (retirement_audit_id IS NULL) AND (retirement_reason IS NULL)) OR (((state)::text = 'retired'::text) AND (retired_at IS NOT NULL) AND (retired_by IS NOT NULL) AND (retirement_audit_id IS NOT NULL) AND (retirement_reason IS NOT NULL) AND (retired_at >= activated_at)))
);
-- Create index "activatedjudgecapability_proof_worker_instance_id_proof_boot_id" to table: "activated_judge_capabilities"
CREATE INDEX "activatedjudgecapability_proof_worker_instance_id_proof_boot_id" ON "activated_judge_capabilities" ("proof_worker_instance_id", "proof_boot_id");
-- Create index "activatedjudgecapability_release_id_capability_profile_hash" to table: "activated_judge_capabilities"
CREATE UNIQUE INDEX "activatedjudgecapability_release_id_capability_profile_hash" ON "activated_judge_capabilities" ("release_id", "capability_profile_hash");
-- Create index "activatedjudgecapability_release_id_runtime_key_checker_key_che" to table: "activated_judge_capabilities"
CREATE UNIQUE INDEX "activatedjudgecapability_release_id_runtime_key_checker_key_che" ON "activated_judge_capabilities" ("release_id", "runtime_key", "checker_key", "checker_version") WHERE ((state)::text = 'active'::text);
-- Create index "activatedjudgecapability_state_release_id_capability_profile_ha" to table: "activated_judge_capabilities"
CREATE INDEX "activatedjudgecapability_state_release_id_capability_profile_ha" ON "activated_judge_capabilities" ("state", "release_id", "capability_profile_hash", "activated_at");
-- Create "attribute_definition_revisions" table
CREATE TABLE "attribute_definition_revisions" (
  "id" uuid NOT NULL,
  "revision" bigint NOT NULL,
  "data_type" character varying NOT NULL,
  "label" character varying NOT NULL,
  "description" character varying NULL,
  "visibility" character varying NOT NULL,
  "user_editable" boolean NOT NULL DEFAULT false,
  "display_order" bigint NOT NULL DEFAULT 0,
  "validation" jsonb NOT NULL,
  "created_by" uuid NULL,
  "created_at" timestamptz NOT NULL,
  "definition_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "attribute_revision_data_type_closed" CHECK ((data_type)::text = ANY ((ARRAY['string'::character varying, 'number'::character varying, 'boolean'::character varying, 'date'::character varying])::text[])),
  CONSTRAINT "attribute_revision_visibility_closed" CHECK ((visibility)::text = ANY ((ARRAY['public'::character varying, 'private'::character varying, 'admin'::character varying])::text[]))
);
-- Create index "attributedefinitionrevision_definition_id_created_at" to table: "attribute_definition_revisions"
CREATE INDEX "attributedefinitionrevision_definition_id_created_at" ON "attribute_definition_revisions" ("definition_id", "created_at");
-- Create index "attributedefinitionrevision_definition_id_revision" to table: "attribute_definition_revisions"
CREATE UNIQUE INDEX "attributedefinitionrevision_definition_id_revision" ON "attribute_definition_revisions" ("definition_id", "revision");
-- Create index "attributedefinitionrevision_id_definition_id" to table: "attribute_definition_revisions"
CREATE UNIQUE INDEX "attributedefinitionrevision_id_definition_id" ON "attribute_definition_revisions" ("id", "definition_id");
-- Create "attribute_definitions" table
CREATE TABLE "attribute_definitions" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "key" character varying NOT NULL,
  "kind" character varying NOT NULL DEFAULT 'custom',
  "required_on_onboarding" boolean NOT NULL DEFAULT false,
  "status" character varying NOT NULL DEFAULT 'active',
  "version" bigint NOT NULL DEFAULT 1,
  "draft_revision_id" uuid NULL,
  "active_revision_id" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "attribute_definition_kind_closed" CHECK ((kind)::text = ANY ((ARRAY['system'::character varying, 'custom'::character varying])::text[])),
  CONSTRAINT "attribute_definition_status_closed" CHECK ((status)::text = ANY ((ARRAY['active'::character varying, 'archived'::character varying])::text[]))
);
-- Create index "attribute_definitions_key_key" to table: "attribute_definitions"
CREATE UNIQUE INDEX "attribute_definitions_key_key" ON "attribute_definitions" ("key");
-- Create "audit_logs" table
CREATE TABLE "audit_logs" (
  "id" uuid NOT NULL,
  "actor_user_id" uuid NULL,
  "actor_session_id" uuid NULL,
  "action" character varying NOT NULL,
  "resource" character varying NOT NULL,
  "resource_id" character varying NOT NULL,
  "reason" character varying NOT NULL,
  "before_data" jsonb NOT NULL,
  "after_data" jsonb NOT NULL,
  "correlation_id" character varying NOT NULL,
  "occurred_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id")
);
-- Create index "auditlog_action_occurred_at_id" to table: "audit_logs"
CREATE INDEX "auditlog_action_occurred_at_id" ON "audit_logs" ("action", "occurred_at", "id");
-- Create index "auditlog_actor_user_id_occurred_at_id" to table: "audit_logs"
CREATE INDEX "auditlog_actor_user_id_occurred_at_id" ON "audit_logs" ("actor_user_id", "occurred_at", "id");
-- Create index "auditlog_correlation_id_occurred_at_id" to table: "audit_logs"
CREATE INDEX "auditlog_correlation_id_occurred_at_id" ON "audit_logs" ("correlation_id", "occurred_at", "id");
-- Create index "auditlog_occurred_at_id" to table: "audit_logs"
CREATE INDEX "auditlog_occurred_at_id" ON "audit_logs" ("occurred_at", "id");
-- Create index "auditlog_resource_id_occurred_at_id" to table: "audit_logs"
CREATE INDEX "auditlog_resource_id_occurred_at_id" ON "audit_logs" ("resource_id", "occurred_at", "id");
-- Create index "auditlog_resource_resource_id_occurred_at_id" to table: "audit_logs"
CREATE INDEX "auditlog_resource_resource_id_occurred_at_id" ON "audit_logs" ("resource", "resource_id", "occurred_at", "id");
-- Create "auth_refresh_receipts" table
CREATE TABLE "auth_refresh_receipts" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "idempotency_key" uuid NOT NULL,
  "recovery_ciphertext" bytea NOT NULL,
  "key_id" character varying NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "recovered_at" timestamptz NULL,
  "erased_at" timestamptz NULL,
  "presented_token_id" uuid NOT NULL,
  "replacement_token_id" uuid NOT NULL,
  "session_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "authrefreshreceipt_expires_at" to table: "auth_refresh_receipts"
CREATE INDEX "authrefreshreceipt_expires_at" ON "auth_refresh_receipts" ("expires_at");
-- Create index "authrefreshreceipt_replacement_token_id" to table: "auth_refresh_receipts"
CREATE UNIQUE INDEX "authrefreshreceipt_replacement_token_id" ON "auth_refresh_receipts" ("replacement_token_id");
-- Create index "authrefreshreceipt_session_id_presented_token_id_idempotency_ke" to table: "auth_refresh_receipts"
CREATE UNIQUE INDEX "authrefreshreceipt_session_id_presented_token_id_idempotency_ke" ON "auth_refresh_receipts" ("session_id", "presented_token_id", "idempotency_key");
-- Create "auth_refresh_tokens" table
CREATE TABLE "auth_refresh_tokens" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "token_hash" bytea NOT NULL,
  "status" character varying NOT NULL DEFAULT 'issued',
  "issued_at" timestamptz NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "consumed_at" timestamptz NULL,
  "revoked_at" timestamptz NULL,
  "parent_id" uuid NULL,
  "replaced_by_id" uuid NULL,
  "session_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "auth_refresh_tokens_auth_refresh_tokens_parent" FOREIGN KEY ("parent_id") REFERENCES "auth_refresh_tokens" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "auth_refresh_tokens_auth_refresh_tokens_replacement" FOREIGN KEY ("replaced_by_id") REFERENCES "auth_refresh_tokens" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "auth_refresh_consumed_complete" CHECK ((((status)::text = 'consumed'::text) AND (consumed_at IS NOT NULL) AND (replaced_by_id IS NOT NULL)) OR ((status)::text <> 'consumed'::text)),
  CONSTRAINT "auth_refresh_expiry_after_issue" CHECK (expires_at > issued_at),
  CONSTRAINT "auth_refresh_revoked_complete" CHECK ((((status)::text = 'revoked'::text) AND (revoked_at IS NOT NULL)) OR ((status)::text <> 'revoked'::text)),
  CONSTRAINT "auth_refresh_status_closed" CHECK ((status)::text = ANY ((ARRAY['issued'::character varying, 'consumed'::character varying, 'revoked'::character varying])::text[]))
);
-- Create index "auth_refresh_tokens_token_hash_key" to table: "auth_refresh_tokens"
CREATE UNIQUE INDEX "auth_refresh_tokens_token_hash_key" ON "auth_refresh_tokens" ("token_hash");
-- Create index "authrefreshtoken_parent_id" to table: "auth_refresh_tokens"
CREATE INDEX "authrefreshtoken_parent_id" ON "auth_refresh_tokens" ("parent_id");
-- Create index "authrefreshtoken_replaced_by_id" to table: "auth_refresh_tokens"
CREATE INDEX "authrefreshtoken_replaced_by_id" ON "auth_refresh_tokens" ("replaced_by_id");
-- Create index "authrefreshtoken_session_id_status_expires_at" to table: "auth_refresh_tokens"
CREATE INDEX "authrefreshtoken_session_id_status_expires_at" ON "auth_refresh_tokens" ("session_id", "status", "expires_at");
-- Create index "authrefreshtoken_status_expires_at" to table: "auth_refresh_tokens"
CREATE INDEX "authrefreshtoken_status_expires_at" ON "auth_refresh_tokens" ("status", "expires_at");
-- Create "auth_session_command_receipts" table
CREATE TABLE "auth_session_command_receipts" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "idempotency_key" uuid NOT NULL,
  "kind" character varying NOT NULL,
  "result_security_version" bigint NOT NULL DEFAULT 0,
  "expires_at" timestamptz NOT NULL,
  "actor_session_id" uuid NOT NULL,
  "target_session_id" uuid NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "auth_session_command_shape" CHECK ((((kind)::text = 'revoke'::text) AND (target_session_id IS NOT NULL) AND (result_security_version = 0)) OR (((kind)::text = 'logout_all'::text) AND (target_session_id IS NULL) AND (result_security_version > 0)))
);
-- Create index "authsessioncommandreceipt_expires_at" to table: "auth_session_command_receipts"
CREATE INDEX "authsessioncommandreceipt_expires_at" ON "auth_session_command_receipts" ("expires_at");
-- Create index "authsessioncommandreceipt_user_id_idempotency_key" to table: "auth_session_command_receipts"
CREATE UNIQUE INDEX "authsessioncommandreceipt_user_id_idempotency_key" ON "auth_session_command_receipts" ("user_id", "idempotency_key");
-- Create "auth_session_issuance_receipts" table
CREATE TABLE "auth_session_issuance_receipts" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "purpose" character varying NOT NULL,
  "request_or_ticket_id" uuid NULL,
  "pre_session_id" uuid NULL,
  "request_fingerprint" bytea NULL,
  "fingerprint_key_id" character varying NULL,
  "idempotency_key" uuid NOT NULL,
  "offer_checksum" bytea NULL,
  "recovery_ciphertext" bytea NOT NULL,
  "key_id" character varying NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "recovered_at" timestamptz NULL,
  "erased_at" timestamptz NULL,
  "refresh_token_id" uuid NOT NULL,
  "session_id" uuid NOT NULL,
  "trait_offer_id" uuid NULL,
  "user_id" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "auth_issuance_external_identity" CHECK (((purpose)::text = 'password_login'::text) OR (request_or_ticket_id IS NOT NULL)),
  CONSTRAINT "auth_issuance_fingerprint_pair" CHECK (((request_fingerprint IS NULL) AND (fingerprint_key_id IS NULL)) OR ((request_fingerprint IS NOT NULL) AND (fingerprint_key_id IS NOT NULL))),
  CONSTRAINT "auth_issuance_offer_checksum_shape" CHECK ((offer_checksum IS NULL) OR ((octet_length(offer_checksum) = 32) AND (offer_checksum <> decode(repeat('00'::text, 32), 'hex'::text)))),
  CONSTRAINT "auth_issuance_offer_pair" CHECK (((trait_offer_id IS NULL) AND (offer_checksum IS NULL)) OR ((trait_offer_id IS NOT NULL) AND (offer_checksum IS NOT NULL))),
  CONSTRAINT "auth_issuance_password_login_identity" CHECK ((((purpose)::text = 'password_login'::text) AND (user_id IS NOT NULL) AND (pre_session_id IS NOT NULL)) OR ((purpose)::text <> 'password_login'::text)),
  CONSTRAINT "auth_issuance_purpose_closed" CHECK ((purpose)::text = ANY ((ARRAY['password_login'::character varying, 'password_registration'::character varying, 'oauth_login'::character varying, 'oauth_registration'::character varying])::text[])),
  CONSTRAINT "auth_issuance_registration_offer" CHECK ((((purpose)::text = ANY ((ARRAY['password_registration'::character varying, 'oauth_registration'::character varying])::text[])) AND (trait_offer_id IS NOT NULL)) OR (((purpose)::text <> ALL ((ARRAY['password_registration'::character varying, 'oauth_registration'::character varying])::text[])) AND (trait_offer_id IS NULL)))
);
-- Create index "authsessionissuancereceipt_expires_at" to table: "auth_session_issuance_receipts"
CREATE INDEX "authsessionissuancereceipt_expires_at" ON "auth_session_issuance_receipts" ("expires_at");
-- Create index "authsessionissuancereceipt_purpose_request_or_ticket_id_idempot" to table: "auth_session_issuance_receipts"
CREATE UNIQUE INDEX "authsessionissuancereceipt_purpose_request_or_ticket_id_idempot" ON "auth_session_issuance_receipts" ("purpose", "request_or_ticket_id", "idempotency_key") WHERE ((purpose)::text <> 'password_login'::text);
-- Create index "authsessionissuancereceipt_refresh_token_id" to table: "auth_session_issuance_receipts"
CREATE UNIQUE INDEX "authsessionissuancereceipt_refresh_token_id" ON "auth_session_issuance_receipts" ("refresh_token_id");
-- Create index "authsessionissuancereceipt_session_id" to table: "auth_session_issuance_receipts"
CREATE UNIQUE INDEX "authsessionissuancereceipt_session_id" ON "auth_session_issuance_receipts" ("session_id");
-- Create index "authsessionissuancereceipt_trait_offer_id" to table: "auth_session_issuance_receipts"
CREATE UNIQUE INDEX "authsessionissuancereceipt_trait_offer_id" ON "auth_session_issuance_receipts" ("trait_offer_id") WHERE (trait_offer_id IS NOT NULL);
-- Create index "authsessionissuancereceipt_user_id_pre_session_id_idempotency_k" to table: "auth_session_issuance_receipts"
CREATE UNIQUE INDEX "authsessionissuancereceipt_user_id_pre_session_id_idempotency_k" ON "auth_session_issuance_receipts" ("user_id", "pre_session_id", "idempotency_key") WHERE ((purpose)::text = 'password_login'::text);
-- Create "auth_sessions" table
CREATE TABLE "auth_sessions" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "family_id" uuid NOT NULL,
  "status" character varying NOT NULL DEFAULT 'active',
  "user_security_version" bigint NOT NULL,
  "last_seen_at" timestamptz NOT NULL,
  "last_reauthenticated_at" timestamptz NOT NULL,
  "idle_expires_at" timestamptz NOT NULL,
  "absolute_expires_at" timestamptz NOT NULL,
  "revoked_at" timestamptz NULL,
  "revoke_reason" character varying NULL,
  "ip_hash" bytea NULL,
  "user_agent" character varying NULL,
  "created_cid" character varying NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "auth_session_expiry_order" CHECK (idle_expires_at <= absolute_expires_at),
  CONSTRAINT "auth_session_revocation_complete" CHECK ((((status)::text = 'revoked'::text) AND (revoked_at IS NOT NULL) AND (revoke_reason IS NOT NULL)) OR ((status)::text <> 'revoked'::text)),
  CONSTRAINT "auth_session_status_closed" CHECK ((status)::text = ANY ((ARRAY['active'::character varying, 'revoked'::character varying, 'expired'::character varying])::text[]))
);
-- Create index "authsession_family_id" to table: "auth_sessions"
CREATE INDEX "authsession_family_id" ON "auth_sessions" ("family_id");
-- Create index "authsession_status_absolute_expires_at" to table: "auth_sessions"
CREATE INDEX "authsession_status_absolute_expires_at" ON "auth_sessions" ("status", "absolute_expires_at");
-- Create index "authsession_status_idle_expires_at" to table: "auth_sessions"
CREATE INDEX "authsession_status_idle_expires_at" ON "auth_sessions" ("status", "idle_expires_at");
-- Create index "authsession_user_id_created_at_id" to table: "auth_sessions"
CREATE INDEX "authsession_user_id_created_at_id" ON "auth_sessions" ("user_id", "created_at", "id");
-- Create index "authsession_user_id_status" to table: "auth_sessions"
CREATE INDEX "authsession_user_id_status" ON "auth_sessions" ("user_id", "status");
-- Create "authorization_actions" table
CREATE TABLE "authorization_actions" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "key" character varying NOT NULL,
  "label" character varying NOT NULL,
  "description" character varying NULL,
  "display_order" bigint NOT NULL DEFAULT 0,
  "catalog_revision" bigint NOT NULL DEFAULT 1,
  "resource_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "authorizationaction_resource_id_display_order_key" to table: "authorization_actions"
CREATE INDEX "authorizationaction_resource_id_display_order_key" ON "authorization_actions" ("resource_id", "display_order", "key");
-- Create index "authorizationaction_resource_id_key" to table: "authorization_actions"
CREATE UNIQUE INDEX "authorizationaction_resource_id_key" ON "authorization_actions" ("resource_id", "key");
-- Create "authorization_resources" table
CREATE TABLE "authorization_resources" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "key" character varying NOT NULL,
  "label" character varying NOT NULL,
  "description" character varying NULL,
  "display_order" bigint NOT NULL DEFAULT 0,
  "catalog_revision" bigint NOT NULL DEFAULT 1,
  PRIMARY KEY ("id")
);
-- Create index "authorization_resources_key_key" to table: "authorization_resources"
CREATE UNIQUE INDEX "authorization_resources_key_key" ON "authorization_resources" ("key");
-- Create "authorization_states" table
CREATE TABLE "authorization_states" (
  "id" character varying NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "desired_revision" bigint NOT NULL DEFAULT 0,
  "updated_by" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "authorization_states_id_check" CHECK ((id)::text = 'default'::text)
);
-- Create "casbin_rules" table
CREATE TABLE "casbin_rules" (
  "id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY,
  "ptype" character varying NOT NULL DEFAULT '',
  "v0" character varying NOT NULL DEFAULT '',
  "v1" character varying NOT NULL DEFAULT '',
  "v2" character varying NOT NULL DEFAULT '',
  "v3" character varying NOT NULL DEFAULT '',
  "v4" character varying NOT NULL DEFAULT '',
  "v5" character varying NOT NULL DEFAULT '',
  PRIMARY KEY ("id")
);
-- Create index "casbinrule_ptype_v0_v1_v2_v3_v4_v5" to table: "casbin_rules"
CREATE UNIQUE INDEX "casbinrule_ptype_v0_v1_v2_v3_v4_v5" ON "casbin_rules" ("ptype", "v0", "v1", "v2", "v3", "v4", "v5");
-- Create "checker_profiles" table
CREATE TABLE "checker_profiles" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "checker_key" character varying(64) NOT NULL,
  "checker_version" character varying(64) NOT NULL,
  "implementation_checksum" character varying(71) NOT NULL,
  "config_kind" character varying NOT NULL,
  "active" boolean NOT NULL DEFAULT false,
  PRIMARY KEY ("id"),
  CONSTRAINT "checker_profile_config_kind_closed" CHECK ((config_kind)::text = ANY ((ARRAY['none'::character varying, 'float_tolerance'::character varying])::text[])),
  CONSTRAINT "checker_profile_identity_closed_shape" CHECK (((checker_key)::text ~ '^[a-z0-9][a-z0-9._-]*$'::text) AND ((checker_version)::text ~ '^[a-z0-9][a-z0-9._-]*$'::text)),
  CONSTRAINT "checker_profile_implementation_digest" CHECK ((implementation_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text)
);
-- Create index "checkerprofile_checker_key_active_checker_version" to table: "checker_profiles"
CREATE INDEX "checkerprofile_checker_key_active_checker_version" ON "checker_profiles" ("checker_key", "active", "checker_version");
-- Create index "checkerprofile_checker_key_checker_version" to table: "checker_profiles"
CREATE UNIQUE INDEX "checkerprofile_checker_key_checker_version" ON "checker_profiles" ("checker_key", "checker_version");
-- Create "content_render_artifacts" table
CREATE TABLE "content_render_artifacts" (
  "id" uuid NOT NULL,
  "render_source_kind" character varying NOT NULL,
  "source_checksum" bytea NOT NULL,
  "renderer_version" character varying NOT NULL,
  "sanitizer_policy_version" character varying NOT NULL,
  "sanitized_html" text NULL,
  "artifact_checksum" bytea NULL,
  "status" character varying NOT NULL,
  "diagnostics" jsonb NOT NULL,
  "created_by" uuid NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "render_source_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "content_render_artifact_diagnostics_bound" CHECK ((jsonb_typeof(diagnostics) = 'array'::text) AND (jsonb_array_length(diagnostics) <= 8) AND (octet_length((diagnostics)::text) <= 2048)),
  CONSTRAINT "content_render_artifact_diagnostics_safe_codes" CHECK ((diagnostics <@ '[{"code": "invalid_utf8"}, {"code": "ambiguous_encoding"}, {"code": "source_too_large"}, {"code": "output_too_large"}, {"code": "nesting_too_deep"}, {"code": "render_rejected"}]'::jsonb) AND (NOT (diagnostics @? '$[*]?(!(exists (@."code")))'::jsonpath))),
  CONSTRAINT "content_render_artifact_html_bound" CHECK ((sanitized_html IS NULL) OR (octet_length(sanitized_html) <= 2097152)),
  CONSTRAINT "content_render_artifact_nonzero_checksums" CHECK ((source_checksum <> decode(repeat('00'::text, 32), 'hex'::text)) AND ((artifact_checksum IS NULL) OR (artifact_checksum <> decode(repeat('00'::text, 32), 'hex'::text)))),
  CONSTRAINT "content_render_artifact_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (render_source_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((created_by IS NULL) OR (created_by <> '00000000-0000-0000-0000-000000000000'::uuid))),
  CONSTRAINT "content_render_artifact_ready_shape" CHECK (((status)::text <> 'ready'::text) OR ((sanitized_html IS NOT NULL) AND (artifact_checksum IS NOT NULL) AND (octet_length(artifact_checksum) = 32) AND (jsonb_array_length(diagnostics) = 0))),
  CONSTRAINT "content_render_artifact_rejected_shape" CHECK (((status)::text <> 'rejected'::text) OR ((sanitized_html IS NULL) AND (artifact_checksum IS NULL) AND ((jsonb_array_length(diagnostics) >= 1) AND (jsonb_array_length(diagnostics) <= 8)))),
  CONSTRAINT "content_render_artifact_renderer_version_bound" CHECK (((octet_length((renderer_version)::text) >= 1) AND (octet_length((renderer_version)::text) <= 64)) AND ((renderer_version)::text ~ '^[a-z0-9._-]+$'::text)),
  CONSTRAINT "content_render_artifact_sanitizer_policy_version_bound" CHECK (((octet_length((sanitizer_policy_version)::text) >= 1) AND (octet_length((sanitizer_policy_version)::text) <= 64)) AND ((sanitizer_policy_version)::text ~ '^[a-z0-9._-]+$'::text)),
  CONSTRAINT "content_render_artifact_source_checksum_length" CHECK (octet_length(source_checksum) = 32),
  CONSTRAINT "content_render_artifact_source_kind_closed" CHECK ((render_source_kind)::text = ANY ((ARRAY['problem_statement'::character varying, 'contest_description'::character varying, 'contest_announcement'::character varying, 'contest_clarification'::character varying, 'material_body'::character varying, 'notification_campaign_body'::character varying])::text[])),
  CONSTRAINT "content_render_artifact_status_closed" CHECK ((status)::text = ANY ((ARRAY['ready'::character varying, 'rejected'::character varying])::text[]))
);
-- Create index "contentrenderartifact_id_render_source_id_render_source_kind_so" to table: "content_render_artifacts"
CREATE UNIQUE INDEX "contentrenderartifact_id_render_source_id_render_source_kind_so" ON "content_render_artifacts" ("id", "render_source_id", "render_source_kind", "source_checksum");
-- Create index "contentrenderartifact_render_source_id_render_source_kind_sourc" to table: "content_render_artifacts"
CREATE INDEX "contentrenderartifact_render_source_id_render_source_kind_sourc" ON "content_render_artifacts" ("render_source_id", "render_source_kind", "source_checksum");
-- Create index "contentrenderartifact_render_source_id_renderer_version_sanitiz" to table: "content_render_artifacts"
CREATE UNIQUE INDEX "contentrenderartifact_render_source_id_renderer_version_sanitiz" ON "content_render_artifacts" ("render_source_id", "renderer_version", "sanitizer_policy_version");
-- Create "content_render_sources" table
CREATE TABLE "content_render_sources" (
  "id" uuid NOT NULL,
  "owner_revision_id" uuid NOT NULL,
  "source_kind" character varying NOT NULL,
  "source_checksum" bytea NOT NULL,
  "created_by" uuid NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "content_render_source_checksum_length" CHECK (octet_length(source_checksum) = 32),
  CONSTRAINT "content_render_source_kind_closed" CHECK ((source_kind)::text = ANY ((ARRAY['problem_statement'::character varying, 'contest_description'::character varying, 'contest_announcement'::character varying, 'contest_clarification'::character varying, 'material_body'::character varying, 'notification_campaign_body'::character varying])::text[])),
  CONSTRAINT "content_render_source_nonzero_checksum" CHECK (source_checksum <> decode(repeat('00'::text, 32), 'hex'::text)),
  CONSTRAINT "content_render_source_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (owner_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((created_by IS NULL) OR (created_by <> '00000000-0000-0000-0000-000000000000'::uuid)))
);
-- Create index "contentrendersource_id_owner_revision_id_source_kind_source_che" to table: "content_render_sources"
CREATE UNIQUE INDEX "contentrendersource_id_owner_revision_id_source_kind_source_che" ON "content_render_sources" ("id", "owner_revision_id", "source_kind", "source_checksum");
-- Create index "contentrendersource_id_source_kind_source_checksum" to table: "content_render_sources"
CREATE UNIQUE INDEX "contentrendersource_id_source_kind_source_checksum" ON "content_render_sources" ("id", "source_kind", "source_checksum");
-- Create index "contentrendersource_source_kind_owner_revision_id" to table: "content_render_sources"
CREATE UNIQUE INDEX "contentrendersource_source_kind_owner_revision_id" ON "content_render_sources" ("source_kind", "owner_revision_id");
-- Create "contest_announcements" table
CREATE TABLE "contest_announcements" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "status" character varying NOT NULL DEFAULT 'draft',
  "audience" character varying NOT NULL,
  "current_draft_revision_id" uuid NOT NULL,
  "current_draft_owner_type" character varying NOT NULL DEFAULT 'announcement',
  "current_draft_content_kind" character varying NOT NULL DEFAULT 'announcement_body',
  "current_draft_render_source_id" uuid NOT NULL,
  "current_draft_render_source_kind" character varying NOT NULL,
  "current_draft_normalized_source_checksum" bytea NOT NULL,
  "current_draft_render_artifact_id" uuid NOT NULL,
  "published_revision_id" uuid NULL,
  "published_owner_type" character varying NULL,
  "published_content_kind" character varying NULL,
  "published_render_source_id" uuid NULL,
  "published_render_source_kind" character varying NULL,
  "published_normalized_source_checksum" bytea NULL,
  "published_render_artifact_id" uuid NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "scheduled_for" timestamptz NULL,
  "published_at" timestamptz NULL,
  "withdrawn_at" timestamptz NULL,
  "contest_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "contest_announcement_draft_checksum" CHECK ((octet_length(current_draft_normalized_source_checksum) = 32) AND (current_draft_normalized_source_checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "contest_announcement_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (contest_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (current_draft_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (current_draft_render_source_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (current_draft_render_artifact_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "contest_announcement_published_pointer" CHECK (((published_revision_id IS NULL) AND (published_owner_type IS NULL) AND (published_content_kind IS NULL) AND (published_render_source_id IS NULL) AND (published_render_source_kind IS NULL) AND (published_normalized_source_checksum IS NULL) AND (published_render_artifact_id IS NULL)) OR ((published_revision_id IS NOT NULL) AND (published_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((published_owner_type)::text = 'announcement'::text) AND ((published_content_kind)::text = 'announcement_body'::text) AND (published_render_source_id IS NOT NULL) AND (published_render_source_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((published_render_source_kind)::text = 'contest_announcement'::text) AND (octet_length(published_normalized_source_checksum) = 32) AND (published_normalized_source_checksum <> decode(repeat('00'::text, 32), 'hex'::text)) AND (published_render_artifact_id IS NOT NULL) AND (published_render_artifact_id <> '00000000-0000-0000-0000-000000000000'::uuid))),
  CONSTRAINT "contest_announcement_state" CHECK ((((status)::text = 'draft'::text) AND (published_at IS NULL) AND (withdrawn_at IS NULL)) OR (((status)::text = 'published'::text) AND (published_at IS NOT NULL) AND (withdrawn_at IS NULL) AND (published_revision_id IS NOT NULL)) OR (((status)::text = 'withdrawn'::text) AND (withdrawn_at IS NOT NULL)))
);
-- Create index "contestannouncement_contest_id_audience_status_published_at_id" to table: "contest_announcements"
CREATE INDEX "contestannouncement_contest_id_audience_status_published_at_id" ON "contest_announcements" ("contest_id", "audience", "status", "published_at", "id");
-- Create index "contestannouncement_contest_id_status_published_at_id" to table: "contest_announcements"
CREATE INDEX "contestannouncement_contest_id_status_published_at_id" ON "contest_announcements" ("contest_id", "status", "published_at", "id");
-- Create index "contestannouncement_current_draft_revision_id_id_current_draft_" to table: "contest_announcements"
CREATE UNIQUE INDEX "contestannouncement_current_draft_revision_id_id_current_draft_" ON "contest_announcements" ("current_draft_revision_id", "id", "current_draft_owner_type", "current_draft_content_kind", "current_draft_render_source_id", "current_draft_render_source_kind", "current_draft_normalized_source_checksum");
-- Create index "contestannouncement_published_revision_id_id_published_owner_ty" to table: "contest_announcements"
CREATE UNIQUE INDEX "contestannouncement_published_revision_id_id_published_owner_ty" ON "contest_announcements" ("published_revision_id", "id", "published_owner_type", "published_content_kind", "published_render_source_id", "published_render_source_kind", "published_normalized_source_checksum");
-- Create index "contestannouncement_status_scheduled_for_id" to table: "contest_announcements"
CREATE INDEX "contestannouncement_status_scheduled_for_id" ON "contest_announcements" ("status", "scheduled_for", "id");
-- Create "contest_clarifications" table
CREATE TABLE "contest_clarifications" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "status" character varying NOT NULL DEFAULT 'open',
  "answer_audience" character varying NOT NULL DEFAULT 'requester',
  "question_revision_id" uuid NOT NULL,
  "question_owner_type" character varying NOT NULL DEFAULT 'clarification',
  "question_content_kind" character varying NOT NULL DEFAULT 'question',
  "question_render_source_id" uuid NOT NULL,
  "question_render_source_kind" character varying NOT NULL,
  "question_normalized_source_checksum" bytea NOT NULL,
  "question_render_artifact_id" uuid NOT NULL,
  "current_public_question_revision_id" uuid NULL,
  "current_public_question_owner_type" character varying NULL,
  "current_public_question_content_kind" character varying NULL,
  "current_public_question_render_source_id" uuid NULL,
  "current_public_question_render_source_kind" character varying NULL,
  "current_public_question_normalized_source_checksum" bytea NULL,
  "current_public_question_render_artifact_id" uuid NULL,
  "current_answer_revision_id" uuid NULL,
  "current_answer_owner_type" character varying NULL,
  "current_answer_content_kind" character varying NULL,
  "current_answer_render_source_id" uuid NULL,
  "current_answer_render_source_kind" character varying NULL,
  "current_answer_normalized_source_checksum" bytea NULL,
  "current_answer_render_artifact_id" uuid NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "answered_at" timestamptz NULL,
  "closed_at" timestamptz NULL,
  "withdrawn_at" timestamptz NULL,
  "contest_id" uuid NOT NULL,
  "contest_problem_id" uuid NULL,
  "requester_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "contest_clarification_answer_pointer" CHECK (((current_answer_revision_id IS NULL) AND (current_answer_owner_type IS NULL) AND (current_answer_content_kind IS NULL) AND (current_answer_render_source_id IS NULL) AND (current_answer_render_source_kind IS NULL) AND (current_answer_normalized_source_checksum IS NULL) AND (current_answer_render_artifact_id IS NULL)) OR ((current_answer_revision_id IS NOT NULL) AND (current_answer_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((current_answer_owner_type)::text = 'clarification'::text) AND ((current_answer_content_kind)::text = 'answer'::text) AND (current_answer_render_source_id IS NOT NULL) AND (current_answer_render_source_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((current_answer_render_source_kind)::text = 'contest_clarification'::text) AND (octet_length(current_answer_normalized_source_checksum) = 32) AND (current_answer_normalized_source_checksum <> decode(repeat('00'::text, 32), 'hex'::text)) AND (current_answer_render_artifact_id IS NOT NULL) AND (current_answer_render_artifact_id <> '00000000-0000-0000-0000-000000000000'::uuid))),
  CONSTRAINT "contest_clarification_audience" CHECK (((answer_audience)::text <> 'participants'::text) OR ((current_public_question_revision_id IS NOT NULL) AND (current_answer_revision_id IS NOT NULL))),
  CONSTRAINT "contest_clarification_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (contest_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (requester_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((contest_problem_id IS NULL) OR (contest_problem_id <> '00000000-0000-0000-0000-000000000000'::uuid)) AND (question_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (question_render_source_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (question_render_artifact_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "contest_clarification_public_pointer" CHECK (((current_public_question_revision_id IS NULL) AND (current_public_question_owner_type IS NULL) AND (current_public_question_content_kind IS NULL) AND (current_public_question_render_source_id IS NULL) AND (current_public_question_render_source_kind IS NULL) AND (current_public_question_normalized_source_checksum IS NULL) AND (current_public_question_render_artifact_id IS NULL)) OR ((current_public_question_revision_id IS NOT NULL) AND (current_public_question_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((current_public_question_owner_type)::text = 'clarification'::text) AND ((current_public_question_content_kind)::text = 'public_question_summary'::text) AND (current_public_question_render_source_id IS NOT NULL) AND (current_public_question_render_source_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((current_public_question_render_source_kind)::text = 'contest_clarification'::text) AND (octet_length(current_public_question_normalized_source_checksum) = 32) AND (current_public_question_normalized_source_checksum <> decode(repeat('00'::text, 32), 'hex'::text)) AND (current_public_question_render_artifact_id IS NOT NULL) AND (current_public_question_render_artifact_id <> '00000000-0000-0000-0000-000000000000'::uuid))),
  CONSTRAINT "contest_clarification_question_checksum" CHECK ((octet_length(question_normalized_source_checksum) = 32) AND (question_normalized_source_checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "contest_clarification_state" CHECK ((((status)::text = 'open'::text) AND (answered_at IS NULL) AND (closed_at IS NULL) AND (withdrawn_at IS NULL) AND (current_answer_revision_id IS NULL)) OR (((status)::text = 'answered'::text) AND (answered_at IS NOT NULL) AND (closed_at IS NULL) AND (withdrawn_at IS NULL) AND (current_answer_revision_id IS NOT NULL)) OR (((status)::text = 'closed'::text) AND (closed_at IS NOT NULL) AND (withdrawn_at IS NULL)) OR (((status)::text = 'withdrawn'::text) AND (withdrawn_at IS NOT NULL) AND (answered_at IS NULL) AND (closed_at IS NULL) AND (current_answer_revision_id IS NULL)))
);
-- Create index "contestclarification_contest_id_answer_audience_status_answered" to table: "contest_clarifications"
CREATE INDEX "contestclarification_contest_id_answer_audience_status_answered" ON "contest_clarifications" ("contest_id", "answer_audience", "status", "answered_at", "id");
-- Create index "contestclarification_contest_id_requester_id_created_at_id" to table: "contest_clarifications"
CREATE INDEX "contestclarification_contest_id_requester_id_created_at_id" ON "contest_clarifications" ("contest_id", "requester_id", "created_at", "id");
-- Create index "contestclarification_contest_id_status_created_at_id" to table: "contest_clarifications"
CREATE INDEX "contestclarification_contest_id_status_created_at_id" ON "contest_clarifications" ("contest_id", "status", "created_at", "id");
-- Create index "contestclarification_current_answer_revision_id_id_current_answ" to table: "contest_clarifications"
CREATE UNIQUE INDEX "contestclarification_current_answer_revision_id_id_current_answ" ON "contest_clarifications" ("current_answer_revision_id", "id", "current_answer_owner_type", "current_answer_content_kind", "current_answer_render_source_id", "current_answer_render_source_kind", "current_answer_normalized_source_checksum");
-- Create index "contestclarification_current_public_question_revision_id_id_cur" to table: "contest_clarifications"
CREATE UNIQUE INDEX "contestclarification_current_public_question_revision_id_id_cur" ON "contest_clarifications" ("current_public_question_revision_id", "id", "current_public_question_owner_type", "current_public_question_content_kind", "current_public_question_render_source_id", "current_public_question_render_source_kind", "current_public_question_normalized_source_checksum");
-- Create index "contestclarification_question_revision_id_id_question_owner_typ" to table: "contest_clarifications"
CREATE UNIQUE INDEX "contestclarification_question_revision_id_id_question_owner_typ" ON "contest_clarifications" ("question_revision_id", "id", "question_owner_type", "question_content_kind", "question_render_source_id", "question_render_source_kind", "question_normalized_source_checksum");
-- Create "contest_content_commands" table
CREATE TABLE "contest_content_commands" (
  "id" uuid NOT NULL,
  "command_kind" character varying NOT NULL,
  "idempotency_key" uuid NOT NULL,
  "request_hash" bytea NOT NULL,
  "actor_session_id" uuid NOT NULL,
  "result_owner_type" character varying NULL,
  "result_content_id" uuid NULL,
  "result_revision_id" uuid NULL,
  "result_version" bigint NULL,
  "result_projection" bytea NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "contest_id" uuid NOT NULL,
  "actor_user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "contest_content_command_hash" CHECK ((octet_length(request_hash) = 32) AND (request_hash <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "contest_content_command_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (contest_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (idempotency_key <> '00000000-0000-0000-0000-000000000000'::uuid) AND (actor_user_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (actor_session_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "contest_content_command_result" CHECK (((result_owner_type IS NULL) AND (result_content_id IS NULL) AND (result_revision_id IS NULL) AND (result_version IS NULL) AND (result_projection IS NULL)) OR ((result_owner_type IS NOT NULL) AND (result_content_id IS NOT NULL) AND (result_content_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (result_version IS NOT NULL) AND (result_version >= 1) AND (result_projection IS NOT NULL) AND ((octet_length(result_projection) >= 2) AND (octet_length(result_projection) <= 1048576))))
);
-- Create index "contestcontentcommand_command_kind_idempotency_key" to table: "contest_content_commands"
CREATE UNIQUE INDEX "contestcontentcommand_command_kind_idempotency_key" ON "contest_content_commands" ("command_kind", "idempotency_key");
-- Create index "contestcontentcommand_contest_id_created_at" to table: "contest_content_commands"
CREATE INDEX "contestcontentcommand_contest_id_created_at" ON "contest_content_commands" ("contest_id", "created_at");
-- Create "contest_content_revisions" table
CREATE TABLE "contest_content_revisions" (
  "id" uuid NOT NULL,
  "owner_type" character varying NOT NULL,
  "owner_id" uuid NOT NULL,
  "content_kind" character varying NOT NULL,
  "revision_number" bigint NOT NULL,
  "title" character varying NULL,
  "markdown" text NOT NULL,
  "render_source_id" uuid NOT NULL,
  "render_source_kind" character varying NOT NULL,
  "normalized_source_checksum" bytea NOT NULL,
  "render_artifact_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "author_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "contest_content_revision_checksum" CHECK ((octet_length(normalized_source_checksum) = 32) AND (normalized_source_checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "contest_content_revision_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (owner_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (author_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (render_source_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (render_artifact_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "contest_content_revision_shape" CHECK ((((owner_type)::text = 'announcement'::text) AND ((content_kind)::text = 'announcement_body'::text) AND ((render_source_kind)::text = 'contest_announcement'::text) AND (title IS NOT NULL) AND ((octet_length((title)::text) >= 1) AND (octet_length((title)::text) <= 200)) AND ((octet_length(markdown) >= 1) AND (octet_length(markdown) <= 65536))) OR (((owner_type)::text = 'clarification'::text) AND ((content_kind)::text = ANY ((ARRAY['question'::character varying, 'public_question_summary'::character varying, 'answer'::character varying])::text[])) AND ((render_source_kind)::text = 'contest_clarification'::text) AND (title IS NULL) AND ((octet_length(markdown) >= 1) AND (octet_length(markdown) <=
CASE content_kind
    WHEN 'question'::text THEN 16384
    WHEN 'public_question_summary'::text THEN 16384
    ELSE 32768
END))))
);
-- Create index "contestcontentrevision_id_owner_type_owner_id_content_kind_rend" to table: "contest_content_revisions"
CREATE UNIQUE INDEX "contestcontentrevision_id_owner_type_owner_id_content_kind_rend" ON "contest_content_revisions" ("id", "owner_type", "owner_id", "content_kind", "render_source_id", "render_source_kind", "normalized_source_checksum");
-- Create index "contestcontentrevision_id_render_source_id_render_source_kind_n" to table: "contest_content_revisions"
CREATE UNIQUE INDEX "contestcontentrevision_id_render_source_id_render_source_kind_n" ON "contest_content_revisions" ("id", "render_source_id", "render_source_kind", "normalized_source_checksum");
-- Create index "contestcontentrevision_owner_type_owner_id_content_kind_revisio" to table: "contest_content_revisions"
CREATE UNIQUE INDEX "contestcontentrevision_owner_type_owner_id_content_kind_revisio" ON "contest_content_revisions" ("owner_type", "owner_id", "content_kind", "revision_number");
-- Create index "contestcontentrevision_owner_type_owner_id_created_at_id" to table: "contest_content_revisions"
CREATE INDEX "contestcontentrevision_owner_type_owner_id_created_at_id" ON "contest_content_revisions" ("owner_type", "owner_id", "created_at", "id");
-- Create index "contestcontentrevision_render_artifact_id_render_source_id_rend" to table: "contest_content_revisions"
CREATE UNIQUE INDEX "contestcontentrevision_render_artifact_id_render_source_id_rend" ON "contest_content_revisions" ("render_artifact_id", "render_source_id", "render_source_kind", "normalized_source_checksum");
-- Create "contest_draft_problems" table
CREATE TABLE "contest_draft_problems" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "display_order" bigint NOT NULL,
  "alias" character varying NOT NULL,
  "points" bigint NULL,
  "visible_before_start" boolean NOT NULL DEFAULT true,
  "contest_id" uuid NOT NULL,
  "problem_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "contest_draft_problem_alias" CHECK ((alias)::text ~ '^[A-Z][A-Z0-9]{0,15}$'::text),
  CONSTRAINT "contest_draft_problem_points" CHECK ((points IS NULL) OR (points > 0))
);
-- Create index "contestdraftproblem_contest_id_alias" to table: "contest_draft_problems"
CREATE UNIQUE INDEX "contestdraftproblem_contest_id_alias" ON "contest_draft_problems" ("contest_id", "alias");
-- Create index "contestdraftproblem_contest_id_display_order" to table: "contest_draft_problems"
CREATE UNIQUE INDEX "contestdraftproblem_contest_id_display_order" ON "contest_draft_problems" ("contest_id", "display_order");
-- Create index "contestdraftproblem_contest_id_problem_id" to table: "contest_draft_problems"
CREATE UNIQUE INDEX "contestdraftproblem_contest_id_problem_id" ON "contest_draft_problems" ("contest_id", "problem_id");
-- Create index "contestdraftproblem_problem_id" to table: "contest_draft_problems"
CREATE INDEX "contestdraftproblem_problem_id" ON "contest_draft_problems" ("problem_id");
-- Create "contest_problem_results" table
CREATE TABLE "contest_problem_results" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "accepted" boolean NOT NULL DEFAULT false,
  "accepted_elapsed_ms" bigint NULL,
  "wrong_attempts_before_accept" bigint NOT NULL DEFAULT 0,
  "best_score" bigint NOT NULL DEFAULT 0,
  "last_accepted_sequence" bigint NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "contest_id" uuid NOT NULL,
  "contest_problem_id" uuid NOT NULL,
  "problem_id" uuid NOT NULL,
  "last_scoring_submission_id" uuid NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "contest_problem_result_acceptance" CHECK ((accepted AND (accepted_elapsed_ms IS NOT NULL) AND (last_scoring_submission_id IS NOT NULL) AND (last_accepted_sequence IS NOT NULL)) OR ((NOT accepted) AND (accepted_elapsed_ms IS NULL) AND (last_accepted_sequence IS NULL))),
  CONSTRAINT "contest_problem_result_nonzero_ids" CHECK ((contest_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (user_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (problem_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (contest_problem_id <> '00000000-0000-0000-0000-000000000000'::uuid))
);
-- Create index "contestproblemresult_contest_id_user_id_contest_problem_id" to table: "contest_problem_results"
CREATE UNIQUE INDEX "contestproblemresult_contest_id_user_id_contest_problem_id" ON "contest_problem_results" ("contest_id", "user_id", "contest_problem_id");
-- Create index "contestproblemresult_contest_id_user_id_problem_id" to table: "contest_problem_results"
CREATE UNIQUE INDEX "contestproblemresult_contest_id_user_id_problem_id" ON "contest_problem_results" ("contest_id", "user_id", "problem_id");
-- Create index "contestproblemresult_last_scoring_submission_id" to table: "contest_problem_results"
CREATE INDEX "contestproblemresult_last_scoring_submission_id" ON "contest_problem_results" ("last_scoring_submission_id");
-- Create "contest_problems" table
CREATE TABLE "contest_problems" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "display_order" bigint NOT NULL,
  "alias" character varying NOT NULL,
  "points" bigint NULL,
  "penalty_policy" jsonb NOT NULL,
  "visible_before_start" boolean NOT NULL DEFAULT false,
  "checker_key" character varying NOT NULL,
  "checker_version" character varying NOT NULL,
  "checker_config" jsonb NOT NULL,
  "allowed_runtime_keys" jsonb NOT NULL,
  "snapshot_checksum" bytea NOT NULL,
  "contest_id" uuid NOT NULL,
  "problem_id" uuid NOT NULL,
  "problem_revision_id" uuid NOT NULL,
  "testset_revision_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "contest_problem_alias" CHECK ((alias)::text ~ '^[A-Z][A-Z0-9]{0,15}$'::text),
  CONSTRAINT "contest_problem_json_bounds" CHECK ((jsonb_typeof(penalty_policy) = 'object'::text) AND (octet_length((penalty_policy)::text) <= 4096) AND (jsonb_typeof(checker_config) = 'object'::text) AND (octet_length((checker_config)::text) <= 4096) AND (jsonb_typeof(allowed_runtime_keys) = 'array'::text) AND ((jsonb_array_length(allowed_runtime_keys) >= 1) AND (jsonb_array_length(allowed_runtime_keys) <= 32))),
  CONSTRAINT "contest_problem_snapshot_checksum" CHECK ((octet_length(snapshot_checksum) = 32) AND (snapshot_checksum <> decode(repeat('00'::text, 32), 'hex'::text)))
);
-- Create index "contestproblem_contest_id_alias" to table: "contest_problems"
CREATE UNIQUE INDEX "contestproblem_contest_id_alias" ON "contest_problems" ("contest_id", "alias");
-- Create index "contestproblem_contest_id_display_order" to table: "contest_problems"
CREATE UNIQUE INDEX "contestproblem_contest_id_display_order" ON "contest_problems" ("contest_id", "display_order");
-- Create index "contestproblem_contest_id_problem_id" to table: "contest_problems"
CREATE UNIQUE INDEX "contestproblem_contest_id_problem_id" ON "contest_problems" ("contest_id", "problem_id");
-- Create index "contestproblem_id_contest_id" to table: "contest_problems"
CREATE UNIQUE INDEX "contestproblem_id_contest_id" ON "contest_problems" ("id", "contest_id");
-- Create index "contestproblem_id_contest_id_problem_id" to table: "contest_problems"
CREATE UNIQUE INDEX "contestproblem_id_contest_id_problem_id" ON "contest_problems" ("id", "contest_id", "problem_id");
-- Create index "contestproblem_problem_revision_id_problem_id" to table: "contest_problems"
CREATE INDEX "contestproblem_problem_revision_id_problem_id" ON "contest_problems" ("problem_revision_id", "problem_id");
-- Create index "contestproblem_testset_revision_id_problem_id" to table: "contest_problems"
CREATE INDEX "contestproblem_testset_revision_id_problem_id" ON "contest_problems" ("testset_revision_id", "problem_id");
-- Create "contest_rating_results" table
CREATE TABLE "contest_rating_results" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "prior_rating" bigint NOT NULL,
  "prior_rated_contest_count" bigint NOT NULL,
  "rank_position" bigint NOT NULL,
  "actual_rank" double precision NOT NULL,
  "expected_rank" double precision NOT NULL,
  "delta" bigint NOT NULL,
  "new_rating" bigint NOT NULL,
  "rating_run_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "contest_rating_result_math" CHECK (new_rating = GREATEST((0)::bigint, (prior_rating + delta))),
  CONSTRAINT "contest_rating_result_nonzero_ids" CHECK ((rating_run_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (user_id <> '00000000-0000-0000-0000-000000000000'::uuid))
);
-- Create index "contestratingresult_rating_run_id_user_id" to table: "contest_rating_results"
CREATE UNIQUE INDEX "contestratingresult_rating_run_id_user_id" ON "contest_rating_results" ("rating_run_id", "user_id");
-- Create index "contestratingresult_user_id_created_at_id" to table: "contest_rating_results"
CREATE INDEX "contestratingresult_user_id_created_at_id" ON "contest_rating_results" ("user_id", "created_at", "id");
-- Create "contest_rating_runs" table
CREATE TABLE "contest_rating_runs" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "rating_sequence" bigint NOT NULL,
  "run_version" bigint NOT NULL,
  "standings_projection_version" bigint NOT NULL,
  "algorithm_version" character varying NOT NULL,
  "status" character varying NOT NULL DEFAULT 'waiting',
  "skip_code" character varying NULL,
  "actor_id" uuid NULL,
  "reason" character varying NULL,
  "input_checksum" bytea NOT NULL,
  "participant_count" bigint NOT NULL,
  "failure_code" character varying NULL,
  "applied_at" timestamptz NULL,
  "contest_id" uuid NOT NULL,
  "rating_sequence_slot_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "contest_rating_run_applied_shape" CHECK ((((status)::text = ANY ((ARRAY['applied'::character varying, 'skipped'::character varying])::text[])) AND (applied_at IS NOT NULL) AND (failure_code IS NULL)) OR (((status)::text <> ALL ((ARRAY['applied'::character varying, 'skipped'::character varying])::text[])) AND (applied_at IS NULL))),
  CONSTRAINT "contest_rating_run_checksum" CHECK (octet_length(input_checksum) = 32),
  CONSTRAINT "contest_rating_run_failed_shape" CHECK ((((status)::text = 'failed'::text) AND (failure_code IS NOT NULL)) OR (((status)::text <> 'failed'::text) AND (failure_code IS NULL))),
  CONSTRAINT "contest_rating_run_nonzero_contest" CHECK ((contest_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (rating_sequence_slot_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "contest_rating_run_skip_shape" CHECK ((((status)::text = 'skipped'::text) AND (skip_code IS NOT NULL) AND (reason IS NOT NULL) AND (applied_at IS NOT NULL)) OR (((status)::text <> 'skipped'::text) AND (skip_code IS NULL)))
);
-- Create index "contestratingrun_contest_id_run_version" to table: "contest_rating_runs"
CREATE UNIQUE INDEX "contestratingrun_contest_id_run_version" ON "contest_rating_runs" ("contest_id", "run_version");
-- Create index "contestratingrun_contest_id_status_run_version" to table: "contest_rating_runs"
CREATE INDEX "contestratingrun_contest_id_status_run_version" ON "contest_rating_runs" ("contest_id", "status", "run_version");
-- Create index "contestratingrun_rating_sequence_run_version" to table: "contest_rating_runs"
CREATE UNIQUE INDEX "contestratingrun_rating_sequence_run_version" ON "contest_rating_runs" ("rating_sequence", "run_version");
-- Create index "contestratingrun_rating_sequence_slot_id_run_version" to table: "contest_rating_runs"
CREATE UNIQUE INDEX "contestratingrun_rating_sequence_slot_id_run_version" ON "contest_rating_runs" ("rating_sequence_slot_id", "run_version");
-- Create index "contestratingrun_status_created_at_id" to table: "contest_rating_runs"
CREATE INDEX "contestratingrun_status_created_at_id" ON "contest_rating_runs" ("status", "created_at", "id");
-- Create "contest_registration_transitions" table
CREATE TABLE "contest_registration_transitions" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "contest_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  "command_id" uuid NOT NULL,
  "command_hash" bytea NOT NULL,
  "registration_version" bigint NOT NULL,
  "official_submission_count" bigint NOT NULL,
  "mutation" character varying NOT NULL,
  "from_status" character varying NOT NULL,
  "to_status" character varying NOT NULL,
  "source" character varying NOT NULL,
  "actor_user_id" uuid NULL,
  "actor_session_id" uuid NULL,
  "reason" character varying NULL,
  "occurred_at" timestamptz NOT NULL,
  "registration_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "contest_registration_transition_graph" CHECK ((((mutation)::text = 'register'::text) AND ((from_status)::text = ANY ((ARRAY['absent'::character varying, 'withdrawn'::character varying])::text[])) AND ((to_status)::text = 'registered'::text)) OR (((mutation)::text = 'withdraw'::text) AND ((from_status)::text = 'registered'::text) AND ((to_status)::text = 'withdrawn'::text)) OR (((mutation)::text = 'ban'::text) AND ((from_status)::text = ANY ((ARRAY['absent'::character varying, 'registered'::character varying, 'withdrawn'::character varying])::text[])) AND ((to_status)::text = 'banned'::text))),
  CONSTRAINT "contest_registration_transition_hash" CHECK ((octet_length(command_hash) = 32) AND (command_hash <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "contest_registration_transition_reason" CHECK ((reason IS NULL) OR ((octet_length((reason)::text) >= 1) AND (octet_length((reason)::text) <= 1024)))
);
-- Create index "contestregistrationtransition_contest_id_occurred_at_id" to table: "contest_registration_transitions"
CREATE INDEX "contestregistrationtransition_contest_id_occurred_at_id" ON "contest_registration_transitions" ("contest_id", "occurred_at", "id");
-- Create index "contestregistrationtransition_contest_id_user_id_command_id" to table: "contest_registration_transitions"
CREATE UNIQUE INDEX "contestregistrationtransition_contest_id_user_id_command_id" ON "contest_registration_transitions" ("contest_id", "user_id", "command_id");
-- Create index "contestregistrationtransition_registration_id_registration_vers" to table: "contest_registration_transitions"
CREATE UNIQUE INDEX "contestregistrationtransition_registration_id_registration_vers" ON "contest_registration_transitions" ("registration_id", "registration_version");
-- Create "contest_registrations" table
CREATE TABLE "contest_registrations" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "status" character varying NOT NULL DEFAULT 'registered',
  "source" character varying NOT NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "official_submission_count" bigint NOT NULL DEFAULT 0,
  "registered_at" timestamptz NULL,
  "withdrawn_at" timestamptz NULL,
  "banned_at" timestamptz NULL,
  "reason" character varying NULL,
  "contest_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "contest_registration_positive_version" CHECK (version >= 1),
  CONSTRAINT "contest_registration_reason_bound" CHECK ((reason IS NULL) OR ((octet_length((reason)::text) >= 1) AND (octet_length((reason)::text) <= 1024))),
  CONSTRAINT "contest_registration_state_complete" CHECK ((((status)::text = 'registered'::text) AND (registered_at IS NOT NULL) AND (withdrawn_at IS NULL) AND (banned_at IS NULL)) OR (((status)::text = 'withdrawn'::text) AND (registered_at IS NOT NULL) AND (withdrawn_at IS NOT NULL) AND (banned_at IS NULL)) OR (((status)::text = 'banned'::text) AND (banned_at IS NOT NULL))),
  CONSTRAINT "contest_registration_submission_count" CHECK (official_submission_count >= 0),
  CONSTRAINT "contest_registration_withdrawal_has_no_submissions" CHECK (((status)::text <> 'withdrawn'::text) OR (official_submission_count = 0))
);
-- Create index "contestregistration_contest_id_status_id" to table: "contest_registrations"
CREATE INDEX "contestregistration_contest_id_status_id" ON "contest_registrations" ("contest_id", "status", "id");
-- Create index "contestregistration_contest_id_user_id" to table: "contest_registrations"
CREATE UNIQUE INDEX "contestregistration_contest_id_user_id" ON "contest_registrations" ("contest_id", "user_id");
-- Create index "contestregistration_id_contest_id_user_id" to table: "contest_registrations"
CREATE UNIQUE INDEX "contestregistration_id_contest_id_user_id" ON "contest_registrations" ("id", "contest_id", "user_id");
-- Create index "contestregistration_user_id" to table: "contest_registrations"
CREATE INDEX "contestregistration_user_id" ON "contest_registrations" ("user_id");
-- Create "contest_scoring_attempts" table
CREATE TABLE "contest_scoring_attempts" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "submitted_at" timestamptz NOT NULL,
  "submission_elapsed_ms" bigint NOT NULL,
  "contest_submission_sequence" bigint NOT NULL,
  "active_generation" bigint NOT NULL,
  "terminal_verdict" character varying NOT NULL,
  "judgement_version" bigint NOT NULL,
  "source_event_id" uuid NOT NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "contest_id" uuid NOT NULL,
  "contest_problem_id" uuid NOT NULL,
  "problem_id" uuid NOT NULL,
  "submission_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "contest_scoring_attempt_nonzero_ids" CHECK ((contest_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (user_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (problem_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (contest_problem_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (submission_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (source_event_id <> '00000000-0000-0000-0000-000000000000'::uuid))
);
-- Create index "contest_scoring_attempts_submission_id_key" to table: "contest_scoring_attempts"
CREATE UNIQUE INDEX "contest_scoring_attempts_submission_id_key" ON "contest_scoring_attempts" ("submission_id");
-- Create index "contestscoringattempt_contest_id_contest_submission_sequence" to table: "contest_scoring_attempts"
CREATE UNIQUE INDEX "contestscoringattempt_contest_id_contest_submission_sequence" ON "contest_scoring_attempts" ("contest_id", "contest_submission_sequence");
-- Create index "contestscoringattempt_contest_id_user_id_contest_problem_id_con" to table: "contest_scoring_attempts"
CREATE INDEX "contestscoringattempt_contest_id_user_id_contest_problem_id_con" ON "contest_scoring_attempts" ("contest_id", "user_id", "contest_problem_id", "contest_submission_sequence");
-- Create index "contestscoringattempt_contest_id_user_id_problem_id_contest_sub" to table: "contest_scoring_attempts"
CREATE INDEX "contestscoringattempt_contest_id_user_id_problem_id_contest_sub" ON "contest_scoring_attempts" ("contest_id", "user_id", "problem_id", "contest_submission_sequence");
-- Create index "contestscoringattempt_source_event_id" to table: "contest_scoring_attempts"
CREATE UNIQUE INDEX "contestscoringattempt_source_event_id" ON "contest_scoring_attempts" ("source_event_id");
-- Create index "contestscoringattempt_submission_id" to table: "contest_scoring_attempts"
CREATE UNIQUE INDEX "contestscoringattempt_submission_id" ON "contest_scoring_attempts" ("submission_id");
-- Create "contest_standings" table
CREATE TABLE "contest_standings" (
  "id" uuid NOT NULL,
  "solved_count" bigint NOT NULL DEFAULT 0,
  "penalty" bigint NOT NULL DEFAULT 0,
  "problem_results" jsonb NULL,
  "last_accepted_elapsed_ms" bigint NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "frozen_solved_count" bigint NOT NULL DEFAULT 0,
  "frozen_penalty" bigint NOT NULL DEFAULT 0,
  "frozen_problem_results" jsonb NULL,
  "frozen_pending_problem_attempts" jsonb NULL,
  "frozen_pending_attempts" bigint NOT NULL DEFAULT 0,
  "frozen_last_accepted_elapsed_ms" bigint NULL,
  "frozen_version" bigint NOT NULL DEFAULT 1,
  "contest_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "conteststanding_contest_id_frozen_solved_count_frozen_penalty_u" to table: "contest_standings"
CREATE INDEX "conteststanding_contest_id_frozen_solved_count_frozen_penalty_u" ON "contest_standings" ("contest_id", "frozen_solved_count", "frozen_penalty", "user_id");
-- Create index "conteststanding_contest_id_solved_count_penalty_last_accepted_e" to table: "contest_standings"
CREATE INDEX "conteststanding_contest_id_solved_count_penalty_last_accepted_e" ON "contest_standings" ("contest_id", "solved_count", "penalty", "last_accepted_elapsed_ms", "user_id");
-- Create index "conteststanding_contest_id_user_id" to table: "contest_standings"
CREATE UNIQUE INDEX "conteststanding_contest_id_user_id" ON "contest_standings" ("contest_id", "user_id");
-- Create "contest_transitions" table
CREATE TABLE "contest_transitions" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "command_id" uuid NOT NULL,
  "command_hash" bytea NOT NULL,
  "version" bigint NOT NULL,
  "from_state" character varying NOT NULL,
  "to_state" character varying NOT NULL,
  "actor_user_id" uuid NULL,
  "actor_session_id" uuid NULL,
  "reason" character varying NOT NULL,
  "occurred_at" timestamptz NOT NULL,
  "contest_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "contest_transition_command_hash" CHECK ((octet_length(command_hash) = 32) AND (command_hash <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "contest_transition_graph" CHECK ((((from_state)::text = 'draft'::text) AND ((to_state)::text = ANY ((ARRAY['upcoming'::character varying, 'cancelled'::character varying])::text[]))) OR (((from_state)::text = 'upcoming'::text) AND ((to_state)::text = ANY ((ARRAY['running'::character varying, 'cancelled'::character varying])::text[]))) OR (((from_state)::text = 'running'::text) AND ((to_state)::text = ANY ((ARRAY['ended'::character varying, 'cancelled'::character varying])::text[])))),
  CONSTRAINT "contest_transition_reason_bound" CHECK ((octet_length((reason)::text) >= 1) AND (octet_length((reason)::text) <= 1024))
);
-- Create index "contesttransition_contest_id_command_id" to table: "contest_transitions"
CREATE UNIQUE INDEX "contesttransition_contest_id_command_id" ON "contest_transitions" ("contest_id", "command_id");
-- Create index "contesttransition_contest_id_occurred_at_id" to table: "contest_transitions"
CREATE INDEX "contesttransition_contest_id_occurred_at_id" ON "contest_transitions" ("contest_id", "occurred_at", "id");
-- Create index "contesttransition_contest_id_version" to table: "contest_transitions"
CREATE UNIQUE INDEX "contesttransition_contest_id_version" ON "contest_transitions" ("contest_id", "version");
-- Create "contest_user_ratings" table
CREATE TABLE "contest_user_ratings" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "rating" bigint NOT NULL,
  "rated_contest_count" bigint NOT NULL,
  "applied_sequence" bigint NOT NULL,
  "revision" bigint NOT NULL,
  "projection_generation_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "contest_user_ratings_check" CHECK ((projection_generation_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (user_id <> '00000000-0000-0000-0000-000000000000'::uuid))
);
-- Create index "contestuserrating_projection_generation_id_rating_user_id" to table: "contest_user_ratings"
CREATE INDEX "contestuserrating_projection_generation_id_rating_user_id" ON "contest_user_ratings" ("projection_generation_id", "rating", "user_id");
-- Create index "contestuserrating_projection_generation_id_user_id" to table: "contest_user_ratings"
CREATE UNIQUE INDEX "contestuserrating_projection_generation_id_user_id" ON "contest_user_ratings" ("projection_generation_id", "user_id");
-- Create index "contestuserrating_user_id_applied_sequence" to table: "contest_user_ratings"
CREATE INDEX "contestuserrating_user_id_applied_sequence" ON "contest_user_ratings" ("user_id", "applied_sequence");
-- Create "contests" table
CREATE TABLE "contests" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "title" character varying NOT NULL,
  "description" text NULL,
  "start_time" timestamptz NOT NULL,
  "end_time" timestamptz NOT NULL,
  "status" character varying NOT NULL DEFAULT 'draft',
  "version" bigint NOT NULL DEFAULT 1,
  "registration_mode" character varying NOT NULL DEFAULT 'registered',
  "registration_opens_at" timestamptz NULL,
  "registration_closes_at" timestamptz NULL,
  "max_participants" bigint NULL,
  "next_submission_sequence" bigint NOT NULL DEFAULT 1,
  "scoring_policy" character varying NOT NULL DEFAULT 'icpc',
  "scoring_policy_version" bigint NOT NULL DEFAULT 1,
  "standings_projection_version" bigint NOT NULL DEFAULT 1,
  "freeze_standings_at" timestamptz NULL,
  "rated" boolean NOT NULL DEFAULT true,
  "rating_algorithm_version" character varying NOT NULL DEFAULT 'expected-rank-v1',
  "published_at" timestamptz NULL,
  "started_at" timestamptz NULL,
  "ended_at" timestamptz NULL,
  "cancelled_at" timestamptz NULL,
  "rating_completed_at" timestamptz NULL,
  "author_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "contest_freeze_window" CHECK ((freeze_standings_at IS NULL) OR ((freeze_standings_at >= start_time) AND (freeze_standings_at < end_time))),
  CONSTRAINT "contest_lifecycle_timestamps" CHECK ((((status)::text = 'draft'::text) AND (published_at IS NULL) AND (started_at IS NULL) AND (ended_at IS NULL) AND (cancelled_at IS NULL)) OR (((status)::text = 'upcoming'::text) AND (published_at IS NOT NULL) AND (started_at IS NULL) AND (ended_at IS NULL) AND (cancelled_at IS NULL)) OR (((status)::text = 'running'::text) AND (published_at IS NOT NULL) AND (started_at IS NOT NULL) AND (ended_at IS NULL) AND (cancelled_at IS NULL)) OR (((status)::text = 'ended'::text) AND (published_at IS NOT NULL) AND (started_at IS NOT NULL) AND (ended_at IS NOT NULL) AND (cancelled_at IS NULL)) OR (((status)::text = 'cancelled'::text) AND (cancelled_at IS NOT NULL) AND (ended_at IS NULL))),
  CONSTRAINT "contest_positive_capacity" CHECK ((max_participants IS NULL) OR (max_participants > 0)),
  CONSTRAINT "contest_positive_scoring_version" CHECK (scoring_policy_version >= 1),
  CONSTRAINT "contest_positive_standings_version" CHECK (standings_projection_version >= 1),
  CONSTRAINT "contest_positive_submission_sequence" CHECK (next_submission_sequence >= 1),
  CONSTRAINT "contest_positive_version" CHECK (version >= 1),
  CONSTRAINT "contest_rating_algorithm" CHECK ((rating_algorithm_version)::text = 'expected-rank-v1'::text),
  CONSTRAINT "contest_rating_completion" CHECK ((rating_completed_at IS NULL) OR (rated AND ((status)::text = 'ended'::text) AND (ended_at IS NOT NULL) AND (rating_completed_at >= ended_at))),
  CONSTRAINT "contest_registration_policy" CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'cancelled'::character varying])::text[])) OR ((((registration_mode)::text = 'registered'::text) AND (registration_opens_at IS NOT NULL) AND (registration_closes_at IS NOT NULL) AND (registration_opens_at <= registration_closes_at) AND (registration_closes_at <= start_time)) OR (((registration_mode)::text = 'open'::text) AND (registration_opens_at IS NULL) AND (registration_closes_at IS NULL)))),
  CONSTRAINT "contest_valid_window" CHECK (start_time < end_time)
);
-- Create index "contest_start_time" to table: "contests"
CREATE INDEX "contest_start_time" ON "contests" ("start_time");
-- Create index "contest_status" to table: "contests"
CREATE INDEX "contest_status" ON "contests" ("status");
-- Create index "contest_status_rated_rating_completed_at_end_time_id" to table: "contests"
CREATE INDEX "contest_status_rated_rating_completed_at_end_time_id" ON "contests" ("status", "rated", "rating_completed_at", "end_time", "id");
-- Create index "contest_status_rating_completed_at_end_time_id" to table: "contests"
CREATE INDEX "contest_status_rating_completed_at_end_time_id" ON "contests" ("status", "rating_completed_at", "end_time", "id");
-- Create index "contest_status_start_time_id" to table: "contests"
CREATE INDEX "contest_status_start_time_id" ON "contests" ("status", "start_time", "id");
-- Create "credentials" table
CREATE TABLE "credentials" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "type" character varying NOT NULL,
  "credential_data" jsonb NOT NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "active" boolean NOT NULL DEFAULT true,
  "last_used_at" timestamptz NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "credential_user_id_type" to table: "credentials"
CREATE UNIQUE INDEX "credential_user_id_type" ON "credentials" ("user_id", "type");
-- Create "cultivation_catalog_revisions" table
CREATE TABLE "cultivation_catalog_revisions" (
  "id" uuid NOT NULL,
  "revision" bigint NOT NULL,
  "algorithm_version" character varying NOT NULL,
  "catalog_checksum" bytea NOT NULL,
  "root_pool" jsonb NOT NULL,
  "talent_pool" jsonb NOT NULL,
  "maximum_offers" bigint NOT NULL,
  "offer_ttl_seconds" bigint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "cultivation_catalog_algorithm_bound" CHECK ((octet_length((algorithm_version)::text) >= 1) AND (octet_length((algorithm_version)::text) <= 64)),
  CONSTRAINT "cultivation_catalog_checksum_shape" CHECK ((octet_length(catalog_checksum) = 32) AND (catalog_checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "cultivation_catalog_pool_shape" CHECK ((jsonb_typeof(root_pool) = 'array'::text) AND (jsonb_array_length(root_pool) >= 1) AND (jsonb_typeof(talent_pool) = 'array'::text) AND (jsonb_array_length(talent_pool) >= 6))
);
-- Create index "cultivationcatalogrevision_catalog_checksum" to table: "cultivation_catalog_revisions"
CREATE UNIQUE INDEX "cultivationcatalogrevision_catalog_checksum" ON "cultivation_catalog_revisions" ("catalog_checksum");
-- Create index "cultivationcatalogrevision_revision" to table: "cultivation_catalog_revisions"
CREATE UNIQUE INDEX "cultivationcatalogrevision_revision" ON "cultivation_catalog_revisions" ("revision");
-- Create "cultivation_ranking_privacy_changes" table
CREATE TABLE "cultivation_ranking_privacy_changes" (
  "id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  "from_visibility" character varying NOT NULL,
  "to_visibility" character varying NOT NULL,
  "from_version" bigint NOT NULL,
  "to_version" bigint NOT NULL,
  "projection_revision" bigint NOT NULL,
  "request_checksum" bytea NOT NULL,
  "reason" character varying NOT NULL,
  "correlation_id" character varying NOT NULL,
  "occurred_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "cultivation_ranking_privacy_checksum" CHECK ((octet_length(request_checksum) = 32) AND (request_checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "cultivation_ranking_privacy_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (user_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "cultivation_ranking_privacy_transition" CHECK (((from_visibility)::text <> (to_visibility)::text) AND (to_version = (from_version + 1)))
);
-- Create index "cultivationrankingprivacychange_request_checksum" to table: "cultivation_ranking_privacy_changes"
CREATE INDEX "cultivationrankingprivacychange_request_checksum" ON "cultivation_ranking_privacy_changes" ("request_checksum");
-- Create index "cultivationrankingprivacychange_user_id_occurred_at_id" to table: "cultivation_ranking_privacy_changes"
CREATE INDEX "cultivationrankingprivacychange_user_id_occurred_at_id" ON "cultivation_ranking_privacy_changes" ("user_id", "occurred_at", "id");
-- Create index "cultivationrankingprivacychange_user_id_to_version" to table: "cultivation_ranking_privacy_changes"
CREATE UNIQUE INDEX "cultivationrankingprivacychange_user_id_to_version" ON "cultivation_ranking_privacy_changes" ("user_id", "to_version");
-- Create "cultivation_ranking_projections" table
CREATE TABLE "cultivation_ranking_projections" (
  "id" uuid NOT NULL,
  "public_id" uuid NOT NULL,
  "visibility" character varying NOT NULL DEFAULT 'public',
  "privacy_version" bigint NOT NULL DEFAULT 1,
  "projection_revision" bigint NOT NULL,
  "progression_source_version" bigint NOT NULL DEFAULT 0,
  "rating_source_version" bigint NOT NULL DEFAULT 0,
  "total_exp" bigint NOT NULL DEFAULT 0,
  "rating" bigint NOT NULL DEFAULT 1200,
  "progression_updated_at" timestamptz NULL,
  "rating_updated_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "cultivation_ranking_projection_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (user_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (public_id <> '00000000-0000-0000-0000-000000000000'::uuid))
);
-- Create index "cultivationrankingprojection_projection_revision" to table: "cultivation_ranking_projections"
CREATE INDEX "cultivationrankingprojection_projection_revision" ON "cultivation_ranking_projections" ("projection_revision");
-- Create index "cultivationrankingprojection_public_id" to table: "cultivation_ranking_projections"
CREATE UNIQUE INDEX "cultivationrankingprojection_public_id" ON "cultivation_ranking_projections" ("public_id");
-- Create index "cultivationrankingprojection_user_id" to table: "cultivation_ranking_projections"
CREATE UNIQUE INDEX "cultivationrankingprojection_user_id" ON "cultivation_ranking_projections" ("user_id");
-- Create index "cultivationrankingprojection_visibility_rating_rating_updated_a" to table: "cultivation_ranking_projections"
CREATE INDEX "cultivationrankingprojection_visibility_rating_rating_updated_a" ON "cultivation_ranking_projections" ("visibility", "rating", "rating_updated_at", "user_id");
-- Create index "cultivationrankingprojection_visibility_total_exp_progression_u" to table: "cultivation_ranking_projections"
CREATE INDEX "cultivationrankingprojection_visibility_total_exp_progression_u" ON "cultivation_ranking_projections" ("visibility", "total_exp", "progression_updated_at", "user_id");
-- Create "cultivation_ranking_states" table
CREATE TABLE "cultivation_ranking_states" (
  "id" uuid NOT NULL,
  "key" character varying NOT NULL,
  "revision" bigint NOT NULL DEFAULT 0,
  "last_reconciled_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "cultivation_ranking_state_identity" CHECK (id <> '00000000-0000-0000-0000-000000000000'::uuid),
  CONSTRAINT "cultivation_ranking_state_key" CHECK ((key)::text = 'global'::text)
);
-- Create index "cultivationrankingstate_key" to table: "cultivation_ranking_states"
CREATE UNIQUE INDEX "cultivationrankingstate_key" ON "cultivation_ranking_states" ("key");
-- Create "difficulties" table
CREATE TABLE "difficulties" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "name" character varying NOT NULL,
  "level" bigint NOT NULL,
  "exp_reward" bigint NOT NULL DEFAULT 0,
  "description" character varying NULL,
  "version" bigint NOT NULL DEFAULT 1,
  PRIMARY KEY ("id"),
  CONSTRAINT "difficulty_exp_reward_bound" CHECK ((exp_reward >= 0) AND (exp_reward <= 1000000000)),
  CONSTRAINT "difficulty_positive_version" CHECK (version >= 1)
);
-- Create index "difficulties_level_key" to table: "difficulties"
CREATE UNIQUE INDEX "difficulties_level_key" ON "difficulties" ("level");
-- Create index "difficulties_name_key" to table: "difficulties"
CREATE UNIQUE INDEX "difficulties_name_key" ON "difficulties" ("name");
-- Create "difficulty_revisions" table
CREATE TABLE "difficulty_revisions" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "revision" bigint NOT NULL,
  "name" character varying NOT NULL,
  "level" bigint NOT NULL,
  "exp_reward" bigint NOT NULL,
  "description" character varying NULL,
  "checksum" bytea NOT NULL,
  "difficulty_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "difficulty_revision_checksum_shape" CHECK ((octet_length(checksum) = 32) AND (checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "difficulty_revision_exp_reward_bound" CHECK ((exp_reward >= 0) AND (exp_reward <= 1000000000)),
  CONSTRAINT "difficulty_revision_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (difficulty_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "difficulty_revision_number_bound" CHECK ((revision >= 1) AND (revision <= 1000000000))
);
-- Create index "difficultyrevision_difficulty_id_created_at_id" to table: "difficulty_revisions"
CREATE INDEX "difficultyrevision_difficulty_id_created_at_id" ON "difficulty_revisions" ("difficulty_id", "created_at", "id");
-- Create index "difficultyrevision_difficulty_id_revision" to table: "difficulty_revisions"
CREATE UNIQUE INDEX "difficultyrevision_difficulty_id_revision" ON "difficulty_revisions" ("difficulty_id", "revision");
-- Create "element_revisions" table
CREATE TABLE "element_revisions" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "revision" bigint NOT NULL,
  "name" character varying NOT NULL,
  "code" character varying NOT NULL,
  "description" character varying NULL,
  "checksum" bytea NOT NULL,
  "element_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "element_revision_checksum_shape" CHECK ((octet_length(checksum) = 32) AND (checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "element_revision_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (element_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "element_revision_number_bound" CHECK ((revision >= 1) AND (revision <= 1000000000))
);
-- Create index "elementrevision_element_id_created_at_id" to table: "element_revisions"
CREATE INDEX "elementrevision_element_id_created_at_id" ON "element_revisions" ("element_id", "created_at", "id");
-- Create index "elementrevision_element_id_revision" to table: "element_revisions"
CREATE UNIQUE INDEX "elementrevision_element_id_revision" ON "element_revisions" ("element_id", "revision");
-- Create "elements" table
CREATE TABLE "elements" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "name" character varying NOT NULL,
  "code" character varying NOT NULL,
  "description" character varying NULL,
  "version" bigint NOT NULL DEFAULT 1,
  PRIMARY KEY ("id"),
  CONSTRAINT "element_positive_version" CHECK (version >= 1)
);
-- Create index "elements_code_key" to table: "elements"
CREATE UNIQUE INDEX "elements_code_key" ON "elements" ("code");
-- Create index "elements_name_key" to table: "elements"
CREATE UNIQUE INDEX "elements_name_key" ON "elements" ("name");
-- Create "federated_identities" table
CREATE TABLE "federated_identities" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "provider" character varying NOT NULL,
  "provider_subject" character varying NOT NULL,
  "display_name" character varying NULL,
  "avatar_url" character varying NULL,
  "last_authenticated_at" timestamptz NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "federatedidentity_provider_provider_subject" to table: "federated_identities"
CREATE UNIQUE INDEX "federatedidentity_provider_provider_subject" ON "federated_identities" ("provider", "provider_subject");
-- Create index "federatedidentity_user_id_provider" to table: "federated_identities"
CREATE UNIQUE INDEX "federatedidentity_user_id_provider" ON "federated_identities" ("user_id", "provider");
-- Create "judge_attempts" table
CREATE TABLE "judge_attempts" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "job_id" uuid NOT NULL,
  "submission_id" uuid NOT NULL,
  "generation" bigint NOT NULL,
  "judge_spec_id" uuid NOT NULL,
  "attempt_number" bigint NOT NULL,
  "lease_owner" character varying(128) NOT NULL,
  "lease_token_hash" bytea NOT NULL,
  "worker_id" uuid NOT NULL,
  "worker_boot_id" uuid NOT NULL,
  "origin_cid" character varying(128) NOT NULL,
  "state" character varying NOT NULL DEFAULT 'created',
  "failure_classification" character varying(64) NULL,
  "failure_code" character varying(64) NULL,
  "failure_summary" character varying NULL,
  "started_at" timestamptz NOT NULL,
  "heartbeat_at" timestamptz NULL,
  "finished_at" timestamptz NULL,
  "runtime_profile_version" character varying(128) NOT NULL,
  "compiler_version" character varying(128) NOT NULL,
  "runtime_bootstrap_version" character varying(128) NOT NULL,
  "supervisor_protocol_version" character varying(128) NOT NULL,
  "sandbox_image_digest" character varying(71) NOT NULL,
  "artifact_manifest_checksum" character varying(71) NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "judge_attempt_failure_metadata" CHECK ((((state)::text = ANY ((ARRAY['created'::character varying, 'preparing'::character varying, 'compiling'::character varying, 'running'::character varying, 'finalizing'::character varying, 'succeeded'::character varying])::text[])) AND (failure_classification IS NULL) AND (failure_code IS NULL) AND (failure_summary IS NULL)) OR (((state)::text = ANY ((ARRAY['infrastructure_failed'::character varying, 'lease_lost'::character varying, 'cancelled'::character varying])::text[])) AND (failure_classification IS NOT NULL) AND (failure_code IS NOT NULL) AND (failure_summary IS NOT NULL) AND ((octet_length((failure_classification)::text) >= 1) AND (octet_length((failure_classification)::text) <= 64)) AND ((octet_length((failure_code)::text) >= 1) AND (octet_length((failure_code)::text) <= 64)) AND ((octet_length((failure_summary)::text) >= 1) AND (octet_length((failure_summary)::text) <= 16384)) AND ((failure_classification)::text ~ '^[a-z][a-z0-9._-]{0,63}$'::text) AND ((failure_code)::text ~ '^[a-z][a-z0-9._-]{0,63}$'::text))),
  CONSTRAINT "judge_attempt_heartbeat_order" CHECK ((heartbeat_at IS NULL) OR (heartbeat_at >= started_at)),
  CONSTRAINT "judge_attempt_lease_hash_shape" CHECK ((octet_length(lease_token_hash) = 32) AND (lease_token_hash <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "judge_attempt_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (job_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (submission_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (judge_spec_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (worker_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (worker_boot_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "judge_attempt_number_bound" CHECK ((generation >= 1) AND ((attempt_number >= 1) AND (attempt_number <= 100))),
  CONSTRAINT "judge_attempt_pinned_digests" CHECK (((sandbox_image_digest)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((artifact_manifest_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text)),
  CONSTRAINT "judge_attempt_state_closed" CHECK ((state)::text = ANY ((ARRAY['created'::character varying, 'preparing'::character varying, 'compiling'::character varying, 'running'::character varying, 'finalizing'::character varying, 'succeeded'::character varying, 'infrastructure_failed'::character varying, 'lease_lost'::character varying, 'cancelled'::character varying])::text[])),
  CONSTRAINT "judge_attempt_terminal_timestamp" CHECK ((((state)::text = ANY ((ARRAY['created'::character varying, 'preparing'::character varying, 'compiling'::character varying, 'running'::character varying, 'finalizing'::character varying])::text[])) AND (finished_at IS NULL)) OR (((state)::text = ANY ((ARRAY['succeeded'::character varying, 'infrastructure_failed'::character varying, 'lease_lost'::character varying, 'cancelled'::character varying])::text[])) AND (finished_at IS NOT NULL) AND (finished_at >= started_at)))
);
-- Create index "judge_attempts_generation_identity" to table: "judge_attempts"
CREATE UNIQUE INDEX "judge_attempts_generation_identity" ON "judge_attempts" ("id", "submission_id", "generation", "judge_spec_id");
-- Create index "judge_attempts_job_identity" to table: "judge_attempts"
CREATE UNIQUE INDEX "judge_attempts_job_identity" ON "judge_attempts" ("id", "job_id", "submission_id", "generation", "judge_spec_id");
-- Create index "judgeattempt_id_judge_spec_id" to table: "judge_attempts"
CREATE UNIQUE INDEX "judgeattempt_id_judge_spec_id" ON "judge_attempts" ("id", "judge_spec_id");
-- Create index "judgeattempt_job_id_attempt_number" to table: "judge_attempts"
CREATE UNIQUE INDEX "judgeattempt_job_id_attempt_number" ON "judge_attempts" ("job_id", "attempt_number");
-- Create index "judgeattempt_job_id_state_attempt_number" to table: "judge_attempts"
CREATE INDEX "judgeattempt_job_id_state_attempt_number" ON "judge_attempts" ("job_id", "state", "attempt_number");
-- Create index "judgeattempt_lease_token_hash" to table: "judge_attempts"
CREATE UNIQUE INDEX "judgeattempt_lease_token_hash" ON "judge_attempts" ("lease_token_hash");
-- Create index "judgeattempt_submission_id_generation_attempt_number" to table: "judge_attempts"
CREATE INDEX "judgeattempt_submission_id_generation_attempt_number" ON "judge_attempts" ("submission_id", "generation", "attempt_number");
-- Create index "judgeattempt_worker_id_state_heartbeat_at" to table: "judge_attempts"
CREATE INDEX "judgeattempt_worker_id_state_heartbeat_at" ON "judge_attempts" ("worker_id", "state", "heartbeat_at");
-- Create index "judgeattempt_worker_id_worker_boot_id_attempt_number" to table: "judge_attempts"
CREATE INDEX "judgeattempt_worker_id_worker_boot_id_attempt_number" ON "judge_attempts" ("worker_id", "worker_boot_id", "attempt_number");
-- Create "judge_jobs" table
CREATE TABLE "judge_jobs" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "submission_id" uuid NOT NULL,
  "generation" bigint NOT NULL,
  "judge_spec_id" uuid NOT NULL,
  "lane" character varying NOT NULL,
  "base_priority" bigint NOT NULL DEFAULT 0,
  "state" character varying NOT NULL DEFAULT 'ready',
  "available_at" timestamptz NOT NULL,
  "attempts" bigint NOT NULL DEFAULT 0,
  "max_attempts" bigint NOT NULL DEFAULT 5,
  "lease_owner" character varying(128) NULL,
  "lease_token_hash" bytea NULL,
  "lease_until" timestamptz NULL,
  "heartbeat_at" timestamptz NULL,
  "last_error_code" character varying(64) NULL,
  "last_error_summary" character varying NULL,
  "started_at" timestamptz NULL,
  "completed_at" timestamptz NULL,
  "dead_at" timestamptz NULL,
  "cancelled_at" timestamptz NULL,
  "origin_cid" character varying(128) NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "judge_job_error_metadata_complete" CHECK (((last_error_code IS NULL) AND (last_error_summary IS NULL)) OR ((last_error_code IS NOT NULL) AND (last_error_summary IS NOT NULL) AND ((octet_length((last_error_code)::text) >= 1) AND (octet_length((last_error_code)::text) <= 64)) AND ((octet_length((last_error_summary)::text) >= 1) AND (octet_length((last_error_summary)::text) <= 16384)) AND ((last_error_code)::text ~ '^[a-z][a-z0-9._-]{0,63}$'::text))),
  CONSTRAINT "judge_job_generation_and_attempt_bounds" CHECK ((generation >= 1) AND ((attempts >= 0) AND (attempts <= max_attempts)) AND ((max_attempts >= 1) AND (max_attempts <= 100))),
  CONSTRAINT "judge_job_lease_complete" CHECK ((((state)::text = 'leased'::text) AND (lease_owner IS NOT NULL) AND (lease_token_hash IS NOT NULL) AND (lease_until IS NOT NULL) AND (heartbeat_at IS NOT NULL)) OR (((state)::text <> 'leased'::text) AND (lease_owner IS NULL) AND (lease_token_hash IS NULL) AND (lease_until IS NULL) AND (heartbeat_at IS NULL))),
  CONSTRAINT "judge_job_lease_hash_shape" CHECK ((lease_token_hash IS NULL) OR ((octet_length(lease_token_hash) = 32) AND (lease_token_hash <> decode(repeat('00'::text, 32), 'hex'::text)))),
  CONSTRAINT "judge_job_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (submission_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (judge_spec_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "judge_job_priority_bound" CHECK ((base_priority >= '-1000'::integer) AND (base_priority <= 1000)),
  CONSTRAINT "judge_job_started_before_terminal" CHECK (((started_at IS NULL) OR (started_at >= created_at)) AND ((completed_at IS NULL) OR ((started_at IS NOT NULL) AND (completed_at >= started_at))) AND ((dead_at IS NULL) OR (dead_at >= created_at)) AND ((cancelled_at IS NULL) OR (cancelled_at >= created_at))),
  CONSTRAINT "judge_job_state_closed" CHECK ((state)::text = ANY ((ARRAY['ready'::character varying, 'leased'::character varying, 'retry_wait'::character varying, 'done'::character varying, 'cancelled'::character varying, 'dead'::character varying])::text[])),
  CONSTRAINT "judge_job_state_timestamps" CHECK ((((state)::text = ANY ((ARRAY['ready'::character varying, 'leased'::character varying, 'retry_wait'::character varying])::text[])) AND (completed_at IS NULL) AND (dead_at IS NULL) AND (cancelled_at IS NULL)) OR (((state)::text = 'done'::text) AND (completed_at IS NOT NULL) AND (dead_at IS NULL) AND (cancelled_at IS NULL)) OR (((state)::text = 'dead'::text) AND (completed_at IS NULL) AND (dead_at IS NOT NULL) AND (cancelled_at IS NULL)) OR (((state)::text = 'cancelled'::text) AND (completed_at IS NULL) AND (dead_at IS NULL) AND (cancelled_at IS NOT NULL)))
);
-- Create index "judgejob_id_submission_id_generation_judge_spec_id" to table: "judge_jobs"
CREATE UNIQUE INDEX "judgejob_id_submission_id_generation_judge_spec_id" ON "judge_jobs" ("id", "submission_id", "generation", "judge_spec_id");
-- Create index "judgejob_judge_spec_id" to table: "judge_jobs"
CREATE UNIQUE INDEX "judgejob_judge_spec_id" ON "judge_jobs" ("judge_spec_id");
-- Create index "judgejob_lease_token_hash" to table: "judge_jobs"
CREATE UNIQUE INDEX "judgejob_lease_token_hash" ON "judge_jobs" ("lease_token_hash");
-- Create index "judgejob_state_lane_available_at_base_priority_created_at_id" to table: "judge_jobs"
CREATE INDEX "judgejob_state_lane_available_at_base_priority_created_at_id" ON "judge_jobs" ("state", "lane", "available_at", "base_priority", "created_at", "id");
-- Create index "judgejob_state_lease_until" to table: "judge_jobs"
CREATE INDEX "judgejob_state_lease_until" ON "judge_jobs" ("state", "lease_until");
-- Create index "judgejob_submission_id_created_at_id" to table: "judge_jobs"
CREATE INDEX "judgejob_submission_id_created_at_id" ON "judge_jobs" ("submission_id", "created_at", "id");
-- Create index "judgejob_submission_id_generation" to table: "judge_jobs"
CREATE UNIQUE INDEX "judgejob_submission_id_generation" ON "judge_jobs" ("submission_id", "generation");
-- Create "judge_specs" table
CREATE TABLE "judge_specs" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "submission_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  "problem_id" uuid NOT NULL,
  "generation" bigint NOT NULL,
  "problem_revision_id" uuid NOT NULL,
  "testset_revision_id" uuid NOT NULL,
  "language" character varying NOT NULL,
  "runtime_key" character varying(64) NOT NULL,
  "runtime_profile_version" character varying(128) NOT NULL,
  "compiler_version" character varying(128) NOT NULL,
  "runtime_bootstrap_version" character varying(128) NOT NULL,
  "sandbox_image_digest" character varying(71) NOT NULL,
  "compile_command_id" character varying(128) NOT NULL,
  "run_command_id" character varying(128) NOT NULL,
  "artifact_manifest_version" character varying(128) NOT NULL,
  "artifact_manifest_checksum" character varying(71) NOT NULL,
  "artifact_manifest" jsonb NOT NULL,
  "supervisor_protocol_version" character varying(128) NOT NULL,
  "checker_key" character varying(64) NOT NULL,
  "checker_version" character varying(64) NOT NULL,
  "checker_config" jsonb NOT NULL,
  "release_id" character varying(128) NOT NULL,
  "approved_release_manifest_checksum" character varying(71) NOT NULL,
  "checker_implementation_checksum" character varying(71) NOT NULL,
  "capability_profile_hash" character varying(71) NOT NULL,
  "reward_snapshot" jsonb NOT NULL,
  "reward_snapshot_checksum" bytea NOT NULL,
  "cpu_time_limit_ms" bigint NOT NULL,
  "wall_time_limit_ms" bigint NOT NULL,
  "memory_limit_kb" bigint NOT NULL,
  "output_limit_bytes" bigint NOT NULL,
  "process_limit" bigint NOT NULL,
  "compile_cpu_time_limit_ms" bigint NOT NULL,
  "compile_wall_time_limit_ms" bigint NOT NULL,
  "total_attempt_wall_time_limit_ms" bigint NOT NULL,
  "source_sha256" bytea NOT NULL,
  "judge_spec_protocol" character varying(128) NOT NULL,
  "spec_checksum" bytea NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "judge_spec_checksum_shapes" CHECK ((octet_length(source_sha256) = 32) AND (source_sha256 <> decode(repeat('00'::text, 32), 'hex'::text)) AND (octet_length(reward_snapshot_checksum) = 32) AND (reward_snapshot_checksum <> decode(repeat('00'::text, 32), 'hex'::text)) AND (octet_length(spec_checksum) = 32) AND (spec_checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "judge_spec_generation_bound" CHECK (generation >= 1),
  CONSTRAINT "judge_spec_identifiers" CHECK (((runtime_key)::text ~ '^[a-z0-9][a-z0-9._-]{0,63}$'::text) AND ((runtime_profile_version)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((compile_command_id)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((run_command_id)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((artifact_manifest_version)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((supervisor_protocol_version)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((checker_key)::text ~ '^[a-z0-9][a-z0-9._-]{0,63}$'::text) AND ((checker_version)::text ~ '^[a-z0-9][a-z0-9._-]{0,63}$'::text) AND ((release_id)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((judge_spec_protocol)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text)),
  CONSTRAINT "judge_spec_json_bounds" CHECK ((jsonb_typeof(checker_config) = 'object'::text) AND (octet_length((checker_config)::text) <= 4096) AND (jsonb_typeof(reward_snapshot) = 'object'::text) AND (octet_length((reward_snapshot)::text) <= 65536) AND (jsonb_typeof(artifact_manifest) = 'array'::text) AND (jsonb_array_length(artifact_manifest) > 0) AND (octet_length((artifact_manifest)::text) <= 32768)),
  CONSTRAINT "judge_spec_limit_bounds" CHECK (((cpu_time_limit_ms >= 10) AND (cpu_time_limit_ms <= 30000)) AND ((wall_time_limit_ms >= 10) AND (wall_time_limit_ms <= 60000)) AND (wall_time_limit_ms >= cpu_time_limit_ms) AND ((memory_limit_kb >= 16384) AND (memory_limit_kb <= 2097152)) AND ((output_limit_bytes >= 1) AND (output_limit_bytes <= 1048576)) AND ((process_limit >= 1) AND (process_limit <= 64)) AND ((compile_cpu_time_limit_ms >= 1) AND (compile_cpu_time_limit_ms <= 600000)) AND ((compile_wall_time_limit_ms >= compile_cpu_time_limit_ms) AND (compile_wall_time_limit_ms <= 600000)) AND ((total_attempt_wall_time_limit_ms >= compile_wall_time_limit_ms) AND (total_attempt_wall_time_limit_ms <= 86400000))),
  CONSTRAINT "judge_spec_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (submission_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (user_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (problem_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (problem_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (testset_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "judge_spec_pinned_digests" CHECK (((sandbox_image_digest)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((artifact_manifest_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((approved_release_manifest_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((checker_implementation_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((capability_profile_hash)::text ~ '^sha256:[0-9a-f]{64}$'::text))
);
-- Create index "judge_specs_attempt_provenance_identity" to table: "judge_specs"
CREATE UNIQUE INDEX "judge_specs_attempt_provenance_identity" ON "judge_specs" ("id", "runtime_profile_version", "compiler_version", "runtime_bootstrap_version", "supervisor_protocol_version", "sandbox_image_digest", "artifact_manifest_checksum");
-- Create index "judge_specs_judgement_identity" to table: "judge_specs"
CREATE UNIQUE INDEX "judge_specs_judgement_identity" ON "judge_specs" ("id", "submission_id", "generation", "testset_revision_id", "problem_revision_id", "spec_checksum", "reward_snapshot_checksum", "runtime_profile_version", "compiler_version", "runtime_bootstrap_version", "supervisor_protocol_version", "sandbox_image_digest", "artifact_manifest_checksum", "release_id", "approved_release_manifest_checksum", "checker_implementation_checksum");
-- Create index "judgespec_capability_profile_hash_runtime_key" to table: "judge_specs"
CREATE INDEX "judgespec_capability_profile_hash_runtime_key" ON "judge_specs" ("capability_profile_hash", "runtime_key");
-- Create index "judgespec_id_submission_id_generation" to table: "judge_specs"
CREATE UNIQUE INDEX "judgespec_id_submission_id_generation" ON "judge_specs" ("id", "submission_id", "generation");
-- Create index "judgespec_id_submission_id_generation_testset_revision_id" to table: "judge_specs"
CREATE UNIQUE INDEX "judgespec_id_submission_id_generation_testset_revision_id" ON "judge_specs" ("id", "submission_id", "generation", "testset_revision_id");
-- Create index "judgespec_id_testset_revision_id" to table: "judge_specs"
CREATE UNIQUE INDEX "judgespec_id_testset_revision_id" ON "judge_specs" ("id", "testset_revision_id");
-- Create index "judgespec_problem_revision_id_testset_revision_id" to table: "judge_specs"
CREATE INDEX "judgespec_problem_revision_id_testset_revision_id" ON "judge_specs" ("problem_revision_id", "testset_revision_id");
-- Create index "judgespec_spec_checksum" to table: "judge_specs"
CREATE INDEX "judgespec_spec_checksum" ON "judge_specs" ("spec_checksum");
-- Create index "judgespec_submission_id_generation" to table: "judge_specs"
CREATE UNIQUE INDEX "judgespec_submission_id_generation" ON "judge_specs" ("submission_id", "generation");
-- Create index "judgespec_submission_id_user_id_problem_id_generation" to table: "judge_specs"
CREATE INDEX "judgespec_submission_id_user_id_problem_id_generation" ON "judge_specs" ("submission_id", "user_id", "problem_id", "generation");
-- Create "judge_test_results" table
CREATE TABLE "judge_test_results" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "attempt_id" uuid NOT NULL,
  "judge_spec_id" uuid NOT NULL,
  "testset_revision_id" uuid NOT NULL,
  "group_id" uuid NOT NULL,
  "group_index" bigint NOT NULL,
  "group_stop_on_failure" boolean NOT NULL,
  "testcase_revision_id" uuid NOT NULL,
  "case_order" bigint NOT NULL,
  "execution_order" bigint NOT NULL,
  "test_kind" character varying NOT NULL,
  "verdict" character varying NOT NULL,
  "time_ms" bigint NOT NULL,
  "memory_kb" bigint NOT NULL,
  "exit_code" bigint NULL,
  "terminating_signal" bigint NULL,
  "stdout_bytes" bigint NOT NULL,
  "stderr_bytes" bigint NOT NULL,
  "safe_diagnostic" character varying NULL,
  "checker_metadata" jsonb NOT NULL,
  "execution_started_at" timestamptz NOT NULL,
  "execution_completed_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "judge_test_result_checker_metadata_bound" CHECK ((jsonb_typeof(checker_metadata) = 'object'::text) AND (octet_length((checker_metadata)::text) <= 4096)),
  CONSTRAINT "judge_test_result_execution_order" CHECK (execution_completed_at >= execution_started_at),
  CONSTRAINT "judge_test_result_exit_complete" CHECK ((exit_code IS NULL) OR (terminating_signal IS NULL)),
  CONSTRAINT "judge_test_result_index_bounds" CHECK (((group_index >= 0) AND (group_index <= 63)) AND ((case_order >= 0) AND (case_order <= 255)) AND ((execution_order >= 0) AND (execution_order <= 255))),
  CONSTRAINT "judge_test_result_kind_closed" CHECK ((test_kind)::text = ANY ((ARRAY['sample'::character varying, 'hidden'::character varying])::text[])),
  CONSTRAINT "judge_test_result_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (attempt_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (judge_spec_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (testset_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (group_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (testcase_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "judge_test_result_resource_bounds" CHECK (((time_ms >= 0) AND (time_ms <= 60000)) AND ((memory_kb >= 0) AND (memory_kb <= 2097152)) AND ((stdout_bytes >= 0) AND (stdout_bytes <= 1048576)) AND ((stderr_bytes >= 0) AND (stderr_bytes <= 1048576))),
  CONSTRAINT "judge_test_result_safe_diagnostic" CHECK ((safe_diagnostic IS NULL) OR (((test_kind)::text = 'sample'::text) AND ((octet_length((safe_diagnostic)::text) >= 1) AND (octet_length((safe_diagnostic)::text) <= 16384)))),
  CONSTRAINT "judge_test_result_verdict_closed" CHECK ((verdict)::text = ANY ((ARRAY['accepted'::character varying, 'wrong_answer'::character varying, 'runtime_error'::character varying, 'time_limit_exceeded'::character varying, 'memory_limit_exceeded'::character varying, 'process_limit_exceeded'::character varying, 'output_limit_exceeded'::character varying])::text[]))
);
-- Create index "judgetestresult_attempt_id_execution_order" to table: "judge_test_results"
CREATE UNIQUE INDEX "judgetestresult_attempt_id_execution_order" ON "judge_test_results" ("attempt_id", "execution_order");
-- Create index "judgetestresult_attempt_id_group_index_case_order" to table: "judge_test_results"
CREATE UNIQUE INDEX "judgetestresult_attempt_id_group_index_case_order" ON "judge_test_results" ("attempt_id", "group_index", "case_order");
-- Create index "judgetestresult_attempt_id_testcase_revision_id" to table: "judge_test_results"
CREATE UNIQUE INDEX "judgetestresult_attempt_id_testcase_revision_id" ON "judge_test_results" ("attempt_id", "testcase_revision_id");
-- Create index "judgetestresult_judge_spec_id_testset_revision_id_group_index_c" to table: "judge_test_results"
CREATE INDEX "judgetestresult_judge_spec_id_testset_revision_id_group_index_c" ON "judge_test_results" ("judge_spec_id", "testset_revision_id", "group_index", "case_order");
-- Create index "judgetestresult_testcase_revision_id" to table: "judge_test_results"
CREATE INDEX "judgetestresult_testcase_revision_id" ON "judge_test_results" ("testcase_revision_id");
-- Create "judge_worker_capabilities" table
CREATE TABLE "judge_worker_capabilities" (
  "id" uuid NOT NULL,
  "boot_id" uuid NOT NULL,
  "judge_spec_protocol" character varying(128) NOT NULL,
  "runtime_key" character varying(64) NOT NULL,
  "runtime_profile_version" character varying(128) NOT NULL,
  "compiler_version" character varying(128) NOT NULL,
  "runtime_bootstrap_version" character varying(128) NOT NULL,
  "compile_command_id" character varying(128) NOT NULL,
  "run_command_id" character varying(128) NOT NULL,
  "image_digest" character varying(71) NOT NULL,
  "manifest_version" character varying(128) NOT NULL,
  "manifest_checksum" character varying(71) NOT NULL,
  "supervisor_protocol" character varying(128) NOT NULL,
  "checker_key" character varying(64) NOT NULL,
  "checker_version" character varying(64) NOT NULL,
  "capability_profile_hash" character varying(71) NOT NULL,
  "smoke_checksum" character varying(71) NOT NULL,
  "checker_implementation_checksum" character varying(71) NOT NULL,
  "golden_checksum" character varying(71) NOT NULL,
  "probed_at" timestamptz NOT NULL,
  "passing" boolean NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "worker_instance_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "judge_worker_capability_identifiers" CHECK (((judge_spec_protocol)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((runtime_key)::text ~ '^[a-z0-9][a-z0-9._-]{0,63}$'::text) AND ((runtime_profile_version)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((compiler_version)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((runtime_bootstrap_version)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((compile_command_id)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((run_command_id)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((manifest_version)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((supervisor_protocol)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text) AND ((checker_key)::text ~ '^[a-z0-9][a-z0-9._-]{0,63}$'::text) AND ((checker_version)::text ~ '^[a-z0-9][a-z0-9._-]{0,63}$'::text)),
  CONSTRAINT "judge_worker_capability_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (worker_instance_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (boot_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "judge_worker_capability_passing_only" CHECK (passing),
  CONSTRAINT "judge_worker_capability_pinned_digests" CHECK (((image_digest)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((manifest_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((capability_profile_hash)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((smoke_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((checker_implementation_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((golden_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text))
);
-- Create index "judge_worker_capabilities_activation_proof_identity" to table: "judge_worker_capabilities"
CREATE UNIQUE INDEX "judge_worker_capabilities_activation_proof_identity" ON "judge_worker_capabilities" ("id", "worker_instance_id", "boot_id", "judge_spec_protocol", "runtime_key", "runtime_profile_version", "compiler_version", "runtime_bootstrap_version", "compile_command_id", "run_command_id", "image_digest", "manifest_version", "manifest_checksum", "supervisor_protocol", "checker_key", "checker_version", "capability_profile_hash", "checker_implementation_checksum", "smoke_checksum", "golden_checksum");
-- Create index "judgeworkercapability_capability_profile_hash_probed_at" to table: "judge_worker_capabilities"
CREATE INDEX "judgeworkercapability_capability_profile_hash_probed_at" ON "judge_worker_capabilities" ("capability_profile_hash", "probed_at");
-- Create index "judgeworkercapability_id_worker_instance_id_boot_id" to table: "judge_worker_capabilities"
CREATE UNIQUE INDEX "judgeworkercapability_id_worker_instance_id_boot_id" ON "judge_worker_capabilities" ("id", "worker_instance_id", "boot_id");
-- Create index "judgeworkercapability_worker_instance_id_boot_id_capability_pro" to table: "judge_worker_capabilities"
CREATE UNIQUE INDEX "judgeworkercapability_worker_instance_id_boot_id_capability_pro" ON "judge_worker_capabilities" ("worker_instance_id", "boot_id", "capability_profile_hash");
-- Create index "judgeworkercapability_worker_instance_id_probed_at" to table: "judge_worker_capabilities"
CREATE INDEX "judgeworkercapability_worker_instance_id_probed_at" ON "judge_worker_capabilities" ("worker_instance_id", "probed_at");
-- Create "judge_worker_instances" table
CREATE TABLE "judge_worker_instances" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "worker_key" character varying(128) NOT NULL,
  "boot_id" uuid NOT NULL,
  "boot_started_at" timestamptz NOT NULL,
  "heartbeat_at" timestamptz NOT NULL,
  "claims_enabled" boolean NOT NULL DEFAULT false,
  "total_capacity" bigint NOT NULL DEFAULT 1,
  "free_capacity" bigint NOT NULL DEFAULT 0,
  PRIMARY KEY ("id"),
  CONSTRAINT "judge_worker_instance_capacity_bound" CHECK ((total_capacity >= 1) AND ((free_capacity >= 0) AND (free_capacity <= total_capacity))),
  CONSTRAINT "judge_worker_instance_heartbeat_order" CHECK (heartbeat_at >= boot_started_at),
  CONSTRAINT "judge_worker_instance_key_shape" CHECK ((worker_key)::text ~ '^[a-z0-9][a-z0-9._-]{0,127}$'::text),
  CONSTRAINT "judge_worker_instance_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (boot_id <> '00000000-0000-0000-0000-000000000000'::uuid))
);
-- Create index "judgeworkerinstance_boot_id" to table: "judge_worker_instances"
CREATE UNIQUE INDEX "judgeworkerinstance_boot_id" ON "judge_worker_instances" ("boot_id");
-- Create index "judgeworkerinstance_claims_enabled_heartbeat_at" to table: "judge_worker_instances"
CREATE INDEX "judgeworkerinstance_claims_enabled_heartbeat_at" ON "judge_worker_instances" ("claims_enabled", "heartbeat_at");
-- Create index "judgeworkerinstance_id_boot_id" to table: "judge_worker_instances"
CREATE UNIQUE INDEX "judgeworkerinstance_id_boot_id" ON "judge_worker_instances" ("id", "boot_id");
-- Create index "judgeworkerinstance_worker_key_boot_id" to table: "judge_worker_instances"
CREATE UNIQUE INDEX "judgeworkerinstance_worker_key_boot_id" ON "judge_worker_instances" ("worker_key", "boot_id");
-- Create "levels" table
CREATE TABLE "levels" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "name" character varying NOT NULL,
  "min_exp" bigint NOT NULL DEFAULT 0,
  "description" character varying NULL,
  PRIMARY KEY ("id")
);
-- Create index "levels_name_key" to table: "levels"
CREATE UNIQUE INDEX "levels_name_key" ON "levels" ("name");
-- Create "material_categories" table
CREATE TABLE "material_categories" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "name" character varying NOT NULL,
  "description" character varying NULL,
  PRIMARY KEY ("id")
);
-- Create index "material_categories_name_key" to table: "material_categories"
CREATE UNIQUE INDEX "material_categories_name_key" ON "material_categories" ("name");
-- Create "material_revision_tags" table
CREATE TABLE "material_revision_tags" (
  "id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY,
  "display_order" bigint NOT NULL,
  "revision_id" uuid NOT NULL,
  "tag_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "materialrevisiontag_revision_id_display_order" to table: "material_revision_tags"
CREATE UNIQUE INDEX "materialrevisiontag_revision_id_display_order" ON "material_revision_tags" ("revision_id", "display_order");
-- Create index "materialrevisiontag_revision_id_tag_id" to table: "material_revision_tags"
CREATE UNIQUE INDEX "materialrevisiontag_revision_id_tag_id" ON "material_revision_tags" ("revision_id", "tag_id");
-- Create index "materialrevisiontag_tag_id_revision_id" to table: "material_revision_tags"
CREATE INDEX "materialrevisiontag_tag_id_revision_id" ON "material_revision_tags" ("tag_id", "revision_id");
-- Create "material_revisions" table
CREATE TABLE "material_revisions" (
  "id" uuid NOT NULL,
  "revision_number" bigint NOT NULL,
  "title" character varying NOT NULL,
  "description" character varying NOT NULL,
  "content_markdown" character varying NOT NULL,
  "estimated_read_minutes" bigint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "change_summary" character varying NOT NULL,
  "normalized_source_checksum" bytea NOT NULL,
  "render_source_id" uuid NOT NULL,
  "render_source_kind" character varying NOT NULL,
  "render_artifact_id" uuid NOT NULL,
  "difficulty_id" uuid NOT NULL,
  "material_id" uuid NOT NULL,
  "category_id" uuid NOT NULL,
  "author_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "material_revision_checksum_shape" CHECK ((octet_length(normalized_source_checksum) = 32) AND (normalized_source_checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "material_revision_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (material_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (category_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (difficulty_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (author_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (render_source_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (render_artifact_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "material_revision_source_kind" CHECK ((render_source_kind)::text = 'material_body'::text),
  CONSTRAINT "material_revision_text_bounds" CHECK (((octet_length((title)::text) >= 1) AND (octet_length((title)::text) <= 255)) AND ((octet_length((description)::text) >= 1) AND (octet_length((description)::text) <= 1000)) AND ((octet_length((content_markdown)::text) >= 1) AND (octet_length((content_markdown)::text) <= 1048576)) AND ((octet_length((change_summary)::text) >= 1) AND (octet_length((change_summary)::text) <= 500)))
);
-- Create index "materialrevision_category_id_created_at_id" to table: "material_revisions"
CREATE INDEX "materialrevision_category_id_created_at_id" ON "material_revisions" ("category_id", "created_at", "id");
-- Create index "materialrevision_difficulty_id_created_at_id" to table: "material_revisions"
CREATE INDEX "materialrevision_difficulty_id_created_at_id" ON "material_revisions" ("difficulty_id", "created_at", "id");
-- Create index "materialrevision_id_material_id" to table: "material_revisions"
CREATE UNIQUE INDEX "materialrevision_id_material_id" ON "material_revisions" ("id", "material_id");
-- Create index "materialrevision_id_render_source_id_render_source_kind_normali" to table: "material_revisions"
CREATE UNIQUE INDEX "materialrevision_id_render_source_id_render_source_kind_normali" ON "material_revisions" ("id", "render_source_id", "render_source_kind", "normalized_source_checksum");
-- Create index "materialrevision_material_id_revision_number" to table: "material_revisions"
CREATE UNIQUE INDEX "materialrevision_material_id_revision_number" ON "material_revisions" ("material_id", "revision_number");
-- Create index "materialrevision_render_artifact_id" to table: "material_revisions"
CREATE UNIQUE INDEX "materialrevision_render_artifact_id" ON "material_revisions" ("render_artifact_id");
-- Create index "materialrevision_render_artifact_id_render_source_id_render_sou" to table: "material_revisions"
CREATE UNIQUE INDEX "materialrevision_render_artifact_id_render_source_id_render_sou" ON "material_revisions" ("render_artifact_id", "render_source_id", "render_source_kind", "normalized_source_checksum");
-- Create index "materialrevision_render_source_id" to table: "material_revisions"
CREATE UNIQUE INDEX "materialrevision_render_source_id" ON "material_revisions" ("render_source_id");
-- Create "material_slugs" table
CREATE TABLE "material_slugs" (
  "id" uuid NOT NULL,
  "slug" character varying NOT NULL,
  "state" character varying NOT NULL DEFAULT 'current',
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "retired_at" timestamptz NULL,
  "material_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "material_slug_normalized" CHECK (((octet_length((slug)::text) >= 1) AND (octet_length((slug)::text) <= 160)) AND ((slug)::text = lower((slug)::text)) AND ((slug)::text ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'::text)),
  CONSTRAINT "material_slug_state_shape" CHECK ((((state)::text = 'current'::text) AND (retired_at IS NULL)) OR (((state)::text = 'retired'::text) AND (retired_at IS NOT NULL)))
);
-- Create index "materialslug_id_material_id" to table: "material_slugs"
CREATE UNIQUE INDEX "materialslug_id_material_id" ON "material_slugs" ("id", "material_id");
-- Create index "materialslug_id_material_id_state" to table: "material_slugs"
CREATE UNIQUE INDEX "materialslug_id_material_id_state" ON "material_slugs" ("id", "material_id", "state");
-- Create index "materialslug_material_id" to table: "material_slugs"
CREATE UNIQUE INDEX "materialslug_material_id" ON "material_slugs" ("material_id") WHERE ((state)::text = 'current'::text);
-- Create index "materialslug_material_id_state" to table: "material_slugs"
CREATE INDEX "materialslug_material_id_state" ON "material_slugs" ("material_id", "state");
-- Create index "materialslug_slug" to table: "material_slugs"
CREATE UNIQUE INDEX "materialslug_slug" ON "material_slugs" ("slug");
-- Create "material_tags" table
CREATE TABLE "material_tags" (
  "material_id" uuid NOT NULL,
  "tag_id" uuid NOT NULL,
  PRIMARY KEY ("material_id", "tag_id")
);
-- Create "material_view_dailies" table
CREATE TABLE "material_view_dailies" (
  "id" uuid NOT NULL,
  "view_day" date NOT NULL,
  "viewer_fingerprint" bytea NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "projected_at" timestamptz NULL,
  "material_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "material_view_daily_fingerprint_shape" CHECK ((octet_length(viewer_fingerprint) = 32) AND (viewer_fingerprint <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "material_view_daily_projection_time" CHECK ((projected_at IS NULL) OR (projected_at >= created_at))
);
-- Create index "materialviewdaily_material_id_view_day_viewer_fingerprint" to table: "material_view_dailies"
CREATE UNIQUE INDEX "materialviewdaily_material_id_view_day_viewer_fingerprint" ON "material_view_dailies" ("material_id", "view_day", "viewer_fingerprint");
-- Create index "materialviewdaily_projected_at_view_day_id" to table: "material_view_dailies"
CREATE INDEX "materialviewdaily_projected_at_view_day_id" ON "material_view_dailies" ("projected_at", "view_day", "id");
-- Create index "materialviewdaily_view_day_id" to table: "material_view_dailies"
CREATE INDEX "materialviewdaily_view_day_id" ON "material_view_dailies" ("view_day", "id");
-- Create "materials" table
CREATE TABLE "materials" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "title" character varying NULL,
  "description" character varying NULL,
  "content" text NULL,
  "group_id" bigint NULL,
  "estimated_read_time" bigint NOT NULL DEFAULT 0,
  "current_slug_id" uuid NULL,
  "current_slug_state" character varying NOT NULL DEFAULT 'current',
  "status" character varying NOT NULL DEFAULT 'draft',
  "visibility" character varying NOT NULL DEFAULT 'public',
  "current_draft_revision_id" uuid NULL,
  "published_revision_id" uuid NULL,
  "published_render_source_id" uuid NULL,
  "published_render_artifact_id" uuid NULL,
  "published_render_source_kind" character varying NULL,
  "published_source_checksum" bytea NULL,
  "published_at" timestamptz NULL,
  "archived_at" timestamptz NULL,
  "view_count" bigint NOT NULL DEFAULT 0,
  "version" bigint NOT NULL DEFAULT 1,
  "difficulty_id" uuid NULL,
  "category_id" uuid NULL,
  "author_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "material_lifecycle_shape" CHECK ((((status)::text = 'archived'::text) AND (archived_at IS NOT NULL)) OR (((status)::text <> 'archived'::text) AND (archived_at IS NULL))),
  CONSTRAINT "material_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (author_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (current_slug_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "material_positive_version" CHECK ((version >= 1) AND (version <= '9223372036854775807'::bigint)),
  CONSTRAINT "material_publication_pointer_shape" CHECK (((published_revision_id IS NULL) AND (published_render_source_id IS NULL) AND (published_render_artifact_id IS NULL) AND (published_render_source_kind IS NULL) AND (published_source_checksum IS NULL) AND (published_at IS NULL)) OR ((published_revision_id IS NOT NULL) AND (published_render_source_id IS NOT NULL) AND (published_render_artifact_id IS NOT NULL) AND ((published_render_source_kind)::text = 'material_body'::text) AND (published_source_checksum IS NOT NULL) AND (octet_length(published_source_checksum) = 32) AND (published_at IS NOT NULL)))
);
-- Create index "material_current_slug_id_id_current_slug_state" to table: "materials"
CREATE UNIQUE INDEX "material_current_slug_id_id_current_slug_state" ON "materials" ("current_slug_id", "id", "current_slug_state");
-- Create index "material_published_at_id" to table: "materials"
CREATE INDEX "material_published_at_id" ON "materials" ("published_at", "id");
-- Create index "material_published_revision_id_id" to table: "materials"
CREATE UNIQUE INDEX "material_published_revision_id_id" ON "materials" ("published_revision_id", "id");
-- Create index "material_status" to table: "materials"
CREATE INDEX "material_status" ON "materials" ("status");
-- Create index "material_view_count_id" to table: "materials"
CREATE INDEX "material_view_count_id" ON "materials" ("view_count", "id");
-- Create "notification_campaign_confirmations" table
CREATE TABLE "notification_campaign_confirmations" (
  "id" uuid NOT NULL,
  "nonce_hash" bytea NOT NULL,
  "actor_id" uuid NOT NULL,
  "session_id" uuid NOT NULL,
  "campaign_revision_id" uuid NOT NULL,
  "render_source_id" uuid NOT NULL,
  "render_source_kind" character varying NOT NULL,
  "normalized_source_checksum" bytea NOT NULL,
  "render_artifact_id" uuid NOT NULL,
  "audience_fingerprint" bytea NOT NULL,
  "expected_campaign_version" bigint NOT NULL,
  "scheduled_for" timestamptz NULL,
  "dry_run_estimated_at" timestamptz NOT NULL,
  "dry_run_recipient_count" bigint NOT NULL,
  "issued_at" timestamptz NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "consumed_at" timestamptz NULL,
  "consumed_by_idempotency_key" uuid NULL,
  "schedule_request_hash" bytea NULL,
  "operation_id" uuid NULL,
  "campaign_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "notification_campaign_confirmation_audience_fingerprint_size" CHECK (octet_length(audience_fingerprint) = 32),
  CONSTRAINT "notification_campaign_confirmation_consumption_shape" CHECK (((consumed_at IS NULL) AND (consumed_by_idempotency_key IS NULL) AND (schedule_request_hash IS NULL) AND (operation_id IS NULL)) OR ((consumed_at IS NOT NULL) AND (consumed_by_idempotency_key IS NOT NULL) AND (schedule_request_hash IS NOT NULL) AND (operation_id IS NOT NULL))),
  CONSTRAINT "notification_campaign_confirmation_expiry_order" CHECK ((expires_at > issued_at) AND (dry_run_estimated_at <= issued_at)),
  CONSTRAINT "notification_campaign_confirmation_nonce_size" CHECK (octet_length(nonce_hash) = 32),
  CONSTRAINT "notification_campaign_confirmation_request_hash_size" CHECK ((schedule_request_hash IS NULL) OR (octet_length(schedule_request_hash) = 32)),
  CONSTRAINT "notification_campaign_confirmation_source_checksum_size" CHECK (octet_length(normalized_source_checksum) = 32),
  CONSTRAINT "notification_campaign_confirmation_source_kind" CHECK ((render_source_kind)::text = 'notification_campaign_body'::text)
);
-- Create index "notificationcampaignconfirmation_campaign_id_issued_at_id" to table: "notification_campaign_confirmations"
CREATE INDEX "notificationcampaignconfirmation_campaign_id_issued_at_id" ON "notification_campaign_confirmations" ("campaign_id", "issued_at", "id");
-- Create index "notificationcampaignconfirmation_expires_at_consumed_at" to table: "notification_campaign_confirmations"
CREATE INDEX "notificationcampaignconfirmation_expires_at_consumed_at" ON "notification_campaign_confirmations" ("expires_at", "consumed_at");
-- Create index "notificationcampaignconfirmation_nonce_hash" to table: "notification_campaign_confirmations"
CREATE UNIQUE INDEX "notificationcampaignconfirmation_nonce_hash" ON "notification_campaign_confirmations" ("nonce_hash");
-- Create index "notificationcampaignconfirmation_operation_id" to table: "notification_campaign_confirmations"
CREATE UNIQUE INDEX "notificationcampaignconfirmation_operation_id" ON "notification_campaign_confirmations" ("operation_id");
-- Create "notification_campaign_explicit_users" table
CREATE TABLE "notification_campaign_explicit_users" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL,
  "campaign_revision_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "notificationcampaignexplicituser_campaign_revision_id_user_id" to table: "notification_campaign_explicit_users"
CREATE UNIQUE INDEX "notificationcampaignexplicituser_campaign_revision_id_user_id" ON "notification_campaign_explicit_users" ("campaign_revision_id", "user_id");
-- Create index "notificationcampaignexplicituser_user_id_campaign_revision_id" to table: "notification_campaign_explicit_users"
CREATE INDEX "notificationcampaignexplicituser_user_id_campaign_revision_id" ON "notification_campaign_explicit_users" ("user_id", "campaign_revision_id");
-- Create "notification_campaign_recipients" table
CREATE TABLE "notification_campaign_recipients" (
  "id" uuid NOT NULL,
  "source_event_id" uuid NOT NULL,
  "status" character varying NOT NULL DEFAULT 'pending',
  "skip_code" character varying NULL,
  "failure_code" character varying NULL,
  "attempt_count" bigint NOT NULL DEFAULT 0,
  "next_attempt_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL,
  "last_attempt_at" timestamptz NULL,
  "processed_at" timestamptz NULL,
  "notification_id" uuid NULL,
  "campaign_id" uuid NOT NULL,
  "campaign_revision_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "notification_campaign_recipient_result_shape" CHECK ((((status)::text = 'pending'::text) AND (notification_id IS NULL) AND (skip_code IS NULL) AND (failure_code IS NULL) AND (processed_at IS NULL)) OR (((status)::text = 'created'::text) AND (notification_id IS NOT NULL) AND (skip_code IS NULL) AND (failure_code IS NULL) AND (processed_at IS NOT NULL)) OR (((status)::text = 'skipped'::text) AND (notification_id IS NULL) AND (skip_code IS NOT NULL) AND (failure_code IS NULL) AND (processed_at IS NOT NULL)) OR (((status)::text = 'failed'::text) AND (notification_id IS NULL) AND (skip_code IS NULL) AND (failure_code IS NOT NULL) AND (processed_at IS NOT NULL))),
  CONSTRAINT "notification_campaign_recipient_retry_shape" CHECK (((status)::text = 'pending'::text) OR (next_attempt_at IS NULL))
);
-- Create index "notification_campaign_recipients_notification_id_key" to table: "notification_campaign_recipients"
CREATE UNIQUE INDEX "notification_campaign_recipients_notification_id_key" ON "notification_campaign_recipients" ("notification_id");
-- Create index "notificationcampaignrecipient_campaign_id_status_next_attempt_a" to table: "notification_campaign_recipients"
CREATE INDEX "notificationcampaignrecipient_campaign_id_status_next_attempt_a" ON "notification_campaign_recipients" ("campaign_id", "status", "next_attempt_at", "user_id");
-- Create index "notificationcampaignrecipient_campaign_id_status_user_id" to table: "notification_campaign_recipients"
CREATE INDEX "notificationcampaignrecipient_campaign_id_status_user_id" ON "notification_campaign_recipients" ("campaign_id", "status", "user_id");
-- Create index "notificationcampaignrecipient_campaign_id_user_id" to table: "notification_campaign_recipients"
CREATE UNIQUE INDEX "notificationcampaignrecipient_campaign_id_user_id" ON "notification_campaign_recipients" ("campaign_id", "user_id");
-- Create index "notificationcampaignrecipient_campaign_revision_id_user_id" to table: "notification_campaign_recipients"
CREATE INDEX "notificationcampaignrecipient_campaign_revision_id_user_id" ON "notification_campaign_recipients" ("campaign_revision_id", "user_id");
-- Create index "notificationcampaignrecipient_source_event_id" to table: "notification_campaign_recipients"
CREATE UNIQUE INDEX "notificationcampaignrecipient_source_event_id" ON "notification_campaign_recipients" ("source_event_id");
-- Create "notification_campaign_revisions" table
CREATE TABLE "notification_campaign_revisions" (
  "id" uuid NOT NULL,
  "revision_number" bigint NOT NULL,
  "title" character varying NOT NULL,
  "body_markdown" text NOT NULL,
  "body_render_source_id" uuid NOT NULL,
  "body_render_source_kind" character varying NOT NULL,
  "body_render_artifact_id" uuid NOT NULL,
  "action_path" character varying NULL,
  "audience_kind" character varying NOT NULL,
  "audience_role_id" uuid NULL,
  "normalized_source_checksum" bytea NOT NULL,
  "created_at" timestamptz NOT NULL,
  "campaign_id" uuid NOT NULL,
  "author_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "notification_campaign_revision_audience_shape" CHECK ((((audience_kind)::text = 'role'::text) AND (audience_role_id IS NOT NULL)) OR (((audience_kind)::text = ANY ((ARRAY['explicit_users'::character varying, 'all_active'::character varying])::text[])) AND (audience_role_id IS NULL))),
  CONSTRAINT "notification_campaign_revision_body_bound" CHECK ((octet_length(body_markdown) >= 1) AND (octet_length(body_markdown) <= 65536)),
  CONSTRAINT "notification_campaign_revision_checksum_size" CHECK (octet_length(normalized_source_checksum) = 32),
  CONSTRAINT "notification_campaign_revision_source_kind" CHECK ((body_render_source_kind)::text = 'notification_campaign_body'::text)
);
-- Create index "notificationcampaignrevision_body_render_artifact_id_body_rende" to table: "notification_campaign_revisions"
CREATE UNIQUE INDEX "notificationcampaignrevision_body_render_artifact_id_body_rende" ON "notification_campaign_revisions" ("body_render_artifact_id", "body_render_source_id", "body_render_source_kind", "normalized_source_checksum");
-- Create index "notificationcampaignrevision_campaign_id_revision_number" to table: "notification_campaign_revisions"
CREATE UNIQUE INDEX "notificationcampaignrevision_campaign_id_revision_number" ON "notification_campaign_revisions" ("campaign_id", "revision_number");
-- Create index "notificationcampaignrevision_id_body_render_source_id_body_rend" to table: "notification_campaign_revisions"
CREATE UNIQUE INDEX "notificationcampaignrevision_id_body_render_source_id_body_rend" ON "notification_campaign_revisions" ("id", "body_render_source_id", "body_render_source_kind", "normalized_source_checksum");
-- Create index "notificationcampaignrevision_id_campaign_id_body_render_source_" to table: "notification_campaign_revisions"
CREATE UNIQUE INDEX "notificationcampaignrevision_id_campaign_id_body_render_source_" ON "notification_campaign_revisions" ("id", "campaign_id", "body_render_source_id", "body_render_source_kind", "normalized_source_checksum");
-- Create "notification_campaigns" table
CREATE TABLE "notification_campaigns" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "status" character varying NOT NULL DEFAULT 'draft',
  "current_draft_revision_id" uuid NOT NULL,
  "scheduled_revision_id" uuid NULL,
  "scheduled_render_source_id" uuid NULL,
  "scheduled_render_source_kind" character varying NULL,
  "scheduled_normalized_source_checksum" bytea NULL,
  "scheduled_render_artifact_id" uuid NULL,
  "operation_id" uuid NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "scheduled_for" timestamptz NULL,
  "last_failure_code" character varying NULL,
  "scheduled_by" uuid NULL,
  "cancelled_by" uuid NULL,
  "schedule_reason" character varying NULL,
  "cancel_reason" character varying NULL,
  "scheduled_at" timestamptz NULL,
  "started_at" timestamptz NULL,
  "completed_at" timestamptz NULL,
  "cancelled_at" timestamptz NULL,
  "created_by" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "notification_campaign_cancel_shape" CHECK ((((status)::text = 'cancelled'::text) AND (cancelled_by IS NOT NULL) AND (cancel_reason IS NOT NULL) AND (cancelled_at IS NOT NULL)) OR (((status)::text <> 'cancelled'::text) AND (cancelled_by IS NULL) AND (cancel_reason IS NULL) AND (cancelled_at IS NULL))),
  CONSTRAINT "notification_campaign_completion_shape" CHECK ((((status)::text = ANY ((ARRAY['completed'::character varying, 'failed'::character varying])::text[])) AND (completed_at IS NOT NULL)) OR (((status)::text <> ALL ((ARRAY['completed'::character varying, 'failed'::character varying])::text[])) AND (completed_at IS NULL))),
  CONSTRAINT "notification_campaign_failure_shape" CHECK ((((status)::text = 'failed'::text) AND (last_failure_code IS NOT NULL)) OR ((status)::text <> 'failed'::text)),
  CONSTRAINT "notification_campaign_positive_version" CHECK (version >= 1),
  CONSTRAINT "notification_campaign_schedule_checksum" CHECK ((scheduled_normalized_source_checksum IS NULL) OR (octet_length(scheduled_normalized_source_checksum) = 32)),
  CONSTRAINT "notification_campaign_schedule_shape" CHECK ((((status)::text = 'draft'::text) AND (scheduled_revision_id IS NULL) AND (scheduled_render_source_id IS NULL) AND (scheduled_render_source_kind IS NULL) AND (scheduled_normalized_source_checksum IS NULL) AND (scheduled_render_artifact_id IS NULL) AND (operation_id IS NULL) AND (scheduled_by IS NULL) AND (schedule_reason IS NULL) AND (scheduled_at IS NULL) AND (started_at IS NULL) AND (completed_at IS NULL)) OR (((status)::text <> 'draft'::text) AND (scheduled_revision_id IS NOT NULL) AND (scheduled_render_source_id IS NOT NULL) AND ((scheduled_render_source_kind)::text = 'notification_campaign_body'::text) AND (scheduled_normalized_source_checksum IS NOT NULL) AND (scheduled_render_artifact_id IS NOT NULL) AND (operation_id IS NOT NULL) AND (scheduled_by IS NOT NULL) AND (schedule_reason IS NOT NULL) AND (scheduled_at IS NOT NULL)))
);
-- Create index "notificationcampaign_created_by_created_at_id" to table: "notification_campaigns"
CREATE INDEX "notificationcampaign_created_by_created_at_id" ON "notification_campaigns" ("created_by", "created_at", "id");
-- Create index "notificationcampaign_operation_id" to table: "notification_campaigns"
CREATE UNIQUE INDEX "notificationcampaign_operation_id" ON "notification_campaigns" ("operation_id");
-- Create index "notificationcampaign_scheduled_revision_id" to table: "notification_campaigns"
CREATE INDEX "notificationcampaign_scheduled_revision_id" ON "notification_campaigns" ("scheduled_revision_id");
-- Create index "notificationcampaign_status_scheduled_for_created_at_id" to table: "notification_campaigns"
CREATE INDEX "notificationcampaign_status_scheduled_for_created_at_id" ON "notification_campaigns" ("status", "scheduled_for", "created_at", "id");
-- Create "notification_inbox_states" table
CREATE TABLE "notification_inbox_states" (
  "user_id" uuid NOT NULL,
  "version" bigint NOT NULL DEFAULT 0,
  "unread_count" bigint NOT NULL DEFAULT 0,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("user_id")
);
-- Create index "notificationinboxstate_updated_at_user_id" to table: "notification_inbox_states"
CREATE INDEX "notificationinboxstate_updated_at_user_id" ON "notification_inbox_states" ("updated_at", "user_id");
-- Create "notification_preferences" table
CREATE TABLE "notification_preferences" (
  "id" uuid NOT NULL,
  "category" character varying NOT NULL,
  "in_app" boolean NOT NULL DEFAULT true,
  "version" bigint NOT NULL DEFAULT 1,
  "updated_at" timestamptz NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "notification_preference_positive_version" CHECK (version >= 1),
  CONSTRAINT "notification_preference_required_categories" CHECK (((category)::text <> ALL ((ARRAY['security'::character varying, 'authorization'::character varying, 'operation'::character varying])::text[])) OR in_app)
);
-- Create index "notificationpreference_user_id_category" to table: "notification_preferences"
CREATE UNIQUE INDEX "notificationpreference_user_id_category" ON "notification_preferences" ("user_id", "category");
-- Create "notifications" table
CREATE TABLE "notifications" (
  "id" uuid NOT NULL,
  "type" character varying NOT NULL,
  "category" character varying NOT NULL,
  "content_kind" character varying NOT NULL,
  "title_key" character varying NULL,
  "body_key" character varying NULL,
  "params" jsonb NULL,
  "action_path" character varying NULL,
  "source_event_id" uuid NOT NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "created_at" timestamptz NOT NULL,
  "read_at" timestamptz NULL,
  "archived_at" timestamptz NULL,
  "expires_at" timestamptz NULL,
  "campaign_revision_id" uuid NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "notification_content_shape" CHECK ((((content_kind)::text = 'catalog'::text) AND (title_key IS NOT NULL) AND (body_key IS NOT NULL) AND (params IS NOT NULL) AND (campaign_revision_id IS NULL)) OR (((content_kind)::text = 'campaign'::text) AND (title_key IS NULL) AND (body_key IS NULL) AND (params IS NULL) AND (campaign_revision_id IS NOT NULL))),
  CONSTRAINT "notification_expiry_after_creation" CHECK ((expires_at IS NULL) OR (expires_at > created_at)),
  CONSTRAINT "notification_params_bound" CHECK ((params IS NULL) OR (octet_length((params)::text) <= 16384)),
  CONSTRAINT "notification_positive_version" CHECK (version >= 1),
  CONSTRAINT "notification_read_before_archive" CHECK ((archived_at IS NULL) OR (read_at IS NOT NULL))
);
-- Create index "notification_campaign_revision_id_user_id" to table: "notifications"
CREATE INDEX "notification_campaign_revision_id_user_id" ON "notifications" ("campaign_revision_id", "user_id");
-- Create index "notification_user_id_archived_at_created_at_id" to table: "notifications"
CREATE INDEX "notification_user_id_archived_at_created_at_id" ON "notifications" ("user_id", "archived_at", "created_at", "id");
-- Create index "notification_user_id_created_at_id" to table: "notifications"
CREATE INDEX "notification_user_id_created_at_id" ON "notifications" ("user_id", "created_at", "id");
-- Create index "notification_user_id_read_at_expires_at" to table: "notifications"
CREATE INDEX "notification_user_id_read_at_expires_at" ON "notifications" ("user_id", "read_at", "expires_at");
-- Create index "notification_user_id_source_event_id_type" to table: "notifications"
CREATE UNIQUE INDEX "notification_user_id_source_event_id_type" ON "notifications" ("user_id", "source_event_id", "type");
-- Create "oauth_completion_tickets" table
CREATE TABLE "oauth_completion_tickets" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "ticket_hash" bytea NOT NULL,
  "provider" character varying NOT NULL,
  "provider_subject" character varying NOT NULL,
  "purpose" character varying NOT NULL,
  "pre_session_id" uuid NULL,
  "provider_profile_ciphertext" bytea NOT NULL,
  "key_id" character varying NOT NULL,
  "status" character varying NOT NULL DEFAULT 'issued',
  "expires_at" timestamptz NOT NULL,
  "consumed_at" timestamptz NULL,
  "bound_session_id" uuid NULL,
  "flow_id" uuid NOT NULL,
  "bound_user_id" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "oauth_ticket_purpose_binding" CHECK ((((purpose)::text = 'login'::text) AND (pre_session_id IS NOT NULL) AND (bound_session_id IS NULL)) OR (((purpose)::text = 'link'::text) AND (bound_user_id IS NOT NULL) AND (bound_session_id IS NOT NULL))),
  CONSTRAINT "oauth_ticket_purpose_closed" CHECK ((purpose)::text = ANY ((ARRAY['login'::character varying, 'link'::character varying])::text[])),
  CONSTRAINT "oauth_ticket_status_closed" CHECK ((status)::text = ANY ((ARRAY['issued'::character varying, 'consumed'::character varying, 'expired'::character varying])::text[]))
);
-- Create index "oauth_completion_tickets_ticket_hash_key" to table: "oauth_completion_tickets"
CREATE UNIQUE INDEX "oauth_completion_tickets_ticket_hash_key" ON "oauth_completion_tickets" ("ticket_hash");
-- Create index "oauthcompletionticket_bound_session_id_purpose_status" to table: "oauth_completion_tickets"
CREATE INDEX "oauthcompletionticket_bound_session_id_purpose_status" ON "oauth_completion_tickets" ("bound_session_id", "purpose", "status");
-- Create index "oauthcompletionticket_pre_session_id_purpose_status" to table: "oauth_completion_tickets"
CREATE INDEX "oauthcompletionticket_pre_session_id_purpose_status" ON "oauth_completion_tickets" ("pre_session_id", "purpose", "status");
-- Create index "oauthcompletionticket_provider_provider_subject" to table: "oauth_completion_tickets"
CREATE INDEX "oauthcompletionticket_provider_provider_subject" ON "oauth_completion_tickets" ("provider", "provider_subject");
-- Create index "oauthcompletionticket_status_expires_at" to table: "oauth_completion_tickets"
CREATE INDEX "oauthcompletionticket_status_expires_at" ON "oauth_completion_tickets" ("status", "expires_at");
-- Create "oauth_flows" table
CREATE TABLE "oauth_flows" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "provider" character varying NOT NULL,
  "purpose" character varying NOT NULL,
  "state_hash" bytea NOT NULL,
  "pre_session_id" uuid NULL,
  "redirect_uri" character varying NOT NULL,
  "pkce_verifier_ciphertext" bytea NOT NULL,
  "key_id" character varying NOT NULL,
  "status" character varying NOT NULL DEFAULT 'pending',
  "expires_at" timestamptz NOT NULL,
  "consumed_at" timestamptz NULL,
  "session_id" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "oauth_flow_purpose_binding" CHECK ((((purpose)::text = 'login'::text) AND (pre_session_id IS NOT NULL) AND (session_id IS NULL)) OR (((purpose)::text = 'link'::text) AND (session_id IS NOT NULL))),
  CONSTRAINT "oauth_flow_purpose_closed" CHECK ((purpose)::text = ANY ((ARRAY['login'::character varying, 'link'::character varying])::text[])),
  CONSTRAINT "oauth_flow_status_closed" CHECK ((status)::text = ANY ((ARRAY['pending'::character varying, 'consumed'::character varying, 'expired'::character varying])::text[]))
);
-- Create index "oauth_flows_state_hash_key" to table: "oauth_flows"
CREATE UNIQUE INDEX "oauth_flows_state_hash_key" ON "oauth_flows" ("state_hash");
-- Create index "oauthflow_pre_session_id_purpose_status" to table: "oauth_flows"
CREATE INDEX "oauthflow_pre_session_id_purpose_status" ON "oauth_flows" ("pre_session_id", "purpose", "status");
-- Create index "oauthflow_provider_status_expires_at" to table: "oauth_flows"
CREATE INDEX "oauthflow_provider_status_expires_at" ON "oauth_flows" ("provider", "status", "expires_at");
-- Create index "oauthflow_session_id_purpose_status" to table: "oauth_flows"
CREATE INDEX "oauthflow_session_id_purpose_status" ON "oauth_flows" ("session_id", "purpose", "status");
-- Create index "oauthflow_status_expires_at" to table: "oauth_flows"
CREATE INDEX "oauthflow_status_expires_at" ON "oauth_flows" ("status", "expires_at");
-- Create "onboarding_trait_offer_talents" table
CREATE TABLE "onboarding_trait_offer_talents" (
  "id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY,
  "trait_revision" bigint NOT NULL,
  "display_order" bigint NOT NULL,
  "offer_id" uuid NOT NULL,
  "trait_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "onboardingtraitoffertalent_offer_id_display_order" to table: "onboarding_trait_offer_talents"
CREATE UNIQUE INDEX "onboardingtraitoffertalent_offer_id_display_order" ON "onboarding_trait_offer_talents" ("offer_id", "display_order");
-- Create index "onboardingtraitoffertalent_offer_id_trait_id" to table: "onboarding_trait_offer_talents"
CREATE UNIQUE INDEX "onboardingtraitoffertalent_offer_id_trait_id" ON "onboarding_trait_offer_talents" ("offer_id", "trait_id");
-- Create "onboarding_trait_offers" table
CREATE TABLE "onboarding_trait_offers" (
  "id" uuid NOT NULL,
  "pre_session_id" uuid NOT NULL,
  "idempotency_key" uuid NOT NULL,
  "offer_sequence" bigint NOT NULL,
  "algorithm_version" character varying NOT NULL,
  "root_bone_trait_id" uuid NOT NULL,
  "root_bone_trait_revision" bigint NOT NULL,
  "offer_checksum" bytea NOT NULL,
  "status" character varying NOT NULL DEFAULT 'issued',
  "issued_at" timestamptz NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "consumed_at" timestamptz NULL,
  "catalog_revision_id" uuid NOT NULL,
  "consumed_by_user_id" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "onboarding_trait_offer_checksum_shape" CHECK ((octet_length(offer_checksum) = 32) AND (offer_checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "onboarding_trait_offer_consumption_shape" CHECK ((((status)::text = 'consumed'::text) AND (consumed_by_user_id IS NOT NULL) AND (consumed_at IS NOT NULL)) OR (((status)::text <> 'consumed'::text) AND (consumed_by_user_id IS NULL) AND (consumed_at IS NULL))),
  CONSTRAINT "onboarding_trait_offer_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (pre_session_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (idempotency_key <> '00000000-0000-0000-0000-000000000000'::uuid) AND (root_bone_trait_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "onboarding_trait_offer_time_shape" CHECK (expires_at > issued_at)
);
-- Create index "onboardingtraitoffer_consumed_by_user_id" to table: "onboarding_trait_offers"
CREATE UNIQUE INDEX "onboardingtraitoffer_consumed_by_user_id" ON "onboarding_trait_offers" ("consumed_by_user_id") WHERE (consumed_by_user_id IS NOT NULL);
-- Create index "onboardingtraitoffer_pre_session_id_idempotency_key" to table: "onboarding_trait_offers"
CREATE UNIQUE INDEX "onboardingtraitoffer_pre_session_id_idempotency_key" ON "onboarding_trait_offers" ("pre_session_id", "idempotency_key");
-- Create index "onboardingtraitoffer_pre_session_id_offer_sequence" to table: "onboarding_trait_offers"
CREATE UNIQUE INDEX "onboardingtraitoffer_pre_session_id_offer_sequence" ON "onboarding_trait_offers" ("pre_session_id", "offer_sequence");
-- Create index "onboardingtraitoffer_pre_session_id_status_offer_sequence" to table: "onboarding_trait_offers"
CREATE INDEX "onboardingtraitoffer_pre_session_id_status_offer_sequence" ON "onboarding_trait_offers" ("pre_session_id", "status", "offer_sequence");
-- Create "operation_batches" table
CREATE TABLE "operation_batches" (
  "id" uuid NOT NULL,
  "sequence" bigint NOT NULL,
  "lease_token" uuid NOT NULL,
  "input_checkpoint" jsonb NOT NULL,
  "output_checkpoint" jsonb NOT NULL,
  "processed_count" bigint NOT NULL,
  "succeeded_count" bigint NOT NULL,
  "failed_count" bigint NOT NULL,
  "failure_summary" jsonb NOT NULL,
  "started_at" timestamptz NOT NULL,
  "completed_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "operation_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "operation_batch_progress_exact" CHECK (processed_count = (succeeded_count + failed_count)),
  CONSTRAINT "operation_batch_time_order" CHECK (completed_at >= started_at)
);
-- Create index "operationbatch_operation_id_completed_at_id" to table: "operation_batches"
CREATE INDEX "operationbatch_operation_id_completed_at_id" ON "operation_batches" ("operation_id", "completed_at", "id");
-- Create index "operationbatch_operation_id_sequence" to table: "operation_batches"
CREATE UNIQUE INDEX "operationbatch_operation_id_sequence" ON "operation_batches" ("operation_id", "sequence");
-- Create "operation_control_receipts" table
CREATE TABLE "operation_control_receipts" (
  "id" uuid NOT NULL,
  "idempotency_key" uuid NOT NULL,
  "request_hash" bytea NOT NULL,
  "action" character varying NOT NULL,
  "expected_version" bigint NOT NULL,
  "resulting_version" bigint NOT NULL,
  "resulting_status" character varying NOT NULL,
  "result_projection" jsonb NOT NULL,
  "actor_user_id" uuid NOT NULL,
  "actor_session_id" uuid NOT NULL,
  "audit_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "operation_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "operation_control_request_hash_size" CHECK (octet_length(request_hash) = 32),
  CONSTRAINT "operation_control_result_object" CHECK (jsonb_typeof(result_projection) = 'object'::text),
  CONSTRAINT "operation_control_version_advanced" CHECK (resulting_version > expected_version)
);
-- Create index "operation_control_receipts_audit_id_key" to table: "operation_control_receipts"
CREATE UNIQUE INDEX "operation_control_receipts_audit_id_key" ON "operation_control_receipts" ("audit_id");
-- Create index "operationcontrolreceipt_operation_id_created_at_id" to table: "operation_control_receipts"
CREATE INDEX "operationcontrolreceipt_operation_id_created_at_id" ON "operation_control_receipts" ("operation_id", "created_at", "id");
-- Create index "operationcontrolreceipt_operation_id_idempotency_key" to table: "operation_control_receipts"
CREATE UNIQUE INDEX "operationcontrolreceipt_operation_id_idempotency_key" ON "operation_control_receipts" ("operation_id", "idempotency_key");
-- Create "operations" table
CREATE TABLE "operations" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "kind" character varying NOT NULL,
  "idempotency_key" uuid NOT NULL,
  "request_hash" bytea NOT NULL,
  "request_payload" jsonb NOT NULL,
  "actor_user_id" uuid NOT NULL,
  "actor_session_id" uuid NULL,
  "audit_id" uuid NOT NULL,
  "reason" character varying NOT NULL,
  "correlation_id" character varying NOT NULL,
  "status" character varying NOT NULL DEFAULT 'pending',
  "requested_action" character varying NOT NULL DEFAULT 'none',
  "total_count" bigint NOT NULL,
  "processed_count" bigint NOT NULL DEFAULT 0,
  "succeeded_count" bigint NOT NULL DEFAULT 0,
  "failed_count" bigint NOT NULL DEFAULT 0,
  "next_batch_sequence" bigint NOT NULL DEFAULT 1,
  "batch_size" bigint NOT NULL,
  "checkpoint" jsonb NOT NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "lease_owner" character varying NULL,
  "lease_token" uuid NULL,
  "leased_until" timestamptz NULL,
  "not_before" timestamptz NULL,
  "available_at" timestamptz NOT NULL,
  "attempt_count" bigint NOT NULL DEFAULT 0,
  "last_error_code" character varying NULL,
  "started_at" timestamptz NULL,
  "finished_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "operation_cancelled_state" CHECK (((status)::text <> 'cancelled'::text) OR ((lease_owner IS NULL) AND ((requested_action)::text = 'none'::text) AND (finished_at IS NOT NULL))),
  CONSTRAINT "operation_completed_state" CHECK (((status)::text <> 'completed'::text) OR ((processed_count = total_count) AND (lease_owner IS NULL) AND ((requested_action)::text = 'none'::text) AND (finished_at IS NOT NULL) AND (last_error_code IS NULL))),
  CONSTRAINT "operation_failed_state" CHECK (((status)::text <> 'failed'::text) OR ((lease_owner IS NULL) AND ((requested_action)::text = 'none'::text) AND (finished_at IS NOT NULL) AND (last_error_code IS NOT NULL))),
  CONSTRAINT "operation_lease_complete" CHECK (((lease_owner IS NULL) AND (lease_token IS NULL) AND (leased_until IS NULL)) OR ((lease_owner IS NOT NULL) AND (lease_token IS NOT NULL) AND (leased_until IS NOT NULL))),
  CONSTRAINT "operation_lease_state" CHECK ((lease_owner IS NULL) OR ((status)::text = 'running'::text)),
  CONSTRAINT "operation_nonterminal_state" CHECK (((status)::text = ANY ((ARRAY['completed'::character varying, 'failed'::character varying, 'cancelled'::character varying])::text[])) OR (finished_at IS NULL)),
  CONSTRAINT "operation_paused_state" CHECK (((status)::text <> 'paused'::text) OR ((lease_owner IS NULL) AND ((requested_action)::text = 'none'::text) AND (finished_at IS NULL))),
  CONSTRAINT "operation_pending_state" CHECK (((status)::text <> 'pending'::text) OR ((processed_count = 0) AND (lease_owner IS NULL) AND (started_at IS NULL) AND (finished_at IS NULL))),
  CONSTRAINT "operation_progress_exact" CHECK ((processed_count = (succeeded_count + failed_count)) AND (processed_count <= total_count)),
  CONSTRAINT "operation_request_hash_size" CHECK (octet_length(request_hash) = 32),
  CONSTRAINT "operation_schedule_floor" CHECK ((not_before IS NULL) OR (available_at >= not_before))
);
-- Create index "operation_actor_user_id_created_at_id" to table: "operations"
CREATE INDEX "operation_actor_user_id_created_at_id" ON "operations" ("actor_user_id", "created_at", "id");
-- Create index "operation_audit_id" to table: "operations"
CREATE UNIQUE INDEX "operation_audit_id" ON "operations" ("audit_id");
-- Create index "operation_correlation_id_created_at_id" to table: "operations"
CREATE INDEX "operation_correlation_id_created_at_id" ON "operations" ("correlation_id", "created_at", "id");
-- Create index "operation_kind_idempotency_key" to table: "operations"
CREATE UNIQUE INDEX "operation_kind_idempotency_key" ON "operations" ("kind", "idempotency_key");
-- Create index "operation_not_before_created_at_id" to table: "operations"
CREATE INDEX "operation_not_before_created_at_id" ON "operations" ("not_before", "created_at", "id");
-- Create index "operation_status_available_at_created_at_id" to table: "operations"
CREATE INDEX "operation_status_available_at_created_at_id" ON "operations" ("status", "available_at", "created_at", "id");
-- Create index "operation_status_leased_until" to table: "operations"
CREATE INDEX "operation_status_leased_until" ON "operations" ("status", "leased_until");
-- Create "outbox_events" table
CREATE TABLE "outbox_events" (
  "id" uuid NOT NULL,
  "event_type" character varying NOT NULL,
  "aggregate_type" character varying NOT NULL,
  "aggregate_id" uuid NOT NULL,
  "aggregate_version" bigint NOT NULL,
  "correlation_id" character varying NOT NULL,
  "causation_id" uuid NULL,
  "payload" jsonb NOT NULL,
  "occurred_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "state" character varying NOT NULL DEFAULT 'pending',
  "attempt_count" bigint NOT NULL DEFAULT 0,
  "lease_owner" character varying NULL,
  "lease_token" character varying NULL,
  "leased_until" timestamptz NULL,
  "next_attempt_at" timestamptz NOT NULL,
  "last_attempt_at" timestamptz NULL,
  "published_at" timestamptz NULL,
  "dead_at" timestamptz NULL,
  "publish_reference" character varying NULL,
  "last_error_code" character varying NULL,
  "ambiguous" boolean NOT NULL DEFAULT false,
  PRIMARY KEY ("id"),
  CONSTRAINT "outbox_lease_fields_complete" CHECK (((lease_owner IS NULL) AND (lease_token IS NULL) AND (leased_until IS NULL)) OR ((lease_owner IS NOT NULL) AND (lease_token IS NOT NULL) AND (leased_until IS NOT NULL))),
  CONSTRAINT "outbox_state_timestamps" CHECK ((((state)::text = 'pending'::text) AND (lease_owner IS NULL) AND (published_at IS NULL) AND (dead_at IS NULL)) OR (((state)::text = 'leased'::text) AND (lease_owner IS NOT NULL) AND (published_at IS NULL) AND (dead_at IS NULL)) OR (((state)::text = 'published'::text) AND (lease_owner IS NULL) AND (published_at IS NOT NULL) AND (dead_at IS NULL)) OR (((state)::text = 'dead'::text) AND (lease_owner IS NULL) AND (published_at IS NULL) AND (dead_at IS NOT NULL)))
);
-- Create index "outboxevent_aggregate_type_aggregate_id_aggregate_version" to table: "outbox_events"
CREATE INDEX "outboxevent_aggregate_type_aggregate_id_aggregate_version" ON "outbox_events" ("aggregate_type", "aggregate_id", "aggregate_version");
-- Create index "outboxevent_correlation_id_occurred_at_id" to table: "outbox_events"
CREATE INDEX "outboxevent_correlation_id_occurred_at_id" ON "outbox_events" ("correlation_id", "occurred_at", "id");
-- Create index "outboxevent_event_type_aggregate_type_aggregate_id_aggregate_ve" to table: "outbox_events"
CREATE UNIQUE INDEX "outboxevent_event_type_aggregate_type_aggregate_id_aggregate_ve" ON "outbox_events" ("event_type", "aggregate_type", "aggregate_id", "aggregate_version");
-- Create index "outboxevent_state_dead_at" to table: "outbox_events"
CREATE INDEX "outboxevent_state_dead_at" ON "outbox_events" ("state", "dead_at");
-- Create index "outboxevent_state_leased_until" to table: "outbox_events"
CREATE INDEX "outboxevent_state_leased_until" ON "outbox_events" ("state", "leased_until");
-- Create index "outboxevent_state_next_attempt_at_occurred_at_id" to table: "outbox_events"
CREATE INDEX "outboxevent_state_next_attempt_at_occurred_at_id" ON "outbox_events" ("state", "next_attempt_at", "occurred_at", "id");
-- Create index "outboxevent_state_published_at" to table: "outbox_events"
CREATE INDEX "outboxevent_state_published_at" ON "outbox_events" ("state", "published_at");
-- Create "password_reset_tokens" table
CREATE TABLE "password_reset_tokens" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "contact_version" bigint NOT NULL,
  "user_security_version" bigint NOT NULL,
  "hash_key_id" character varying NOT NULL,
  "token_hash" bytea NOT NULL,
  "status" character varying NOT NULL DEFAULT 'issued',
  "expires_at" timestamptz NOT NULL,
  "consumed_at" timestamptz NULL,
  "superseded_at" timestamptz NULL,
  "attempt_count" bigint NOT NULL DEFAULT 0,
  "last_attempt_at" timestamptz NULL,
  "user_id" uuid NOT NULL,
  "contact_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "password_reset_status_closed" CHECK ((status)::text = ANY ((ARRAY['issued'::character varying, 'consumed'::character varying, 'superseded'::character varying, 'expired'::character varying])::text[]))
);
-- Create index "passwordresettoken_contact_id_contact_version_status" to table: "password_reset_tokens"
CREATE INDEX "passwordresettoken_contact_id_contact_version_status" ON "password_reset_tokens" ("contact_id", "contact_version", "status");
-- Create index "passwordresettoken_hash_key_id_token_hash" to table: "password_reset_tokens"
CREATE UNIQUE INDEX "passwordresettoken_hash_key_id_token_hash" ON "password_reset_tokens" ("hash_key_id", "token_hash");
-- Create index "passwordresettoken_status_expires_at" to table: "password_reset_tokens"
CREATE INDEX "passwordresettoken_status_expires_at" ON "password_reset_tokens" ("status", "expires_at");
-- Create index "passwordresettoken_user_id_status_expires_at" to table: "password_reset_tokens"
CREATE INDEX "passwordresettoken_user_id_status_expires_at" ON "password_reset_tokens" ("user_id", "status", "expires_at");
-- Create "problem_revisions" table
CREATE TABLE "problem_revisions" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "revision_number" bigint NOT NULL,
  "title" character varying NOT NULL,
  "statement_markdown" character varying NOT NULL,
  "render_source_id" uuid NOT NULL,
  "render_source_kind" character varying NOT NULL,
  "render_source_checksum" bytea NOT NULL,
  "difficulty_id" uuid NOT NULL,
  "difficulty_revision" bigint NOT NULL,
  "base_exp" bigint NOT NULL,
  "tag_mapping_id" uuid NOT NULL,
  "tag_mapping_revision" bigint NOT NULL,
  "tag_snapshot" jsonb NOT NULL,
  "element_mapping_id" uuid NULL,
  "element_mapping_revision" bigint NULL,
  "element_snapshot" jsonb NULL,
  "reward_snapshot_checksum" bytea NOT NULL,
  "cpu_time_limit_ms" bigint NOT NULL,
  "wall_time_limit_ms" bigint NOT NULL,
  "memory_limit_kb" bigint NOT NULL,
  "output_limit_bytes" bigint NOT NULL,
  "process_limit" bigint NOT NULL,
  "checker_key" character varying NOT NULL,
  "checker_version" character varying NOT NULL,
  "checker_config" jsonb NOT NULL,
  "allowed_runtime_keys" jsonb NOT NULL,
  "created_by" uuid NULL,
  "content_checksum" bytea NOT NULL,
  "problem_id" uuid NOT NULL,
  "testset_revision_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "problem_revision_checker_config_bound" CHECK ((jsonb_typeof(checker_config) = 'object'::text) AND (octet_length((checker_config)::text) <= 4096)),
  CONSTRAINT "problem_revision_checker_identity_bound" CHECK (((octet_length((checker_key)::text) >= 1) AND (octet_length((checker_key)::text) <= 64)) AND ((octet_length((checker_version)::text) >= 1) AND (octet_length((checker_version)::text) <= 64))),
  CONSTRAINT "problem_revision_checksum_shape" CHECK ((octet_length(render_source_checksum) = 32) AND (octet_length(reward_snapshot_checksum) = 32) AND (octet_length(content_checksum) = 32)),
  CONSTRAINT "problem_revision_difficulty_bound" CHECK ((difficulty_revision >= 1) AND (difficulty_revision <= 1000000000)),
  CONSTRAINT "problem_revision_element_snapshot_complete" CHECK (((element_mapping_id IS NULL) AND (element_mapping_revision IS NULL) AND (element_snapshot IS NULL)) OR ((element_mapping_id IS NOT NULL) AND (element_mapping_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (element_mapping_revision IS NOT NULL) AND (element_snapshot IS NOT NULL))),
  CONSTRAINT "problem_revision_limit_bounds" CHECK (((cpu_time_limit_ms >= 10) AND (cpu_time_limit_ms <= 30000)) AND ((wall_time_limit_ms >= 10) AND (wall_time_limit_ms <= 60000)) AND ((memory_limit_kb >= 16384) AND (memory_limit_kb <= 2097152)) AND ((output_limit_bytes >= 1) AND (output_limit_bytes <= 1048576)) AND ((process_limit >= 1) AND (process_limit <= 64))),
  CONSTRAINT "problem_revision_nonzero_checksums" CHECK ((render_source_checksum <> decode(repeat('00'::text, 32), 'hex'::text)) AND (reward_snapshot_checksum <> decode(repeat('00'::text, 32), 'hex'::text)) AND (content_checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "problem_revision_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (problem_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (render_source_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (difficulty_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (tag_mapping_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (testset_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((created_by IS NULL) OR (created_by <> '00000000-0000-0000-0000-000000000000'::uuid))),
  CONSTRAINT "problem_revision_number_bound" CHECK ((revision_number >= 1) AND (revision_number <= 1000000000)),
  CONSTRAINT "problem_revision_reward_mapping_bound" CHECK (((tag_mapping_revision >= 1) AND (tag_mapping_revision <= 1000000000)) AND ((element_mapping_revision IS NULL) OR ((element_mapping_revision >= 1) AND (element_mapping_revision <= 1000000000))) AND ((base_exp >= 0) AND (base_exp <= 1000000000))),
  CONSTRAINT "problem_revision_reward_snapshot_bound" CHECK ((jsonb_typeof(tag_snapshot) = 'object'::text) AND (octet_length((tag_snapshot)::text) <= 65536) AND ((element_snapshot IS NULL) OR ((jsonb_typeof(element_snapshot) = 'object'::text) AND (octet_length((element_snapshot)::text) <= 65536)))),
  CONSTRAINT "problem_revision_runtime_keys_bound" CHECK ((jsonb_typeof(allowed_runtime_keys) = 'array'::text) AND ((jsonb_array_length(allowed_runtime_keys) >= 1) AND (jsonb_array_length(allowed_runtime_keys) <= 16)) AND (octet_length((allowed_runtime_keys)::text) <= 1072) AND (NOT jsonb_path_exists(allowed_runtime_keys, '$[*]?(@.type() != "string")'::jsonpath))),
  CONSTRAINT "problem_revision_source_kind" CHECK ((render_source_kind)::text = 'problem_statement'::text),
  CONSTRAINT "problem_revision_statement_bound" CHECK ((octet_length((statement_markdown)::text) >= 1) AND (octet_length((statement_markdown)::text) <= 1048576)),
  CONSTRAINT "problem_revision_title_bound" CHECK ((octet_length((title)::text) >= 1) AND (octet_length((title)::text) <= 300)),
  CONSTRAINT "problem_revision_wall_covers_cpu" CHECK (wall_time_limit_ms >= cpu_time_limit_ms)
);
-- Create index "problem_revisions_submission_spec_identity" to table: "problem_revisions"
CREATE UNIQUE INDEX "problem_revisions_submission_spec_identity" ON "problem_revisions" ("id", "problem_id", "testset_revision_id");
-- Create index "problemrevision_content_checksum" to table: "problem_revisions"
CREATE INDEX "problemrevision_content_checksum" ON "problem_revisions" ("content_checksum");
-- Create index "problemrevision_id_problem_id" to table: "problem_revisions"
CREATE UNIQUE INDEX "problemrevision_id_problem_id" ON "problem_revisions" ("id", "problem_id");
-- Create index "problemrevision_id_render_source_id_render_source_kind_render_s" to table: "problem_revisions"
CREATE UNIQUE INDEX "problemrevision_id_render_source_id_render_source_kind_render_s" ON "problem_revisions" ("id", "render_source_id", "render_source_kind", "render_source_checksum");
-- Create index "problemrevision_problem_id_created_at_id" to table: "problem_revisions"
CREATE INDEX "problemrevision_problem_id_created_at_id" ON "problem_revisions" ("problem_id", "created_at", "id");
-- Create index "problemrevision_problem_id_revision_number" to table: "problem_revisions"
CREATE UNIQUE INDEX "problemrevision_problem_id_revision_number" ON "problem_revisions" ("problem_id", "revision_number");
-- Create index "problemrevision_render_source_id_render_source_kind" to table: "problem_revisions"
CREATE UNIQUE INDEX "problemrevision_render_source_id_render_source_kind" ON "problem_revisions" ("render_source_id", "render_source_kind");
-- Create index "problemrevision_testset_revision_id" to table: "problem_revisions"
CREATE INDEX "problemrevision_testset_revision_id" ON "problem_revisions" ("testset_revision_id");
-- Create "problem_tags" table
CREATE TABLE "problem_tags" (
  "problem_id" uuid NOT NULL,
  "tag_id" uuid NOT NULL,
  PRIMARY KEY ("problem_id", "tag_id")
);
-- Create "problems" table
CREATE TABLE "problems" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "slug" character varying NOT NULL,
  "lifecycle" character varying NOT NULL DEFAULT 'draft',
  "version" bigint NOT NULL DEFAULT 1,
  "tag_mapping_id" uuid NOT NULL,
  "element_mapping_id" uuid NOT NULL,
  "draft_revision_id" uuid NULL,
  "draft_render_source_id" uuid NULL,
  "draft_render_source_kind" character varying NULL,
  "draft_render_source_checksum" bytea NULL,
  "draft_render_artifact_id" uuid NULL,
  "published_revision_id" uuid NULL,
  "published_render_source_id" uuid NULL,
  "published_render_source_kind" character varying NULL,
  "published_render_source_checksum" bytea NULL,
  "published_render_artifact_id" uuid NULL,
  "published_at" timestamptz NULL,
  "archived_at" timestamptz NULL,
  "title" character varying NOT NULL,
  "description" text NOT NULL,
  "time_limit_ms" bigint NOT NULL DEFAULT 1000,
  "memory_limit_kb" bigint NOT NULL DEFAULT 262144,
  "is_published" boolean NOT NULL DEFAULT false,
  "submission_count" bigint NOT NULL DEFAULT 0,
  "accepted_count" bigint NOT NULL DEFAULT 0,
  "difficulty_id" uuid NOT NULL,
  "author_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "problem_draft_selection_complete" CHECK (((draft_revision_id IS NULL) AND (draft_render_source_id IS NULL) AND (draft_render_source_kind IS NULL) AND (draft_render_source_checksum IS NULL) AND (draft_render_artifact_id IS NULL)) OR ((draft_revision_id IS NOT NULL) AND (draft_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (draft_render_source_id IS NOT NULL) AND (draft_render_source_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((draft_render_source_kind)::text = 'problem_statement'::text) AND (draft_render_source_checksum IS NOT NULL) AND (octet_length(draft_render_source_checksum) = 32) AND (draft_render_source_checksum <> decode(repeat('00'::text, 32), 'hex'::text)) AND (draft_render_artifact_id IS NOT NULL) AND (draft_render_artifact_id <> '00000000-0000-0000-0000-000000000000'::uuid))),
  CONSTRAINT "problem_lifecycle_closed" CHECK ((lifecycle)::text = ANY ((ARRAY['draft'::character varying, 'published'::character varying, 'archived'::character varying])::text[])),
  CONSTRAINT "problem_lifecycle_selection" CHECK ((((lifecycle)::text = 'draft'::text) AND (published_revision_id IS NULL) AND (published_at IS NULL) AND (archived_at IS NULL)) OR (((lifecycle)::text = 'published'::text) AND (published_revision_id IS NOT NULL) AND (published_at IS NOT NULL) AND (archived_at IS NULL)) OR (((lifecycle)::text = 'archived'::text) AND (published_revision_id IS NOT NULL) AND (published_at IS NOT NULL) AND (archived_at IS NOT NULL) AND (archived_at >= published_at))),
  CONSTRAINT "problem_nonnegative_counts" CHECK ((submission_count >= 0) AND (accepted_count >= 0) AND (accepted_count <= submission_count)),
  CONSTRAINT "problem_nonzero_mapping_identities" CHECK ((tag_mapping_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (element_mapping_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (tag_mapping_id <> element_mapping_id)),
  CONSTRAINT "problem_positive_version" CHECK (version >= 1),
  CONSTRAINT "problem_public_visibility_matches_lifecycle" CHECK (is_published = ((lifecycle)::text = 'published'::text)),
  CONSTRAINT "problem_published_selection_complete" CHECK (((published_revision_id IS NULL) AND (published_render_source_id IS NULL) AND (published_render_source_kind IS NULL) AND (published_render_source_checksum IS NULL) AND (published_render_artifact_id IS NULL)) OR ((published_revision_id IS NOT NULL) AND (published_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (published_render_source_id IS NOT NULL) AND (published_render_source_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((published_render_source_kind)::text = 'problem_statement'::text) AND (published_render_source_checksum IS NOT NULL) AND (octet_length(published_render_source_checksum) = 32) AND (published_render_source_checksum <> decode(repeat('00'::text, 32), 'hex'::text)) AND (published_render_artifact_id IS NOT NULL) AND (published_render_artifact_id <> '00000000-0000-0000-0000-000000000000'::uuid))),
  CONSTRAINT "problem_slug_bound" CHECK (((octet_length((slug)::text) >= 1) AND (octet_length((slug)::text) <= 128)) AND ((slug)::text ~ '^[a-z0-9]+(-[a-z0-9]+)*$'::text))
);
-- Create index "problem_author_id_lifecycle_created_at_id" to table: "problems"
CREATE INDEX "problem_author_id_lifecycle_created_at_id" ON "problems" ("author_id", "lifecycle", "created_at", "id");
-- Create index "problem_draft_revision_id" to table: "problems"
CREATE INDEX "problem_draft_revision_id" ON "problems" ("draft_revision_id");
-- Create index "problem_id_element_mapping_id" to table: "problems"
CREATE UNIQUE INDEX "problem_id_element_mapping_id" ON "problems" ("id", "element_mapping_id");
-- Create index "problem_id_tag_mapping_id" to table: "problems"
CREATE UNIQUE INDEX "problem_id_tag_mapping_id" ON "problems" ("id", "tag_mapping_id");
-- Create index "problem_lifecycle_created_at_id" to table: "problems"
CREATE INDEX "problem_lifecycle_created_at_id" ON "problems" ("lifecycle", "created_at", "id");
-- Create index "problem_published_revision_id" to table: "problems"
CREATE INDEX "problem_published_revision_id" ON "problems" ("published_revision_id");
-- Create index "problems_slug_key" to table: "problems"
CREATE UNIQUE INDEX "problems_slug_key" ON "problems" ("slug");
-- Create "profile_schema_states" table
CREATE TABLE "profile_schema_states" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "key" character varying NOT NULL,
  "revision" bigint NOT NULL DEFAULT 1,
  PRIMARY KEY ("id"),
  CONSTRAINT "profile_schema_state_identity" CHECK (id <> '00000000-0000-0000-0000-000000000000'::uuid),
  CONSTRAINT "profile_schema_state_key" CHECK ((key)::text = 'onboarding'::text)
);
-- Create index "profileschemastate_key" to table: "profile_schema_states"
CREATE UNIQUE INDEX "profileschemastate_key" ON "profile_schema_states" ("key");
-- Create "progression_ledger_entries" table
CREATE TABLE "progression_ledger_entries" (
  "id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  "kind" character varying NOT NULL,
  "source_type" character varying NOT NULL,
  "source_aggregate_id" uuid NOT NULL,
  "source_aggregate_version" bigint NOT NULL,
  "source_transition_id" uuid NOT NULL,
  "source_event_id" uuid NOT NULL,
  "total_exp_delta" bigint NOT NULL,
  "balance_before" bigint NOT NULL,
  "balance_after" bigint NOT NULL,
  "progression_version" bigint NOT NULL,
  "element_deltas" jsonb NOT NULL,
  "difficulty_deltas" jsonb NOT NULL,
  "tag_deltas" jsonb NOT NULL,
  "source_checksum" bytea NOT NULL,
  "provenance" jsonb NOT NULL,
  "correlation_id" character varying(128) NOT NULL,
  "occurred_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "reversed_entry_id" uuid NULL,
  "reward_rule_revision_id" uuid NULL,
  "reward_profile_revision_id" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "progression_ledger_entries_progression_ledger_entries_reversal" FOREIGN KEY ("reversed_entry_id") REFERENCES "progression_ledger_entries" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "progression_ledger_balance" CHECK ((balance_before <= '9007199254740991'::bigint) AND (balance_after <= '9007199254740991'::bigint) AND ((balance_after - balance_before) = total_exp_delta)),
  CONSTRAINT "progression_ledger_json_shape" CHECK ((jsonb_typeof(element_deltas) = 'array'::text) AND (jsonb_typeof(difficulty_deltas) = 'array'::text) AND (jsonb_typeof(tag_deltas) = 'array'::text) AND (jsonb_typeof(provenance) = 'object'::text) AND (octet_length((element_deltas)::text) <= 65536) AND (octet_length((difficulty_deltas)::text) <= 65536) AND (octet_length((tag_deltas)::text) <= 65536) AND (octet_length((provenance)::text) <= 65536)),
  CONSTRAINT "progression_ledger_kind_shape" CHECK ((((kind)::text = ANY ((ARRAY['grant'::character varying, 'legacy_reconstructed'::character varying])::text[])) AND (reversed_entry_id IS NULL) AND (total_exp_delta > 0)) OR (((kind)::text = 'reversal'::text) AND (reversed_entry_id IS NOT NULL) AND (total_exp_delta < 0)) OR (((kind)::text = 'manual_adjustment'::text) AND (reversed_entry_id IS NULL) AND (total_exp_delta <> 0)) OR (((kind)::text = 'manual_reversal'::text) AND (reversed_entry_id IS NOT NULL) AND (total_exp_delta <> 0))),
  CONSTRAINT "progression_ledger_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (user_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (source_aggregate_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (source_transition_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (source_event_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((reversed_entry_id IS NULL) OR ((reversed_entry_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (reversed_entry_id <> id)))),
  CONSTRAINT "progression_ledger_rule_shape" CHECK ((((kind)::text = ANY ((ARRAY['grant'::character varying, 'reversal'::character varying])::text[])) AND (reward_rule_revision_id IS NOT NULL) AND (reward_profile_revision_id IS NOT NULL)) OR (((kind)::text <> ALL ((ARRAY['grant'::character varying, 'reversal'::character varying])::text[])) AND (reward_rule_revision_id IS NULL) AND (reward_profile_revision_id IS NULL))),
  CONSTRAINT "progression_ledger_source_checksum" CHECK ((octet_length(source_checksum) = 32) AND (source_checksum <> decode(repeat('00'::text, 32), 'hex'::text)))
);
-- Create index "progression_ledger_entries_reversed_entry_id_key" to table: "progression_ledger_entries"
CREATE UNIQUE INDEX "progression_ledger_entries_reversed_entry_id_key" ON "progression_ledger_entries" ("reversed_entry_id");
-- Create index "progressionledgerentry_id_source_aggregate_id_user_id" to table: "progression_ledger_entries"
CREATE UNIQUE INDEX "progressionledgerentry_id_source_aggregate_id_user_id" ON "progression_ledger_entries" ("id", "source_aggregate_id", "user_id");
-- Create index "progressionledgerentry_id_source_transition_id_source_event_id_" to table: "progression_ledger_entries"
CREATE UNIQUE INDEX "progressionledgerentry_id_source_transition_id_source_event_id_" ON "progression_ledger_entries" ("id", "source_transition_id", "source_event_id", "source_aggregate_id", "user_id", "source_aggregate_version", "kind");
-- Create index "progressionledgerentry_reversed_entry_id" to table: "progression_ledger_entries"
CREATE UNIQUE INDEX "progressionledgerentry_reversed_entry_id" ON "progression_ledger_entries" ("reversed_entry_id");
-- Create index "progressionledgerentry_source_event_id" to table: "progression_ledger_entries"
CREATE UNIQUE INDEX "progressionledgerentry_source_event_id" ON "progression_ledger_entries" ("source_event_id");
-- Create index "progressionledgerentry_source_transition_id" to table: "progression_ledger_entries"
CREATE UNIQUE INDEX "progressionledgerentry_source_transition_id" ON "progression_ledger_entries" ("source_transition_id");
-- Create index "progressionledgerentry_source_type_source_aggregate_id_occurred" to table: "progression_ledger_entries"
CREATE INDEX "progressionledgerentry_source_type_source_aggregate_id_occurred" ON "progression_ledger_entries" ("source_type", "source_aggregate_id", "occurred_at", "id");
-- Create index "progressionledgerentry_source_type_source_aggregate_id_source_a" to table: "progression_ledger_entries"
CREATE UNIQUE INDEX "progressionledgerentry_source_type_source_aggregate_id_source_a" ON "progression_ledger_entries" ("source_type", "source_aggregate_id", "source_aggregate_version");
-- Create index "progressionledgerentry_user_id_occurred_at_id" to table: "progression_ledger_entries"
CREATE INDEX "progressionledgerentry_user_id_occurred_at_id" ON "progression_ledger_entries" ("user_id", "occurred_at", "id");
-- Create index "progressionledgerentry_user_id_progression_version" to table: "progression_ledger_entries"
CREATE UNIQUE INDEX "progressionledgerentry_user_id_progression_version" ON "progression_ledger_entries" ("user_id", "progression_version");
-- Create "ranks" table
CREATE TABLE "ranks" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "name" character varying NOT NULL,
  "min_rating" bigint NOT NULL DEFAULT 0,
  "description" character varying NULL,
  PRIMARY KEY ("id")
);
-- Create index "ranks_name_key" to table: "ranks"
CREATE UNIQUE INDEX "ranks_name_key" ON "ranks" ("name");
-- Create "rarities" table
CREATE TABLE "rarities" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "name" character varying NOT NULL,
  "code" character varying NOT NULL,
  "weight" bigint NOT NULL DEFAULT 100,
  "description" character varying NULL,
  PRIMARY KEY ("id")
);
-- Create index "rarities_code_key" to table: "rarities"
CREATE UNIQUE INDEX "rarities_code_key" ON "rarities" ("code");
-- Create index "rarities_name_key" to table: "rarities"
CREATE UNIQUE INDEX "rarities_name_key" ON "rarities" ("name");
-- Create "rating_histories" table
CREATE TABLE "rating_histories" (
  "id" uuid NOT NULL,
  "rating_sequence" bigint NULL,
  "run_version" bigint NULL,
  "old_rating" bigint NOT NULL DEFAULT 1200,
  "new_rating" bigint NOT NULL DEFAULT 1200,
  "rank_position" bigint NOT NULL DEFAULT 0,
  "prior_rated_contest_count" bigint NOT NULL DEFAULT 0,
  "actual_rank" double precision NULL,
  "expected_rank" double precision NULL,
  "delta" bigint NOT NULL DEFAULT 0,
  "algorithm_version" character varying NULL,
  "contest_id" uuid NOT NULL,
  "rating_run_id" uuid NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "ratinghistory_contest_id" to table: "rating_histories"
CREATE INDEX "ratinghistory_contest_id" ON "rating_histories" ("contest_id");
-- Create index "ratinghistory_rating_run_id_user_id" to table: "rating_histories"
CREATE UNIQUE INDEX "ratinghistory_rating_run_id_user_id" ON "rating_histories" ("rating_run_id", "user_id");
-- Create index "ratinghistory_rating_sequence_user_id" to table: "rating_histories"
CREATE INDEX "ratinghistory_rating_sequence_user_id" ON "rating_histories" ("rating_sequence", "user_id");
-- Create index "ratinghistory_user_id" to table: "rating_histories"
CREATE INDEX "ratinghistory_user_id" ON "rating_histories" ("user_id");
-- Create index "ratinghistory_user_id_contest_id" to table: "rating_histories"
CREATE INDEX "ratinghistory_user_id_contest_id" ON "rating_histories" ("user_id", "contest_id");
-- Create "rating_projection_generations" table
CREATE TABLE "rating_projection_generations" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "status" character varying NOT NULL DEFAULT 'building',
  "based_on_projection_revision" bigint NOT NULL,
  "through_sequence" bigint NOT NULL DEFAULT 0,
  "checksum" bytea NULL,
  "verified_at" timestamptz NULL,
  "activated_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "rating_projection_checksum" CHECK ((checksum IS NULL) OR (octet_length(checksum) = 32)),
  CONSTRAINT "rating_projection_verified" CHECK ((((status)::text = 'building'::text) AND (checksum IS NULL) AND (verified_at IS NULL) AND (activated_at IS NULL)) OR (((status)::text = 'verified'::text) AND (checksum IS NOT NULL) AND (verified_at IS NOT NULL) AND (activated_at IS NULL)) OR (((status)::text = ANY ((ARRAY['active'::character varying, 'retired'::character varying])::text[])) AND (checksum IS NOT NULL) AND (verified_at IS NOT NULL) AND (activated_at IS NOT NULL)))
);
-- Create index "ratingprojectiongeneration_status_created_at_id" to table: "rating_projection_generations"
CREATE INDEX "ratingprojectiongeneration_status_created_at_id" ON "rating_projection_generations" ("status", "created_at", "id");
-- Create index "ratingprojectiongeneration_through_sequence_id" to table: "rating_projection_generations"
CREATE INDEX "ratingprojectiongeneration_through_sequence_id" ON "rating_projection_generations" ("through_sequence", "id");
-- Create "rating_run_chain_entries" table
CREATE TABLE "rating_run_chain_entries" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "rating_sequence" bigint NOT NULL,
  "rating_run_id" uuid NOT NULL,
  "chain_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "rating_run_chain_entries_check" CHECK ((chain_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (rating_run_id <> '00000000-0000-0000-0000-000000000000'::uuid))
);
-- Create index "ratingrunchainentry_chain_id_rating_run_id" to table: "rating_run_chain_entries"
CREATE UNIQUE INDEX "ratingrunchainentry_chain_id_rating_run_id" ON "rating_run_chain_entries" ("chain_id", "rating_run_id");
-- Create index "ratingrunchainentry_chain_id_rating_sequence" to table: "rating_run_chain_entries"
CREATE UNIQUE INDEX "ratingrunchainentry_chain_id_rating_sequence" ON "rating_run_chain_entries" ("chain_id", "rating_sequence");
-- Create index "ratingrunchainentry_rating_run_id" to table: "rating_run_chain_entries"
CREATE INDEX "ratingrunchainentry_rating_run_id" ON "rating_run_chain_entries" ("rating_run_id");
-- Create "rating_run_chains" table
CREATE TABLE "rating_run_chains" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "status" character varying NOT NULL DEFAULT 'building',
  "based_on_chain_id" uuid NULL,
  "through_sequence" bigint NOT NULL DEFAULT 0,
  "checksum" bytea NULL,
  "verified_at" timestamptz NULL,
  "activated_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "rating_chain_checksum" CHECK ((checksum IS NULL) OR (octet_length(checksum) = 32)),
  CONSTRAINT "rating_chain_verified" CHECK ((((status)::text = 'building'::text) AND (checksum IS NULL) AND (verified_at IS NULL) AND (activated_at IS NULL)) OR (((status)::text = 'verified'::text) AND (checksum IS NOT NULL) AND (verified_at IS NOT NULL) AND (activated_at IS NULL)) OR (((status)::text = ANY ((ARRAY['active'::character varying, 'retired'::character varying])::text[])) AND (checksum IS NOT NULL) AND (verified_at IS NOT NULL) AND (activated_at IS NOT NULL)))
);
-- Create index "ratingrunchain_based_on_chain_id" to table: "rating_run_chains"
CREATE INDEX "ratingrunchain_based_on_chain_id" ON "rating_run_chains" ("based_on_chain_id");
-- Create index "ratingrunchain_status_created_at_id" to table: "rating_run_chains"
CREATE INDEX "ratingrunchain_status_created_at_id" ON "rating_run_chains" ("status", "created_at", "id");
-- Create index "ratingrunchain_through_sequence_id" to table: "rating_run_chains"
CREATE INDEX "ratingrunchain_through_sequence_id" ON "rating_run_chains" ("through_sequence", "id");
-- Create "rating_sequence_slots" table
CREATE TABLE "rating_sequence_slots" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "rating_sequence" bigint NOT NULL,
  "effective_end_at" timestamptz NOT NULL,
  "assigned_at" timestamptz NOT NULL,
  "contest_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "rating_sequence_slots_contest_id_check" CHECK (contest_id <> '00000000-0000-0000-0000-000000000000'::uuid)
);
-- Create index "ratingsequenceslot_contest_id" to table: "rating_sequence_slots"
CREATE UNIQUE INDEX "ratingsequenceslot_contest_id" ON "rating_sequence_slots" ("contest_id");
-- Create index "ratingsequenceslot_effective_end_at_contest_id" to table: "rating_sequence_slots"
CREATE UNIQUE INDEX "ratingsequenceslot_effective_end_at_contest_id" ON "rating_sequence_slots" ("effective_end_at", "contest_id");
-- Create index "ratingsequenceslot_rating_sequence" to table: "rating_sequence_slots"
CREATE UNIQUE INDEX "ratingsequenceslot_rating_sequence" ON "rating_sequence_slots" ("rating_sequence");
-- Create "rating_states" table
CREATE TABLE "rating_states" (
  "id" character varying NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "next_sequence" bigint NOT NULL DEFAULT 1,
  "applied_sequence" bigint NOT NULL DEFAULT 0,
  "projection_revision" bigint NOT NULL DEFAULT 1,
  "active_projection_generation_id" uuid NOT NULL,
  "active_run_chain_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "rating_state_pointers" CHECK ((active_projection_generation_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (active_run_chain_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "rating_state_sequence" CHECK (next_sequence > applied_sequence),
  CONSTRAINT "rating_state_singleton" CHECK ((id)::text = 'global'::text)
);
-- Create "realtime_cursor_states" table
CREATE TABLE "realtime_cursor_states" (
  "id" character varying NOT NULL,
  "last_sequence" bigint NOT NULL DEFAULT 0,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id")
);
-- Create "realtime_events" table
CREATE TABLE "realtime_events" (
  "id" uuid NOT NULL,
  "event_id" character varying NOT NULL,
  "topic" character varying NOT NULL,
  "sequence" bigint NOT NULL,
  "type" character varying NOT NULL,
  "aggregate_version" bigint NOT NULL,
  "occurred_at" timestamptz NOT NULL,
  "data" jsonb NOT NULL,
  "delivery_class" character varying NOT NULL,
  "coalesce_key" character varying NULL,
  "evidence_hash" bytea NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "realtime_event_hash_length" CHECK (octet_length(evidence_hash) = 32),
  CONSTRAINT "realtime_event_sequence_positive" CHECK (sequence > 0)
);
-- Create index "realtimeevent_created_at_id" to table: "realtime_events"
CREATE INDEX "realtimeevent_created_at_id" ON "realtime_events" ("created_at", "id");
-- Create index "realtimeevent_created_at_sequence" to table: "realtime_events"
CREATE INDEX "realtimeevent_created_at_sequence" ON "realtime_events" ("created_at", "sequence");
-- Create index "realtimeevent_sequence" to table: "realtime_events"
CREATE UNIQUE INDEX "realtimeevent_sequence" ON "realtime_events" ("sequence");
-- Create index "realtimeevent_topic_event_id" to table: "realtime_events"
CREATE UNIQUE INDEX "realtimeevent_topic_event_id" ON "realtime_events" ("topic", "event_id");
-- Create index "realtimeevent_topic_sequence" to table: "realtime_events"
CREATE INDEX "realtimeevent_topic_sequence" ON "realtime_events" ("topic", "sequence");
-- Create "recovery_contact_command_receipts" table
CREATE TABLE "recovery_contact_command_receipts" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "idempotency_key" uuid NOT NULL,
  "kind" character varying NOT NULL,
  "request_fingerprint" bytea NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "session_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "recovery_contact_command_kind_closed" CHECK ((kind)::text = ANY ((ARRAY['replace'::character varying, 'resend'::character varying, 'cancel_pending'::character varying, 'remove'::character varying])::text[]))
);
-- Create index "recoverycontactcommandreceipt_expires_at" to table: "recovery_contact_command_receipts"
CREATE INDEX "recoverycontactcommandreceipt_expires_at" ON "recovery_contact_command_receipts" ("expires_at");
-- Create index "recoverycontactcommandreceipt_user_id_idempotency_key" to table: "recovery_contact_command_receipts"
CREATE UNIQUE INDEX "recoverycontactcommandreceipt_user_id_idempotency_key" ON "recovery_contact_command_receipts" ("user_id", "idempotency_key");
-- Create "recovery_contact_verification_tokens" table
CREATE TABLE "recovery_contact_verification_tokens" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "contact_version" bigint NOT NULL,
  "hash_key_id" character varying NOT NULL,
  "token_hash" bytea NOT NULL,
  "purpose" character varying NOT NULL,
  "status" character varying NOT NULL DEFAULT 'issued',
  "expires_at" timestamptz NOT NULL,
  "consumed_at" timestamptz NULL,
  "superseded_at" timestamptz NULL,
  "attempt_count" bigint NOT NULL DEFAULT 0,
  "last_attempt_at" timestamptz NULL,
  "contact_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "recovery_verification_purpose_closed" CHECK ((purpose)::text = ANY ((ARRAY['registration'::character varying, 'replacement'::character varying, 'invitation'::character varying])::text[])),
  CONSTRAINT "recovery_verification_status_closed" CHECK ((status)::text = ANY ((ARRAY['issued'::character varying, 'consumed'::character varying, 'superseded'::character varying, 'expired'::character varying])::text[]))
);
-- Create index "recoverycontactverificationtoken_contact_id_contact_version_sta" to table: "recovery_contact_verification_tokens"
CREATE INDEX "recoverycontactverificationtoken_contact_id_contact_version_sta" ON "recovery_contact_verification_tokens" ("contact_id", "contact_version", "status");
-- Create index "recoverycontactverificationtoken_hash_key_id_token_hash" to table: "recovery_contact_verification_tokens"
CREATE UNIQUE INDEX "recoverycontactverificationtoken_hash_key_id_token_hash" ON "recovery_contact_verification_tokens" ("hash_key_id", "token_hash");
-- Create index "recoverycontactverificationtoken_status_expires_at" to table: "recovery_contact_verification_tokens"
CREATE INDEX "recoverycontactverificationtoken_status_expires_at" ON "recovery_contact_verification_tokens" ("status", "expires_at");
-- Create "reward_rule_revisions" table
CREATE TABLE "reward_rule_revisions" (
  "id" uuid NOT NULL,
  "rule_key" character varying NOT NULL,
  "revision" bigint NOT NULL,
  "algorithm_version" character varying NOT NULL,
  "configuration" jsonb NOT NULL,
  "checksum" bytea NOT NULL,
  "request_checksum" bytea NOT NULL,
  "effective_from" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "reward_rule_revision_checksum" CHECK ((octet_length(checksum) = 32) AND (checksum <> decode(repeat('00'::text, 32), 'hex'::text)) AND (octet_length(request_checksum) = 32) AND (request_checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "reward_rule_revision_configuration" CHECK ((jsonb_typeof(configuration) = 'object'::text) AND (octet_length((configuration)::text) <= 16384)),
  CONSTRAINT "reward_rule_revision_identity" CHECK (id <> '00000000-0000-0000-0000-000000000000'::uuid)
);
-- Create index "rewardrulerevision_checksum" to table: "reward_rule_revisions"
CREATE UNIQUE INDEX "rewardrulerevision_checksum" ON "reward_rule_revisions" ("checksum");
-- Create index "rewardrulerevision_request_checksum" to table: "reward_rule_revisions"
CREATE UNIQUE INDEX "rewardrulerevision_request_checksum" ON "reward_rule_revisions" ("request_checksum");
-- Create index "rewardrulerevision_rule_key_effective_from" to table: "reward_rule_revisions"
CREATE INDEX "rewardrulerevision_rule_key_effective_from" ON "reward_rule_revisions" ("rule_key", "effective_from");
-- Create index "rewardrulerevision_rule_key_revision" to table: "reward_rule_revisions"
CREATE UNIQUE INDEX "rewardrulerevision_rule_key_revision" ON "reward_rule_revisions" ("rule_key", "revision");
-- Create "roles" table
CREATE TABLE "roles" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "key" character varying NOT NULL,
  "name" character varying NOT NULL,
  "description" character varying NULL,
  "is_system" boolean NOT NULL DEFAULT false,
  "status" character varying NOT NULL DEFAULT 'active',
  "version" bigint NOT NULL DEFAULT 1,
  PRIMARY KEY ("id"),
  CONSTRAINT "role_status_closed" CHECK ((status)::text = ANY ((ARRAY['active'::character varying, 'archived'::character varying])::text[]))
);
-- Create index "roles_key_key" to table: "roles"
CREATE UNIQUE INDEX "roles_key_key" ON "roles" ("key");
-- Create "runtime_profiles" table
CREATE TABLE "runtime_profiles" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "runtime_key" character varying(64) NOT NULL,
  "language" character varying(64) NOT NULL,
  "display_name" character varying(128) NOT NULL,
  "profile_version" character varying(128) NOT NULL,
  "compiler_version" character varying(128) NOT NULL,
  "runtime_bootstrap_version" character varying(128) NOT NULL,
  "sandbox_image_digest" character varying(71) NOT NULL,
  "compile_command_id" character varying(128) NOT NULL,
  "run_command_id" character varying(128) NOT NULL,
  "default_compile_timeout_ms" integer NOT NULL,
  "manifest_version" character varying(128) NOT NULL,
  "manifest_checksum" character varying(71) NOT NULL,
  "artifact_manifest" jsonb NOT NULL,
  "active" boolean NOT NULL DEFAULT false,
  PRIMARY KEY ("id"),
  CONSTRAINT "runtime_profile_artifact_manifest_bound" CHECK ((jsonb_typeof(artifact_manifest) = 'array'::text) AND (jsonb_array_length(artifact_manifest) > 0) AND (octet_length((artifact_manifest)::text) <= 32768)),
  CONSTRAINT "runtime_profile_command_ids_closed_shape" CHECK (((compile_command_id)::text ~ '^[a-z0-9][a-z0-9._-]*$'::text) AND ((run_command_id)::text ~ '^[a-z0-9][a-z0-9._-]*$'::text)),
  CONSTRAINT "runtime_profile_key_closed_shape" CHECK (((octet_length((runtime_key)::text) >= 1) AND (octet_length((runtime_key)::text) <= 64)) AND ((runtime_key)::text ~ '^[a-z0-9][a-z0-9._-]*$'::text)),
  CONSTRAINT "runtime_profile_pinned_digests" CHECK (((sandbox_image_digest)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((manifest_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text)),
  CONSTRAINT "runtime_profile_version_closed_shape" CHECK (((profile_version)::text ~ '^[a-z0-9][a-z0-9._-]*$'::text) AND ((manifest_version)::text ~ '^[a-z0-9][a-z0-9._-]*$'::text)),
  CONSTRAINT "runtime_profiles_default_compile_timeout_ms" CHECK ((default_compile_timeout_ms >= 1) AND (default_compile_timeout_ms <= 60000))
);
-- Create index "runtimeprofile_runtime_key_active_profile_version" to table: "runtime_profiles"
CREATE INDEX "runtimeprofile_runtime_key_active_profile_version" ON "runtime_profiles" ("runtime_key", "active", "profile_version");
-- Create index "runtimeprofile_runtime_key_profile_version" to table: "runtime_profiles"
CREATE UNIQUE INDEX "runtimeprofile_runtime_key_profile_version" ON "runtime_profiles" ("runtime_key", "profile_version");
-- Create index "runtimeprofile_sandbox_image_digest" to table: "runtime_profiles"
CREATE INDEX "runtimeprofile_sandbox_image_digest" ON "runtime_profiles" ("sandbox_image_digest");
-- Create "security_email_deliveries" table
CREATE TABLE "security_email_deliveries" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "kind" character varying NOT NULL,
  "provider_idempotency_key" character varying NOT NULL,
  "payload_ciphertext" bytea NOT NULL,
  "key_id" character varying NOT NULL,
  "payload_expires_at" timestamptz NOT NULL,
  "status" character varying NOT NULL DEFAULT 'ready',
  "attempt_count" bigint NOT NULL DEFAULT 0,
  "next_attempt_at" timestamptz NOT NULL,
  "lease_owner" character varying NULL,
  "lease_token" uuid NULL,
  "leased_until" timestamptz NULL,
  "sent_at" timestamptz NULL,
  "dead_at" timestamptz NULL,
  "cancelled_at" timestamptz NULL,
  "last_error_class" character varying NULL,
  "ciphertext_erased_at" timestamptz NULL,
  "password_reset_token_id" uuid NULL,
  "verification_token_id" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "security_email_intent_matches_kind" CHECK ((((kind)::text = ANY ((ARRAY['recovery_contact_verification'::character varying, 'account_invitation'::character varying])::text[])) AND (verification_token_id IS NOT NULL) AND (password_reset_token_id IS NULL)) OR (((kind)::text = 'password_reset'::text) AND (password_reset_token_id IS NOT NULL) AND (verification_token_id IS NULL))),
  CONSTRAINT "security_email_kind_closed" CHECK ((kind)::text = ANY ((ARRAY['recovery_contact_verification'::character varying, 'password_reset'::character varying, 'account_invitation'::character varying])::text[])),
  CONSTRAINT "security_email_lease_complete" CHECK (((lease_owner IS NULL) AND (lease_token IS NULL) AND (leased_until IS NULL)) OR ((lease_owner IS NOT NULL) AND (lease_token IS NOT NULL) AND (leased_until IS NOT NULL))),
  CONSTRAINT "security_email_state_shape" CHECK ((((status)::text = 'ready'::text) AND (lease_owner IS NULL) AND (lease_token IS NULL) AND (leased_until IS NULL) AND (sent_at IS NULL) AND (dead_at IS NULL) AND (cancelled_at IS NULL) AND (ciphertext_erased_at IS NULL) AND (octet_length(payload_ciphertext) > 1)) OR (((status)::text = 'leased'::text) AND (lease_owner IS NOT NULL) AND (lease_token IS NOT NULL) AND (leased_until IS NOT NULL) AND (attempt_count > 0) AND (sent_at IS NULL) AND (dead_at IS NULL) AND (cancelled_at IS NULL) AND (ciphertext_erased_at IS NULL) AND (octet_length(payload_ciphertext) > 1)) OR (((status)::text = 'sent'::text) AND (lease_owner IS NULL) AND (lease_token IS NULL) AND (leased_until IS NULL) AND (sent_at IS NOT NULL) AND (dead_at IS NULL) AND (cancelled_at IS NULL) AND (ciphertext_erased_at IS NOT NULL) AND (octet_length(payload_ciphertext) = 1)) OR (((status)::text = 'dead'::text) AND (lease_owner IS NULL) AND (lease_token IS NULL) AND (leased_until IS NULL) AND (sent_at IS NULL) AND (dead_at IS NOT NULL) AND (cancelled_at IS NULL) AND (ciphertext_erased_at IS NOT NULL) AND (octet_length(payload_ciphertext) = 1)) OR (((status)::text = 'cancelled'::text) AND (lease_owner IS NULL) AND (lease_token IS NULL) AND (leased_until IS NULL) AND (sent_at IS NULL) AND (dead_at IS NULL) AND (cancelled_at IS NOT NULL) AND (ciphertext_erased_at IS NOT NULL) AND (octet_length(payload_ciphertext) = 1))),
  CONSTRAINT "security_email_status_closed" CHECK ((status)::text = ANY ((ARRAY['ready'::character varying, 'leased'::character varying, 'sent'::character varying, 'dead'::character varying, 'cancelled'::character varying])::text[])),
  CONSTRAINT "security_email_terminal_timestamp" CHECK ((((status)::text = 'sent'::text) AND (sent_at IS NOT NULL)) OR (((status)::text = 'dead'::text) AND (dead_at IS NOT NULL)) OR (((status)::text = 'cancelled'::text) AND (cancelled_at IS NOT NULL)) OR ((status)::text = ANY ((ARRAY['ready'::character varying, 'leased'::character varying])::text[])))
);
-- Create index "security_email_deliveries_provider_idempotency_key_key" to table: "security_email_deliveries"
CREATE UNIQUE INDEX "security_email_deliveries_provider_idempotency_key_key" ON "security_email_deliveries" ("provider_idempotency_key");
-- Create index "securityemaildelivery_password_reset_token_id" to table: "security_email_deliveries"
CREATE INDEX "securityemaildelivery_password_reset_token_id" ON "security_email_deliveries" ("password_reset_token_id");
-- Create index "securityemaildelivery_payload_expires_at" to table: "security_email_deliveries"
CREATE INDEX "securityemaildelivery_payload_expires_at" ON "security_email_deliveries" ("payload_expires_at");
-- Create index "securityemaildelivery_status_leased_until" to table: "security_email_deliveries"
CREATE INDEX "securityemaildelivery_status_leased_until" ON "security_email_deliveries" ("status", "leased_until");
-- Create index "securityemaildelivery_status_next_attempt_at_created_at_id" to table: "security_email_deliveries"
CREATE INDEX "securityemaildelivery_status_next_attempt_at_created_at_id" ON "security_email_deliveries" ("status", "next_attempt_at", "created_at", "id");
-- Create index "securityemaildelivery_verification_token_id" to table: "security_email_deliveries"
CREATE INDEX "securityemaildelivery_verification_token_id" ON "security_email_deliveries" ("verification_token_id");
-- Create "solve_reward_cursors" table
CREATE TABLE "solve_reward_cursors" (
  "id" uuid NOT NULL,
  "solve_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  "problem_id" uuid NOT NULL,
  "last_applied_version" bigint NOT NULL DEFAULT 0,
  "current_status" character varying NOT NULL DEFAULT 'unsolved',
  "active_ledger_entry_id" uuid NULL,
  "last_transition_id" uuid NULL,
  "last_event_id" uuid NULL,
  "last_transition_hash" bytea NULL,
  "reconciliation_state" character varying NOT NULL DEFAULT 'ready',
  "missing_from_version" bigint NULL,
  "missing_to_version" bigint NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "solve_reward_cursor_active_entry" CHECK ((((current_status)::text = 'unsolved'::text) AND (active_ledger_entry_id IS NULL)) OR (((current_status)::text = 'solved'::text) AND (active_ledger_entry_id IS NOT NULL))),
  CONSTRAINT "solve_reward_cursor_gap" CHECK ((((reconciliation_state)::text = 'ready'::text) AND (missing_from_version IS NULL) AND (missing_to_version IS NULL)) OR (((reconciliation_state)::text = 'blocked_gap'::text) AND (missing_from_version = (last_applied_version + 1)) AND (missing_to_version >= missing_from_version))),
  CONSTRAINT "solve_reward_cursor_last_identity" CHECK (((last_applied_version = 0) AND (last_transition_id IS NULL) AND (last_event_id IS NULL) AND (last_transition_hash IS NULL)) OR ((last_applied_version > 0) AND (last_transition_id IS NOT NULL) AND (last_event_id IS NOT NULL) AND (octet_length(last_transition_hash) = 32) AND (last_transition_hash <> decode(repeat('00'::text, 32), 'hex'::text)))),
  CONSTRAINT "solve_reward_cursor_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (solve_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (user_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (problem_id <> '00000000-0000-0000-0000-000000000000'::uuid))
);
-- Create index "solverewardcursor_active_ledger_entry_id" to table: "solve_reward_cursors"
CREATE INDEX "solverewardcursor_active_ledger_entry_id" ON "solve_reward_cursors" ("active_ledger_entry_id");
-- Create index "solverewardcursor_last_event_id" to table: "solve_reward_cursors"
CREATE INDEX "solverewardcursor_last_event_id" ON "solve_reward_cursors" ("last_event_id");
-- Create index "solverewardcursor_reconciliation_state_updated_at_solve_id" to table: "solve_reward_cursors"
CREATE INDEX "solverewardcursor_reconciliation_state_updated_at_solve_id" ON "solve_reward_cursors" ("reconciliation_state", "updated_at", "solve_id");
-- Create index "solverewardcursor_solve_id" to table: "solve_reward_cursors"
CREATE UNIQUE INDEX "solverewardcursor_solve_id" ON "solve_reward_cursors" ("solve_id");
-- Create index "solverewardcursor_user_id_problem_id" to table: "solve_reward_cursors"
CREATE UNIQUE INDEX "solverewardcursor_user_id_problem_id" ON "solve_reward_cursors" ("user_id", "problem_id");
-- Create "solve_reward_processed_transitions" table
CREATE TABLE "solve_reward_processed_transitions" (
  "id" uuid NOT NULL,
  "solve_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  "problem_id" uuid NOT NULL,
  "aggregate_version" bigint NOT NULL,
  "transition_id" uuid NOT NULL,
  "event_id" uuid NOT NULL,
  "transition_hash" bytea NOT NULL,
  "outcome" character varying NOT NULL,
  "ledger_entry_id" uuid NULL,
  "correlation_id" character varying(128) NOT NULL,
  "occurred_at" timestamptz NOT NULL,
  "processed_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "solve_reward_processed_hash" CHECK ((octet_length(transition_hash) = 32) AND (transition_hash <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "solve_reward_processed_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (solve_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (user_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (problem_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (transition_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (event_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "solve_reward_processed_outcome" CHECK ((((outcome)::text = ANY ((ARRAY['grant'::character varying, 'reversal'::character varying])::text[])) AND (ledger_entry_id IS NOT NULL)) OR (((outcome)::text = 'pointer_move'::text) AND (ledger_entry_id IS NULL)))
);
-- Create index "solverewardprocessedtransition_event_id" to table: "solve_reward_processed_transitions"
CREATE UNIQUE INDEX "solverewardprocessedtransition_event_id" ON "solve_reward_processed_transitions" ("event_id");
-- Create index "solverewardprocessedtransition_ledger_entry_id" to table: "solve_reward_processed_transitions"
CREATE INDEX "solverewardprocessedtransition_ledger_entry_id" ON "solve_reward_processed_transitions" ("ledger_entry_id");
-- Create index "solverewardprocessedtransition_solve_id_aggregate_version" to table: "solve_reward_processed_transitions"
CREATE UNIQUE INDEX "solverewardprocessedtransition_solve_id_aggregate_version" ON "solve_reward_processed_transitions" ("solve_id", "aggregate_version");
-- Create index "solverewardprocessedtransition_transition_id" to table: "solve_reward_processed_transitions"
CREATE UNIQUE INDEX "solverewardprocessedtransition_transition_id" ON "solve_reward_processed_transitions" ("transition_id");
-- Create index "solverewardprocessedtransition_user_id_processed_at_id" to table: "solve_reward_processed_transitions"
CREATE INDEX "solverewardprocessedtransition_user_id_processed_at_id" ON "solve_reward_processed_transitions" ("user_id", "processed_at", "id");
-- Create "submission_judgements" table
CREATE TABLE "submission_judgements" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "submission_id" uuid NOT NULL,
  "generation" bigint NOT NULL,
  "judge_spec_id" uuid NOT NULL,
  "problem_revision_id" uuid NOT NULL,
  "testset_revision_id" uuid NOT NULL,
  "verdict" character varying NOT NULL,
  "passed_count" bigint NOT NULL,
  "total_count" bigint NOT NULL,
  "maximum_time_ms" bigint NULL,
  "maximum_memory_kb" bigint NULL,
  "compile_diagnostic_summary" character varying NULL,
  "judge_spec_checksum" bytea NOT NULL,
  "reward_snapshot" jsonb NOT NULL,
  "reward_snapshot_checksum" bytea NOT NULL,
  "runtime_profile_version" character varying(128) NOT NULL,
  "compiler_version" character varying(128) NOT NULL,
  "runtime_bootstrap_version" character varying(128) NOT NULL,
  "supervisor_protocol_version" character varying(128) NOT NULL,
  "sandbox_image_digest" character varying(71) NOT NULL,
  "artifact_manifest_checksum" character varying(71) NOT NULL,
  "release_id" character varying(128) NOT NULL,
  "approved_release_manifest_checksum" character varying(71) NOT NULL,
  "checker_implementation_checksum" character varying(71) NOT NULL,
  "winning_attempt_id" uuid NULL,
  "completed_at" timestamptz NOT NULL,
  "origin_cid" character varying(128) NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "submission_judgement_accepted_counts" CHECK (((verdict)::text <> 'accepted'::text) OR ((total_count > 0) AND (passed_count = total_count))),
  CONSTRAINT "submission_judgement_attempt_complete" CHECK (((verdict)::text = 'system_error'::text) OR (winning_attempt_id IS NOT NULL)),
  CONSTRAINT "submission_judgement_checksum_shapes" CHECK ((octet_length(judge_spec_checksum) = 32) AND (judge_spec_checksum <> decode(repeat('00'::text, 32), 'hex'::text)) AND (octet_length(reward_snapshot_checksum) = 32) AND (reward_snapshot_checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "submission_judgement_count_bounds" CHECK (((passed_count >= 0) AND (passed_count <= total_count)) AND ((total_count >= 0) AND (total_count <= 256))),
  CONSTRAINT "submission_judgement_diagnostic_bound" CHECK ((compile_diagnostic_summary IS NULL) OR ((octet_length((compile_diagnostic_summary)::text) >= 1) AND (octet_length((compile_diagnostic_summary)::text) <= 16384))),
  CONSTRAINT "submission_judgement_generation_bound" CHECK (generation >= 1),
  CONSTRAINT "submission_judgement_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (submission_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (judge_spec_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (problem_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (testset_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((winning_attempt_id IS NULL) OR (winning_attempt_id <> '00000000-0000-0000-0000-000000000000'::uuid))),
  CONSTRAINT "submission_judgement_pinned_digests" CHECK (((sandbox_image_digest)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((artifact_manifest_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((approved_release_manifest_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text) AND ((checker_implementation_checksum)::text ~ '^sha256:[0-9a-f]{64}$'::text)),
  CONSTRAINT "submission_judgement_resource_bounds" CHECK (((maximum_time_ms IS NULL) OR ((maximum_time_ms >= 0) AND (maximum_time_ms <= 86400000))) AND ((maximum_memory_kb IS NULL) OR ((maximum_memory_kb >= 0) AND (maximum_memory_kb <= 2097152)))),
  CONSTRAINT "submission_judgement_reward_bound" CHECK ((jsonb_typeof(reward_snapshot) = 'object'::text) AND (octet_length((reward_snapshot)::text) <= 65536)),
  CONSTRAINT "submission_judgement_verdict_closed" CHECK ((verdict)::text = ANY ((ARRAY['accepted'::character varying, 'wrong_answer'::character varying, 'compile_error'::character varying, 'runtime_error'::character varying, 'time_limit_exceeded'::character varying, 'memory_limit_exceeded'::character varying, 'process_limit_exceeded'::character varying, 'output_limit_exceeded'::character varying, 'system_error'::character varying])::text[]))
);
-- Create index "judgement_id_submission_id_generation_problem_revision_id" to table: "submission_judgements"
CREATE UNIQUE INDEX "judgement_id_submission_id_generation_problem_revision_id" ON "submission_judgements" ("id", "submission_id", "generation", "problem_revision_id");
-- Create index "judgement_id_submission_id_generation_verdict" to table: "submission_judgements"
CREATE UNIQUE INDEX "judgement_id_submission_id_generation_verdict" ON "submission_judgements" ("id", "submission_id", "generation", "verdict");
-- Create index "judgement_judge_spec_id" to table: "submission_judgements"
CREATE UNIQUE INDEX "judgement_judge_spec_id" ON "submission_judgements" ("judge_spec_id");
-- Create index "judgement_submission_id_completed_at_id" to table: "submission_judgements"
CREATE INDEX "judgement_submission_id_completed_at_id" ON "submission_judgements" ("submission_id", "completed_at", "id");
-- Create index "judgement_submission_id_generation" to table: "submission_judgements"
CREATE UNIQUE INDEX "judgement_submission_id_generation" ON "submission_judgements" ("submission_id", "generation");
-- Create index "judgement_verdict_completed_at_id" to table: "submission_judgements"
CREATE INDEX "judgement_verdict_completed_at_id" ON "submission_judgements" ("verdict", "completed_at", "id");
-- Create index "judgement_winning_attempt_id" to table: "submission_judgements"
CREATE UNIQUE INDEX "judgement_winning_attempt_id" ON "submission_judgements" ("winning_attempt_id");
-- Create index "submission_judgements_reward_identity" to table: "submission_judgements"
CREATE UNIQUE INDEX "submission_judgements_reward_identity" ON "submission_judgements" ("id", "submission_id", "generation", "problem_revision_id", "reward_snapshot_checksum");
-- Create "submissions" table
CREATE TABLE "submissions" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "contest_problem_id" uuid NULL,
  "contest_scoring_policy_version" bigint NULL,
  "submitted_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "contest_elapsed_ms" bigint NULL,
  "contest_submission_sequence" bigint NULL,
  "language" character varying NOT NULL,
  "runtime_key" character varying(64) NOT NULL,
  "source_code" text NOT NULL,
  "source_sha256" bytea NOT NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "latest_generation" bigint NOT NULL DEFAULT 1,
  "latest_generation_phase" character varying NOT NULL DEFAULT 'queued',
  "latest_generation_outcome" character varying NULL,
  "active_judgement_id" uuid NULL,
  "active_judgement_generation" bigint NULL,
  "active_verdict" character varying NULL,
  "idempotency_key" character varying(128) NOT NULL,
  "request_hash" bytea NOT NULL,
  "origin_cid" character varying(128) NOT NULL,
  "latest_generation_queued_at" timestamptz NOT NULL,
  "latest_generation_started_at" timestamptz NULL,
  "latest_generation_completed_at" timestamptz NULL,
  "status" character varying NOT NULL DEFAULT 'pending',
  "judged_at" timestamptz NULL,
  "verdict_version" bigint NOT NULL DEFAULT 0,
  "passed_count" bigint NOT NULL DEFAULT 0,
  "total_count" bigint NOT NULL DEFAULT 0,
  "time_ms" bigint NULL,
  "memory_kb" bigint NULL,
  "error_message" text NULL,
  "contest_id" uuid NULL,
  "problem_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "submission_active_judgement_complete" CHECK (((active_judgement_id IS NULL) AND (active_judgement_generation IS NULL) AND (active_verdict IS NULL)) OR ((active_judgement_id IS NOT NULL) AND ((active_judgement_generation >= 1) AND (active_judgement_generation <= latest_generation)) AND ((active_verdict)::text = ANY ((ARRAY['accepted'::character varying, 'wrong_answer'::character varying, 'compile_error'::character varying, 'runtime_error'::character varying, 'time_limit_exceeded'::character varying, 'memory_limit_exceeded'::character varying, 'process_limit_exceeded'::character varying, 'output_limit_exceeded'::character varying])::text[])))),
  CONSTRAINT "submission_contest_snapshot_complete" CHECK (((contest_id IS NULL) AND (contest_problem_id IS NULL) AND (contest_scoring_policy_version IS NULL) AND (contest_elapsed_ms IS NULL) AND (contest_submission_sequence IS NULL)) OR ((contest_id IS NOT NULL) AND (contest_problem_id IS NOT NULL) AND (contest_scoring_policy_version >= 1) AND (contest_elapsed_ms >= 0) AND (contest_submission_sequence >= 1))),
  CONSTRAINT "submission_generation_and_version_bounds" CHECK ((latest_generation >= 1) AND (version >= 1)),
  CONSTRAINT "submission_generation_timestamps_ordered" CHECK ((latest_generation_queued_at >= submitted_at) AND ((latest_generation_started_at IS NULL) OR (latest_generation_started_at >= latest_generation_queued_at)) AND ((latest_generation_completed_at IS NULL) OR (latest_generation_completed_at >= latest_generation_queued_at))),
  CONSTRAINT "submission_idempotency_key_bound" CHECK (((octet_length((idempotency_key)::text) >= 1) AND (octet_length((idempotency_key)::text) <= 128)) AND ((idempotency_key)::text ~ '^[ -~]+$'::text)),
  CONSTRAINT "submission_latest_generation_state" CHECK ((((latest_generation_phase)::text = ANY ((ARRAY['queued'::character varying, 'claimed'::character varying, 'compiling'::character varying, 'running'::character varying, 'finalizing'::character varying])::text[])) AND (latest_generation_outcome IS NULL) AND (latest_generation_completed_at IS NULL)) OR (((latest_generation_phase)::text = 'terminal'::text) AND ((latest_generation_outcome)::text = ANY ((ARRAY['accepted'::character varying, 'wrong_answer'::character varying, 'compile_error'::character varying, 'runtime_error'::character varying, 'time_limit_exceeded'::character varying, 'memory_limit_exceeded'::character varying, 'process_limit_exceeded'::character varying, 'output_limit_exceeded'::character varying, 'system_error'::character varying])::text[])) AND (latest_generation_completed_at IS NOT NULL)) OR (((latest_generation_phase)::text = 'cancelled'::text) AND ((latest_generation_outcome)::text = 'cancelled'::text) AND (latest_generation_completed_at IS NOT NULL))),
  CONSTRAINT "submission_nonzero_durable_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (user_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (problem_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((active_judgement_id IS NULL) OR (active_judgement_id <> '00000000-0000-0000-0000-000000000000'::uuid))),
  CONSTRAINT "submission_origin_cid_bound" CHECK ((octet_length((origin_cid)::text) >= 1) AND (octet_length((origin_cid)::text) <= 128)),
  CONSTRAINT "submission_runtime_key_bound" CHECK (((octet_length((runtime_key)::text) >= 1) AND (octet_length((runtime_key)::text) <= 64)) AND ((runtime_key)::text ~ '^[a-z0-9][a-z0-9._-]*$'::text)),
  CONSTRAINT "submission_source_and_request_hashes" CHECK ((octet_length(source_sha256) = 32) AND (source_sha256 <> decode(repeat('00'::text, 32), 'hex'::text)) AND (octet_length(request_hash) = 32) AND (request_hash <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "submission_source_code_bytes" CHECK ((octet_length(source_code) >= 1) AND (octet_length(source_code) <= 262144)),
  CONSTRAINT "submission_status_closed" CHECK ((status)::text = ANY ((ARRAY['pending'::character varying, 'judging'::character varying, 'accepted'::character varying, 'wrong_answer'::character varying, 'time_limit_exceeded'::character varying, 'memory_limit_exceeded'::character varying, 'process_limit_exceeded'::character varying, 'output_limit_exceeded'::character varying, 'runtime_error'::character varying, 'compile_error'::character varying, 'internal_error'::character varying])::text[])),
  CONSTRAINT "submission_verdict_complete" CHECK ((((status)::text = ANY ((ARRAY['pending'::character varying, 'judging'::character varying])::text[])) AND (judged_at IS NULL) AND (verdict_version = 0)) OR (((status)::text <> ALL ((ARRAY['pending'::character varying, 'judging'::character varying])::text[])) AND (judged_at IS NOT NULL) AND (verdict_version > 0)))
);
-- Create index "submission_active_judgement_id" to table: "submissions"
CREATE INDEX "submission_active_judgement_id" ON "submissions" ("active_judgement_id");
-- Create index "submission_contest_id" to table: "submissions"
CREATE INDEX "submission_contest_id" ON "submissions" ("contest_id");
-- Create index "submission_contest_id_contest_submission_sequence" to table: "submissions"
CREATE UNIQUE INDEX "submission_contest_id_contest_submission_sequence" ON "submissions" ("contest_id", "contest_submission_sequence");
-- Create index "submission_contest_id_user_id_problem_id_contest_submission_seq" to table: "submissions"
CREATE INDEX "submission_contest_id_user_id_problem_id_contest_submission_seq" ON "submissions" ("contest_id", "user_id", "problem_id", "contest_submission_sequence");
-- Create index "submission_id_user_id_problem_id" to table: "submissions"
CREATE UNIQUE INDEX "submission_id_user_id_problem_id" ON "submissions" ("id", "user_id", "problem_id");
-- Create index "submission_latest_generation_phase_latest_generation_queued_at_" to table: "submissions"
CREATE INDEX "submission_latest_generation_phase_latest_generation_queued_at_" ON "submissions" ("latest_generation_phase", "latest_generation_queued_at", "id");
-- Create index "submission_problem_id" to table: "submissions"
CREATE INDEX "submission_problem_id" ON "submissions" ("problem_id");
-- Create index "submission_problem_id_created_at_id" to table: "submissions"
CREATE INDEX "submission_problem_id_created_at_id" ON "submissions" ("problem_id", "created_at", "id");
-- Create index "submission_status_created_at_id" to table: "submissions"
CREATE INDEX "submission_status_created_at_id" ON "submissions" ("status", "created_at", "id");
-- Create index "submission_user_id" to table: "submissions"
CREATE INDEX "submission_user_id" ON "submissions" ("user_id");
-- Create index "submission_user_id_created_at_id" to table: "submissions"
CREATE INDEX "submission_user_id_created_at_id" ON "submissions" ("user_id", "created_at", "id");
-- Create index "submission_user_id_idempotency_key" to table: "submissions"
CREATE UNIQUE INDEX "submission_user_id_idempotency_key" ON "submissions" ("user_id", "idempotency_key");
-- Create index "submission_user_id_problem_id_contest_id_created_at_id" to table: "submissions"
CREATE INDEX "submission_user_id_problem_id_contest_id_created_at_id" ON "submissions" ("user_id", "problem_id", "contest_id", "created_at", "id");
-- Create index "submission_user_id_problem_id_created_at_id" to table: "submissions"
CREATE INDEX "submission_user_id_problem_id_created_at_id" ON "submissions" ("user_id", "problem_id", "created_at", "id");
-- Create "tag_elements" table
CREATE TABLE "tag_elements" (
  "tag_id" uuid NOT NULL,
  "element_id" uuid NOT NULL,
  PRIMARY KEY ("tag_id", "element_id")
);
-- Create "tag_revision_elements" table
CREATE TABLE "tag_revision_elements" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "element_revision" bigint NOT NULL,
  "element_id" uuid NOT NULL,
  "tag_revision_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "tag_revision_element_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (tag_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (element_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "tag_revision_element_revision_bound" CHECK ((element_revision >= 1) AND (element_revision <= 1000000000))
);
-- Create index "tagrevisionelement_element_id_element_revision" to table: "tag_revision_elements"
CREATE INDEX "tagrevisionelement_element_id_element_revision" ON "tag_revision_elements" ("element_id", "element_revision");
-- Create index "tagrevisionelement_tag_revision_id_element_id" to table: "tag_revision_elements"
CREATE UNIQUE INDEX "tagrevisionelement_tag_revision_id_element_id" ON "tag_revision_elements" ("tag_revision_id", "element_id");
-- Create "tag_revisions" table
CREATE TABLE "tag_revisions" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "revision" bigint NOT NULL,
  "name" character varying NOT NULL,
  "checksum" bytea NOT NULL,
  "tag_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "tag_revision_checksum_shape" CHECK ((octet_length(checksum) = 32) AND (checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "tag_revision_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (tag_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "tag_revision_number_bound" CHECK ((revision >= 1) AND (revision <= 1000000000))
);
-- Create index "tagrevision_tag_id_created_at_id" to table: "tag_revisions"
CREATE INDEX "tagrevision_tag_id_created_at_id" ON "tag_revisions" ("tag_id", "created_at", "id");
-- Create index "tagrevision_tag_id_revision" to table: "tag_revisions"
CREATE UNIQUE INDEX "tagrevision_tag_id_revision" ON "tag_revisions" ("tag_id", "revision");
-- Create "tags" table
CREATE TABLE "tags" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "name" character varying NOT NULL,
  "version" bigint NOT NULL DEFAULT 1,
  PRIMARY KEY ("id"),
  CONSTRAINT "tag_positive_version" CHECK (version >= 1)
);
-- Create index "tags_name_key" to table: "tags"
CREATE UNIQUE INDEX "tags_name_key" ON "tags" ("name");
-- Create "test_assets" table
CREATE TABLE "test_assets" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "sha256" bytea NOT NULL,
  "encoding" character varying NOT NULL,
  "content" bytea NOT NULL,
  "uncompressed_size" bigint NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "test_asset_checksum_shape" CHECK ((octet_length(sha256) = 32) AND (sha256 <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "test_asset_content_bound" CHECK ((octet_length(content) >= 0) AND (octet_length(content) <= 1048576)),
  CONSTRAINT "test_asset_encoding_closed" CHECK ((encoding)::text = ANY ((ARRAY['identity'::character varying, 'zstd'::character varying])::text[])),
  CONSTRAINT "test_asset_encoding_size_consistent" CHECK ((((encoding)::text = 'identity'::text) AND (uncompressed_size = octet_length(content))) OR (((encoding)::text = 'zstd'::text) AND ((octet_length(content) >= 1) AND (octet_length(content) <= 1048576)) AND ((uncompressed_size >= 0) AND (uncompressed_size <= 1048576)))),
  CONSTRAINT "test_asset_nonzero_identity" CHECK (id <> '00000000-0000-0000-0000-000000000000'::uuid)
);
-- Create index "testasset_created_at_id" to table: "test_assets"
CREATE INDEX "testasset_created_at_id" ON "test_assets" ("created_at", "id");
-- Create index "testasset_sha256" to table: "test_assets"
CREATE UNIQUE INDEX "testasset_sha256" ON "test_assets" ("sha256");
-- Create "test_cases" table
CREATE TABLE "test_cases" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "problem_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "test_case_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (problem_id <> '00000000-0000-0000-0000-000000000000'::uuid))
);
-- Create index "testcase_id_problem_id" to table: "test_cases"
CREATE UNIQUE INDEX "testcase_id_problem_id" ON "test_cases" ("id", "problem_id");
-- Create "testcase_groups" table
CREATE TABLE "testcase_groups" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "group_index" bigint NOT NULL,
  "stop_on_failure" boolean NOT NULL DEFAULT false,
  "testset_revision_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "testcase_group_index_bound" CHECK ((group_index >= 0) AND (group_index <= 63)),
  CONSTRAINT "testcase_group_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (testset_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid))
);
-- Create index "testcasegroup_id_testset_revision_id_group_index_stop_on_failur" to table: "testcase_groups"
CREATE UNIQUE INDEX "testcasegroup_id_testset_revision_id_group_index_stop_on_failur" ON "testcase_groups" ("id", "testset_revision_id", "group_index", "stop_on_failure");
-- Create index "testcasegroup_testset_revision_id_created_at_id" to table: "testcase_groups"
CREATE INDEX "testcasegroup_testset_revision_id_created_at_id" ON "testcase_groups" ("testset_revision_id", "created_at", "id");
-- Create index "testcasegroup_testset_revision_id_group_index" to table: "testcase_groups"
CREATE UNIQUE INDEX "testcasegroup_testset_revision_id_group_index" ON "testcase_groups" ("testset_revision_id", "group_index");
-- Create index "testcasegroup_testset_revision_id_group_index_stop_on_failure" to table: "testcase_groups"
CREATE UNIQUE INDEX "testcasegroup_testset_revision_id_group_index_stop_on_failure" ON "testcase_groups" ("testset_revision_id", "group_index", "stop_on_failure");
-- Create "testcase_revisions" table
CREATE TABLE "testcase_revisions" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "problem_id" uuid NOT NULL,
  "kind" character varying NOT NULL,
  "group_index" bigint NOT NULL,
  "order_index" bigint NOT NULL,
  "weight" bigint NOT NULL DEFAULT 1,
  "stop_on_failure" boolean NOT NULL DEFAULT false,
  "input_asset_id" uuid NOT NULL,
  "expected_asset_id" uuid NOT NULL,
  "test_case_id" uuid NOT NULL,
  "group_id" uuid NOT NULL,
  "testset_revision_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "testcase_revision_group_index_bound" CHECK ((group_index >= 0) AND (group_index <= 63)),
  CONSTRAINT "testcase_revision_kind_closed" CHECK ((kind)::text = ANY ((ARRAY['sample'::character varying, 'hidden'::character varying])::text[])),
  CONSTRAINT "testcase_revision_launch_weight" CHECK (weight = 1),
  CONSTRAINT "testcase_revision_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (test_case_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (problem_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (testset_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (group_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (input_asset_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (expected_asset_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "testcase_revision_order_index_bound" CHECK ((order_index >= 0) AND (order_index <= 255))
);
-- Create index "testcase_revisions_judge_result_identity" to table: "testcase_revisions"
CREATE UNIQUE INDEX "testcase_revisions_judge_result_identity" ON "testcase_revisions" ("id", "testset_revision_id", "group_id", "group_index", "order_index", "kind", "stop_on_failure");
-- Create index "testcaserevision_expected_asset_id" to table: "testcase_revisions"
CREATE INDEX "testcaserevision_expected_asset_id" ON "testcase_revisions" ("expected_asset_id");
-- Create index "testcaserevision_group_id_order_index" to table: "testcase_revisions"
CREATE UNIQUE INDEX "testcaserevision_group_id_order_index" ON "testcase_revisions" ("group_id", "order_index");
-- Create index "testcaserevision_group_id_testset_revision_id_group_index_stop_" to table: "testcase_revisions"
CREATE INDEX "testcaserevision_group_id_testset_revision_id_group_index_stop_" ON "testcase_revisions" ("group_id", "testset_revision_id", "group_index", "stop_on_failure");
-- Create index "testcaserevision_input_asset_id" to table: "testcase_revisions"
CREATE INDEX "testcaserevision_input_asset_id" ON "testcase_revisions" ("input_asset_id");
-- Create index "testcaserevision_test_case_id_problem_id" to table: "testcase_revisions"
CREATE INDEX "testcaserevision_test_case_id_problem_id" ON "testcase_revisions" ("test_case_id", "problem_id");
-- Create index "testcaserevision_testset_revision_id_group_index_order_index" to table: "testcase_revisions"
CREATE UNIQUE INDEX "testcaserevision_testset_revision_id_group_index_order_index" ON "testcase_revisions" ("testset_revision_id", "group_index", "order_index");
-- Create index "testcaserevision_testset_revision_id_kind_group_index_order_ind" to table: "testcase_revisions"
CREATE INDEX "testcaserevision_testset_revision_id_kind_group_index_order_ind" ON "testcase_revisions" ("testset_revision_id", "kind", "group_index", "order_index");
-- Create index "testcaserevision_testset_revision_id_problem_id" to table: "testcase_revisions"
CREATE INDEX "testcaserevision_testset_revision_id_problem_id" ON "testcase_revisions" ("testset_revision_id", "problem_id");
-- Create index "testcaserevision_testset_revision_id_test_case_id" to table: "testcase_revisions"
CREATE UNIQUE INDEX "testcaserevision_testset_revision_id_test_case_id" ON "testcase_revisions" ("testset_revision_id", "test_case_id");
-- Create "testset_revisions" table
CREATE TABLE "testset_revisions" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "revision_number" bigint NOT NULL,
  "checksum" bytea NOT NULL,
  "test_count" bigint NOT NULL,
  "total_uncompressed_bytes" bigint NOT NULL,
  "created_by" uuid NULL,
  "problem_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "testset_revision_checksum_shape" CHECK ((octet_length(checksum) = 32) AND (checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "testset_revision_count_bound" CHECK ((test_count >= 1) AND (test_count <= 256)),
  CONSTRAINT "testset_revision_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (problem_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((created_by IS NULL) OR (created_by <> '00000000-0000-0000-0000-000000000000'::uuid))),
  CONSTRAINT "testset_revision_number_bound" CHECK ((revision_number >= 1) AND (revision_number <= 1000000000)),
  CONSTRAINT "testset_revision_total_size_bound" CHECK ((total_uncompressed_bytes >= 0) AND (total_uncompressed_bytes <= 536870912))
);
-- Create index "testsetrevision_checksum" to table: "testset_revisions"
CREATE INDEX "testsetrevision_checksum" ON "testset_revisions" ("checksum");
-- Create index "testsetrevision_id_problem_id" to table: "testset_revisions"
CREATE UNIQUE INDEX "testsetrevision_id_problem_id" ON "testset_revisions" ("id", "problem_id");
-- Create index "testsetrevision_problem_id_created_at_id" to table: "testset_revisions"
CREATE INDEX "testsetrevision_problem_id_created_at_id" ON "testset_revisions" ("problem_id", "created_at", "id");
-- Create index "testsetrevision_problem_id_revision_number" to table: "testset_revisions"
CREATE UNIQUE INDEX "testsetrevision_problem_id_revision_number" ON "testset_revisions" ("problem_id", "revision_number");
-- Create "trait_effect_revisions" table
CREATE TABLE "trait_effect_revisions" (
  "id" uuid NOT NULL,
  "revision" bigint NOT NULL,
  "schema_version" bigint NOT NULL,
  "effect" jsonb NOT NULL,
  "target_element_ids" jsonb NOT NULL,
  "checksum" bytea NOT NULL,
  "effective_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "trait_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "trait_effect_revision_checksum_shape" CHECK ((octet_length(checksum) = 32) AND (checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "trait_effect_revision_document_shape" CHECK ((schema_version = 1) AND (jsonb_typeof(effect) = 'object'::text) AND (jsonb_typeof(target_element_ids) = 'array'::text) AND (octet_length((effect)::text) <= 8192) AND (octet_length((target_element_ids)::text) <= 8192)),
  CONSTRAINT "trait_effect_revision_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (trait_id <> '00000000-0000-0000-0000-000000000000'::uuid))
);
-- Create index "traiteffectrevision_checksum" to table: "trait_effect_revisions"
CREATE UNIQUE INDEX "traiteffectrevision_checksum" ON "trait_effect_revisions" ("checksum");
-- Create index "traiteffectrevision_trait_id_effective_at_revision_id" to table: "trait_effect_revisions"
CREATE UNIQUE INDEX "traiteffectrevision_trait_id_effective_at_revision_id" ON "trait_effect_revisions" ("trait_id", "effective_at", "revision", "id");
-- Create index "traiteffectrevision_trait_id_revision" to table: "trait_effect_revisions"
CREATE UNIQUE INDEX "traiteffectrevision_trait_id_revision" ON "trait_effect_revisions" ("trait_id", "revision");
-- Create "traits" table
CREATE TABLE "traits" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "code" character varying NOT NULL,
  "type" character varying NOT NULL,
  "name" character varying NOT NULL,
  "description" character varying NULL,
  "display_order" bigint NOT NULL DEFAULT 0,
  "active" boolean NOT NULL DEFAULT true,
  "version" bigint NOT NULL DEFAULT 1,
  "rarity_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "traits_code_key" to table: "traits"
CREATE UNIQUE INDEX "traits_code_key" ON "traits" ("code");
-- Create index "traits_name_key" to table: "traits"
CREATE UNIQUE INDEX "traits_name_key" ON "traits" ("name");
-- Create "user_attribute_values" table
CREATE TABLE "user_attribute_values" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "canonical_value" jsonb NOT NULL,
  "source_value_version" bigint NOT NULL DEFAULT 1,
  "version" bigint NOT NULL DEFAULT 1,
  "value" text NOT NULL,
  "attribute_id" uuid NOT NULL,
  "definition_revision_id" uuid NOT NULL,
  "source_revision_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "userattributevalue_user_id_attribute_id_definition_revision_id" to table: "user_attribute_values"
CREATE UNIQUE INDEX "userattributevalue_user_id_attribute_id_definition_revision_id" ON "user_attribute_values" ("user_id", "attribute_id", "definition_revision_id") WHERE (deleted_at IS NULL);
-- Create "user_difficulty_stats" table
CREATE TABLE "user_difficulty_stats" (
  "id" uuid NOT NULL,
  "solved_count" bigint NOT NULL DEFAULT 0,
  "difficulty_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "userdifficultystats_user_id" to table: "user_difficulty_stats"
CREATE INDEX "userdifficultystats_user_id" ON "user_difficulty_stats" ("user_id");
-- Create index "userdifficultystats_user_id_difficulty_id" to table: "user_difficulty_stats"
CREATE UNIQUE INDEX "userdifficultystats_user_id_difficulty_id" ON "user_difficulty_stats" ("user_id", "difficulty_id");
-- Create "user_element_exps" table
CREATE TABLE "user_element_exps" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "exp" bigint NOT NULL DEFAULT 0,
  "element_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "userelementexp_user_id_element_id" to table: "user_element_exps"
CREATE UNIQUE INDEX "userelementexp_user_id_element_id" ON "user_element_exps" ("user_id", "element_id");
-- Create "user_problem_solve_transitions" table
CREATE TABLE "user_problem_solve_transitions" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "event_id" uuid NOT NULL,
  "solve_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  "problem_id" uuid NOT NULL,
  "from_version" bigint NOT NULL,
  "to_version" bigint NOT NULL,
  "from_status" character varying NOT NULL,
  "to_status" character varying NOT NULL,
  "from_submission_id" uuid NULL,
  "to_submission_id" uuid NULL,
  "from_judgement_id" uuid NULL,
  "to_judgement_id" uuid NULL,
  "from_generation" bigint NULL,
  "to_generation" bigint NULL,
  "from_problem_revision_id" uuid NULL,
  "to_problem_revision_id" uuid NULL,
  "from_reward_facts" jsonb NULL,
  "to_reward_facts" jsonb NULL,
  "from_reward_facts_checksum" bytea NULL,
  "to_reward_facts_checksum" bytea NULL,
  "from_reward_grant_source_id" uuid NULL,
  "to_reward_grant_source_id" uuid NULL,
  "from_reward_provenance" character varying NULL,
  "to_reward_provenance" character varying NULL,
  "from_solved_at" timestamptz NULL,
  "to_solved_at" timestamptz NULL,
  "provenance" character varying NOT NULL,
  "occurred_at" timestamptz NOT NULL,
  "correlation_id" character varying(128) NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "user_problem_solve_transition_changes_projection" CHECK (NOT (((from_status)::text = 'unsolved'::text) AND ((to_status)::text = 'unsolved'::text))),
  CONSTRAINT "user_problem_solve_transition_contiguous" CHECK (to_version = (from_version + 1)),
  CONSTRAINT "user_problem_solve_transition_from_side" CHECK ((((from_status)::text = 'unsolved'::text) AND (from_submission_id IS NULL) AND (from_judgement_id IS NULL) AND (from_generation IS NULL) AND (from_problem_revision_id IS NULL) AND (from_reward_facts IS NULL) AND (from_reward_facts_checksum IS NULL) AND (from_reward_grant_source_id IS NULL) AND (from_reward_provenance IS NULL) AND (from_solved_at IS NULL)) OR (((from_status)::text = 'solved'::text) AND (from_submission_id IS NOT NULL) AND (from_judgement_id IS NOT NULL) AND (from_generation >= 1) AND (from_problem_revision_id IS NOT NULL) AND (from_reward_facts IS NOT NULL) AND (from_reward_facts_checksum IS NOT NULL) AND (from_reward_grant_source_id IS NOT NULL) AND (from_reward_provenance IS NOT NULL) AND (from_solved_at IS NOT NULL))),
  CONSTRAINT "user_problem_solve_transition_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (event_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (solve_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (user_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (problem_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "user_problem_solve_transition_origin" CHECK ((from_version <> 0) OR (((from_status)::text = 'unsolved'::text) AND (from_submission_id IS NULL) AND (from_judgement_id IS NULL))),
  CONSTRAINT "user_problem_solve_transition_reward_bounds" CHECK (((from_reward_facts IS NULL) OR ((jsonb_typeof(from_reward_facts) = 'object'::text) AND (octet_length((from_reward_facts)::text) <= 65536))) AND ((to_reward_facts IS NULL) OR ((jsonb_typeof(to_reward_facts) = 'object'::text) AND (octet_length((to_reward_facts)::text) <= 65536)))),
  CONSTRAINT "user_problem_solve_transition_reward_checksums" CHECK (((from_reward_facts_checksum IS NULL) OR ((octet_length(from_reward_facts_checksum) = 32) AND (from_reward_facts_checksum <> decode(repeat('00'::text, 32), 'hex'::text)))) AND ((to_reward_facts_checksum IS NULL) OR ((octet_length(to_reward_facts_checksum) = 32) AND (to_reward_facts_checksum <> decode(repeat('00'::text, 32), 'hex'::text))))),
  CONSTRAINT "user_problem_solve_transition_reward_grant" CHECK ((((from_status)::text = 'unsolved'::text) AND ((to_status)::text = 'solved'::text) AND (to_reward_grant_source_id = id)) OR (((from_status)::text = 'solved'::text) AND ((to_status)::text = 'solved'::text) AND (to_reward_grant_source_id = from_reward_grant_source_id) AND ((to_reward_provenance)::text = (from_reward_provenance)::text)) OR (((from_status)::text = 'solved'::text) AND ((to_status)::text = 'unsolved'::text))),
  CONSTRAINT "user_problem_solve_transition_to_side" CHECK ((((to_status)::text = 'unsolved'::text) AND (to_submission_id IS NULL) AND (to_judgement_id IS NULL) AND (to_generation IS NULL) AND (to_problem_revision_id IS NULL) AND (to_reward_facts IS NULL) AND (to_reward_facts_checksum IS NULL) AND (to_reward_grant_source_id IS NULL) AND (to_reward_provenance IS NULL) AND (to_solved_at IS NULL)) OR (((to_status)::text = 'solved'::text) AND (to_submission_id IS NOT NULL) AND (to_judgement_id IS NOT NULL) AND (to_generation >= 1) AND (to_problem_revision_id IS NOT NULL) AND (to_reward_facts IS NOT NULL) AND (to_reward_facts_checksum IS NOT NULL) AND (to_reward_grant_source_id IS NOT NULL) AND (to_reward_provenance IS NOT NULL) AND (to_solved_at IS NOT NULL)))
);
-- Create index "userproblemsolvetransition_correlation_id_occurred_at_id" to table: "user_problem_solve_transitions"
CREATE INDEX "userproblemsolvetransition_correlation_id_occurred_at_id" ON "user_problem_solve_transitions" ("correlation_id", "occurred_at", "id");
-- Create index "userproblemsolvetransition_event_id" to table: "user_problem_solve_transitions"
CREATE UNIQUE INDEX "userproblemsolvetransition_event_id" ON "user_problem_solve_transitions" ("event_id");
-- Create index "userproblemsolvetransition_id_event_id_solve_id_user_id_problem" to table: "user_problem_solve_transitions"
CREATE UNIQUE INDEX "userproblemsolvetransition_id_event_id_solve_id_user_id_problem" ON "user_problem_solve_transitions" ("id", "event_id", "solve_id", "user_id", "problem_id", "to_version");
-- Create index "userproblemsolvetransition_id_solve_id" to table: "user_problem_solve_transitions"
CREATE UNIQUE INDEX "userproblemsolvetransition_id_solve_id" ON "user_problem_solve_transitions" ("id", "solve_id");
-- Create index "userproblemsolvetransition_id_solve_id_to_version" to table: "user_problem_solve_transitions"
CREATE UNIQUE INDEX "userproblemsolvetransition_id_solve_id_to_version" ON "user_problem_solve_transitions" ("id", "solve_id", "to_version");
-- Create index "userproblemsolvetransition_solve_id_occurred_at_id" to table: "user_problem_solve_transitions"
CREATE INDEX "userproblemsolvetransition_solve_id_occurred_at_id" ON "user_problem_solve_transitions" ("solve_id", "occurred_at", "id");
-- Create index "userproblemsolvetransition_solve_id_to_version" to table: "user_problem_solve_transitions"
CREATE UNIQUE INDEX "userproblemsolvetransition_solve_id_to_version" ON "user_problem_solve_transitions" ("solve_id", "to_version");
-- Create index "userproblemsolvetransition_user_id_problem_id_to_version" to table: "user_problem_solve_transitions"
CREATE INDEX "userproblemsolvetransition_user_id_problem_id_to_version" ON "user_problem_solve_transitions" ("user_id", "problem_id", "to_version");
-- Add post-creation foreign keys to the "user_problem_solve_transitions" table
ALTER TABLE "user_problem_solve_transitions" ADD CONSTRAINT "solve_transitions_from_reward_grant_fkey" FOREIGN KEY ("from_reward_grant_source_id", "solve_id") REFERENCES "user_problem_solve_transitions" ("id", "solve_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "solve_transitions_to_reward_grant_fkey" FOREIGN KEY ("to_reward_grant_source_id", "solve_id") REFERENCES "user_problem_solve_transitions" ("id", "solve_id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create "user_problem_solves" table
CREATE TABLE "user_problem_solves" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "user_id" uuid NOT NULL,
  "problem_id" uuid NOT NULL,
  "status" character varying NOT NULL DEFAULT 'unsolved',
  "active_submission_id" uuid NULL,
  "active_judgement_id" uuid NULL,
  "active_generation" bigint NULL,
  "active_problem_revision_id" uuid NULL,
  "reward_facts" jsonb NULL,
  "reward_facts_checksum" bytea NULL,
  "active_reward_transition_id" uuid NULL,
  "reward_provenance" character varying NULL,
  "solved_at" timestamptz NULL,
  "version" bigint NOT NULL DEFAULT 0,
  PRIMARY KEY ("id"),
  CONSTRAINT "user_problem_solve_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (user_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (problem_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "user_problem_solve_pointer_invariant" CHECK ((((status)::text = 'unsolved'::text) AND (active_submission_id IS NULL) AND (active_judgement_id IS NULL) AND (active_generation IS NULL) AND (active_problem_revision_id IS NULL) AND (reward_facts IS NULL) AND (reward_facts_checksum IS NULL) AND (active_reward_transition_id IS NULL) AND (reward_provenance IS NULL) AND (solved_at IS NULL)) OR (((status)::text = 'solved'::text) AND (active_submission_id IS NOT NULL) AND (active_judgement_id IS NOT NULL) AND (active_generation >= 1) AND (active_problem_revision_id IS NOT NULL) AND (reward_facts IS NOT NULL) AND (reward_facts_checksum IS NOT NULL) AND (active_reward_transition_id IS NOT NULL) AND (reward_provenance IS NOT NULL) AND (solved_at IS NOT NULL))),
  CONSTRAINT "user_problem_solve_reward_checksum" CHECK ((reward_facts_checksum IS NULL) OR ((octet_length(reward_facts_checksum) = 32) AND (reward_facts_checksum <> decode(repeat('00'::text, 32), 'hex'::text)))),
  CONSTRAINT "user_problem_solve_reward_facts_bound" CHECK ((reward_facts IS NULL) OR ((jsonb_typeof(reward_facts) = 'object'::text) AND (octet_length((reward_facts)::text) <= 65536))),
  CONSTRAINT "user_problem_solve_version_bound" CHECK ((version >= 0) AND (((status)::text = 'unsolved'::text) OR (version >= 1)))
);
-- Create index "userproblemsolve_active_reward_transition_id" to table: "user_problem_solves"
CREATE UNIQUE INDEX "userproblemsolve_active_reward_transition_id" ON "user_problem_solves" ("active_reward_transition_id");
-- Create index "userproblemsolve_active_submission_id_active_judgement_id" to table: "user_problem_solves"
CREATE INDEX "userproblemsolve_active_submission_id_active_judgement_id" ON "user_problem_solves" ("active_submission_id", "active_judgement_id");
-- Create index "userproblemsolve_id_user_id_problem_id" to table: "user_problem_solves"
CREATE UNIQUE INDEX "userproblemsolve_id_user_id_problem_id" ON "user_problem_solves" ("id", "user_id", "problem_id");
-- Create index "userproblemsolve_status_problem_id_solved_at_id" to table: "user_problem_solves"
CREATE INDEX "userproblemsolve_status_problem_id_solved_at_id" ON "user_problem_solves" ("status", "problem_id", "solved_at", "id");
-- Create index "userproblemsolve_user_id_problem_id" to table: "user_problem_solves"
CREATE UNIQUE INDEX "userproblemsolve_user_id_problem_id" ON "user_problem_solves" ("user_id", "problem_id");
-- Create "user_recovery_contacts" table
CREATE TABLE "user_recovery_contacts" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "kind" character varying NOT NULL,
  "purpose" character varying NOT NULL,
  "lookup_hash" bytea NOT NULL,
  "fingerprint_hash" bytea NOT NULL,
  "address_ciphertext" bytea NOT NULL,
  "key_id" character varying NOT NULL,
  "status" character varying NOT NULL DEFAULT 'pending',
  "version" bigint NOT NULL DEFAULT 1,
  "verified_at" timestamptz NULL,
  "revoked_at" timestamptz NULL,
  "replaced_contact_id" uuid NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "recovery_contact_kind_closed" CHECK ((kind)::text = 'email'::text),
  CONSTRAINT "recovery_contact_purpose_closed" CHECK ((purpose)::text = ANY ((ARRAY['registration'::character varying, 'replacement'::character varying, 'invitation'::character varying])::text[])),
  CONSTRAINT "recovery_contact_revoked_at" CHECK ((((status)::text = 'revoked'::text) AND (revoked_at IS NOT NULL)) OR ((status)::text <> 'revoked'::text)),
  CONSTRAINT "recovery_contact_status_closed" CHECK ((status)::text = ANY ((ARRAY['pending'::character varying, 'verified'::character varying, 'revoked'::character varying])::text[])),
  CONSTRAINT "recovery_contact_verified_at" CHECK ((((status)::text = 'verified'::text) AND (verified_at IS NOT NULL)) OR ((status)::text <> 'verified'::text))
);
-- Create index "userrecoverycontact_fingerprint_hash" to table: "user_recovery_contacts"
CREATE INDEX "userrecoverycontact_fingerprint_hash" ON "user_recovery_contacts" ("fingerprint_hash");
-- Create index "userrecoverycontact_lookup_hash" to table: "user_recovery_contacts"
CREATE INDEX "userrecoverycontact_lookup_hash" ON "user_recovery_contacts" ("lookup_hash");
-- Create index "userrecoverycontact_user_id_status" to table: "user_recovery_contacts"
CREATE INDEX "userrecoverycontact_user_id_status" ON "user_recovery_contacts" ("user_id", "status");
-- Create index "userrecoverycontact_user_kind_pending" to table: "user_recovery_contacts"
CREATE UNIQUE INDEX "userrecoverycontact_user_kind_pending" ON "user_recovery_contacts" ("user_id", "kind") WHERE ((status)::text = 'pending'::text);
-- Create index "userrecoverycontact_user_kind_verified" to table: "user_recovery_contacts"
CREATE UNIQUE INDEX "userrecoverycontact_user_kind_verified" ON "user_recovery_contacts" ("user_id", "kind") WHERE ((status)::text = 'verified'::text);
-- Create "user_reward_profile_revisions" table
CREATE TABLE "user_reward_profile_revisions" (
  "id" uuid NOT NULL,
  "revision" bigint NOT NULL,
  "assignment_checksum" bytea NOT NULL,
  "source" character varying NOT NULL,
  "command_id" uuid NULL,
  "request_checksum" bytea NOT NULL,
  "effective_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "source_offer_id" uuid NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "user_reward_profile_checksum_shape" CHECK ((octet_length(assignment_checksum) = 32) AND (assignment_checksum <> decode(repeat('00'::text, 32), 'hex'::text)) AND (octet_length(request_checksum) = 32) AND (request_checksum <> decode(repeat('00'::text, 32), 'hex'::text))),
  CONSTRAINT "user_reward_profile_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (user_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND ((source_offer_id IS NULL) OR (source_offer_id <> '00000000-0000-0000-0000-000000000000'::uuid)) AND ((command_id IS NULL) OR (command_id <> '00000000-0000-0000-0000-000000000000'::uuid))),
  CONSTRAINT "user_reward_profile_source_shape" CHECK ((((source)::text = 'onboarding'::text) AND (source_offer_id IS NOT NULL) AND (command_id IS NULL)) OR (((source)::text = 'administrative'::text) AND (source_offer_id IS NULL) AND (command_id IS NOT NULL)))
);
-- Create index "user_reward_profile_revisions_source_offer_id_key" to table: "user_reward_profile_revisions"
CREATE UNIQUE INDEX "user_reward_profile_revisions_source_offer_id_key" ON "user_reward_profile_revisions" ("source_offer_id");
-- Create index "userrewardprofilerevision_command_id" to table: "user_reward_profile_revisions"
CREATE UNIQUE INDEX "userrewardprofilerevision_command_id" ON "user_reward_profile_revisions" ("command_id") WHERE (command_id IS NOT NULL);
-- Create index "userrewardprofilerevision_source_offer_id" to table: "user_reward_profile_revisions"
CREATE UNIQUE INDEX "userrewardprofilerevision_source_offer_id" ON "user_reward_profile_revisions" ("source_offer_id") WHERE (source_offer_id IS NOT NULL);
-- Create index "userrewardprofilerevision_user_id_effective_at_revision_id" to table: "user_reward_profile_revisions"
CREATE UNIQUE INDEX "userrewardprofilerevision_user_id_effective_at_revision_id" ON "user_reward_profile_revisions" ("user_id", "effective_at", "revision", "id");
-- Create index "userrewardprofilerevision_user_id_revision" to table: "user_reward_profile_revisions"
CREATE UNIQUE INDEX "userrewardprofilerevision_user_id_revision" ON "user_reward_profile_revisions" ("user_id", "revision");
-- Create "user_reward_profile_traits" table
CREATE TABLE "user_reward_profile_traits" (
  "id" uuid NOT NULL,
  "trait_code" character varying NOT NULL,
  "trait_type" character varying NOT NULL,
  "trait_name" character varying NOT NULL,
  "trait_description" character varying NULL,
  "rarity_code" character varying NOT NULL,
  "rarity_name" character varying NOT NULL,
  "slot" character varying NOT NULL,
  "slot_order" bigint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "trait_id" uuid NOT NULL,
  "trait_effect_revision_id" uuid NOT NULL,
  "profile_revision_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "user_reward_profile_trait_nonzero_identity" CHECK ((id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (profile_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (trait_id <> '00000000-0000-0000-0000-000000000000'::uuid) AND (trait_effect_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid)),
  CONSTRAINT "user_reward_profile_trait_slot_shape" CHECK ((((slot)::text = 'root_bone'::text) AND (slot_order = 0)) OR (((slot)::text = 'talent'::text) AND ((slot_order >= 0) AND (slot_order <= 2)))),
  CONSTRAINT "user_reward_profile_trait_type_shape" CHECK ((trait_type)::text = (slot)::text)
);
-- Create index "userrewardprofiletrait_id_profile_revision_id_trait_id_trait_ef" to table: "user_reward_profile_traits"
CREATE UNIQUE INDEX "userrewardprofiletrait_id_profile_revision_id_trait_id_trait_ef" ON "user_reward_profile_traits" ("id", "profile_revision_id", "trait_id", "trait_effect_revision_id");
-- Create index "userrewardprofiletrait_profile_revision_id_slot_slot_order" to table: "user_reward_profile_traits"
CREATE UNIQUE INDEX "userrewardprofiletrait_profile_revision_id_slot_slot_order" ON "user_reward_profile_traits" ("profile_revision_id", "slot", "slot_order");
-- Create index "userrewardprofiletrait_profile_revision_id_trait_effect_revisio" to table: "user_reward_profile_traits"
CREATE UNIQUE INDEX "userrewardprofiletrait_profile_revision_id_trait_effect_revisio" ON "user_reward_profile_traits" ("profile_revision_id", "trait_effect_revision_id");
-- Create index "userrewardprofiletrait_profile_revision_id_trait_id" to table: "user_reward_profile_traits"
CREATE UNIQUE INDEX "userrewardprofiletrait_profile_revision_id_trait_id" ON "user_reward_profile_traits" ("profile_revision_id", "trait_id");
-- Create "user_solved_problems" table
CREATE TABLE "user_solved_problems" (
  "id" uuid NOT NULL,
  "solved_at" timestamptz NOT NULL,
  "problem_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "usersolvedproblem_user_id" to table: "user_solved_problems"
CREATE INDEX "usersolvedproblem_user_id" ON "user_solved_problems" ("user_id");
-- Create index "usersolvedproblem_user_id_problem_id" to table: "user_solved_problems"
CREATE UNIQUE INDEX "usersolvedproblem_user_id_problem_id" ON "user_solved_problems" ("user_id", "problem_id");
-- Create "user_stats" table
CREATE TABLE "user_stats" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "total_exp" bigint NOT NULL DEFAULT 0,
  "rating" bigint NOT NULL DEFAULT 1200,
  "total_submissions" bigint NOT NULL DEFAULT 0,
  "accepted_count" bigint NOT NULL DEFAULT 0,
  "progression_version" bigint NOT NULL DEFAULT 0,
  "rating_version" bigint NOT NULL DEFAULT 0,
  "progression_updated_at" timestamptz NULL,
  "rating_updated_at" timestamptz NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "userstats_user_id" to table: "user_stats"
CREATE UNIQUE INDEX "userstats_user_id" ON "user_stats" ("user_id");
-- Create "user_tag_stats" table
CREATE TABLE "user_tag_stats" (
  "id" uuid NOT NULL,
  "solved_count" bigint NOT NULL DEFAULT 0,
  "tag_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "usertagstats_user_id" to table: "user_tag_stats"
CREATE INDEX "usertagstats_user_id" ON "user_tag_stats" ("user_id");
-- Create index "usertagstats_user_id_tag_id" to table: "user_tag_stats"
CREATE UNIQUE INDEX "usertagstats_user_id_tag_id" ON "user_tag_stats" ("user_id", "tag_id");
-- Create "user_traits" table
CREATE TABLE "user_traits" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "trait_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "usertrait_user_id_trait_id" to table: "user_traits"
CREATE UNIQUE INDEX "usertrait_user_id_trait_id" ON "user_traits" ("user_id", "trait_id");
-- Create "users" table
CREATE TABLE "users" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  "deleted_by" character varying NULL,
  "username" character varying NOT NULL,
  "status" character varying NOT NULL DEFAULT 'active',
  "security_version" bigint NOT NULL DEFAULT 1,
  "profile_visibility" character varying NOT NULL DEFAULT 'private',
  "profile_visibility_version" bigint NOT NULL DEFAULT 1,
  "profile_visibility_updated_at" timestamptz NOT NULL,
  "suspended_at" timestamptz NULL,
  "suspension_reason" character varying NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "user_profile_visibility_closed" CHECK ((profile_visibility)::text = ANY ((ARRAY['private'::character varying, 'public'::character varying])::text[])),
  CONSTRAINT "user_profile_visibility_version_positive" CHECK (profile_visibility_version >= 1),
  CONSTRAINT "user_status_closed" CHECK ((status)::text = ANY ((ARRAY['invited'::character varying, 'active'::character varying, 'suspended'::character varying, 'deleted'::character varying])::text[])),
  CONSTRAINT "user_suspension_state" CHECK ((((status)::text = 'suspended'::text) AND (suspended_at IS NOT NULL) AND (suspension_reason IS NOT NULL)) OR (((status)::text <> 'suspended'::text) AND (suspended_at IS NULL) AND (suspension_reason IS NULL)))
);
-- Create index "users_username_key" to table: "users"
CREATE UNIQUE INDEX "users_username_key" ON "users" ("username");
-- Modify "activated_judge_capabilities" table
ALTER TABLE "activated_judge_capabilities" ADD CONSTRAINT "activated_judge_capabilities_worker_evidence_fkey" FOREIGN KEY ("proof_evidence_id", "proof_worker_instance_id", "proof_boot_id", "judge_spec_protocol", "runtime_key", "runtime_profile_version", "compiler_version", "runtime_bootstrap_version", "compile_command_id", "run_command_id", "image_digest", "manifest_version", "manifest_checksum", "supervisor_protocol", "checker_key", "checker_version", "capability_profile_hash", "checker_implementation_checksum", "smoke_checksum", "golden_checksum") REFERENCES "judge_worker_capabilities" ("id", "worker_instance_id", "boot_id", "judge_spec_protocol", "runtime_key", "runtime_profile_version", "compiler_version", "runtime_bootstrap_version", "compile_command_id", "run_command_id", "image_digest", "manifest_version", "manifest_checksum", "supervisor_protocol", "checker_key", "checker_version", "capability_profile_hash", "checker_implementation_checksum", "smoke_checksum", "golden_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "attribute_definition_revisions" table
ALTER TABLE "attribute_definition_revisions" ADD CONSTRAINT "attribute_definition_revisions_attribute_definitions_revisions" FOREIGN KEY ("definition_id") REFERENCES "attribute_definitions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "attribute_definitions" table
ALTER TABLE "attribute_definitions" ADD CONSTRAINT "attribute_definitions_attribute_definition_revisions_active_rev" FOREIGN KEY ("active_revision_id") REFERENCES "attribute_definition_revisions" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, ADD CONSTRAINT "attribute_definitions_attribute_definition_revisions_draft_revi" FOREIGN KEY ("draft_revision_id") REFERENCES "attribute_definition_revisions" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;
-- Modify "auth_refresh_receipts" table
ALTER TABLE "auth_refresh_receipts" ADD CONSTRAINT "auth_refresh_receipts_auth_refresh_tokens_presented_receipts" FOREIGN KEY ("presented_token_id") REFERENCES "auth_refresh_tokens" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "auth_refresh_receipts_auth_refresh_tokens_replacement_receipts" FOREIGN KEY ("replacement_token_id") REFERENCES "auth_refresh_tokens" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "auth_refresh_receipts_auth_sessions_refresh_receipts" FOREIGN KEY ("session_id") REFERENCES "auth_sessions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "auth_refresh_tokens" table
ALTER TABLE "auth_refresh_tokens" ADD CONSTRAINT "auth_refresh_tokens_auth_sessions_refresh_tokens" FOREIGN KEY ("session_id") REFERENCES "auth_sessions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "auth_session_command_receipts" table
ALTER TABLE "auth_session_command_receipts" ADD CONSTRAINT "auth_session_command_receipts_auth_sessions_actor_commands" FOREIGN KEY ("actor_session_id") REFERENCES "auth_sessions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "auth_session_command_receipts_auth_sessions_targeted_commands" FOREIGN KEY ("target_session_id") REFERENCES "auth_sessions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "auth_session_command_receipts_users_session_commands" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "auth_session_issuance_receipts" table
ALTER TABLE "auth_session_issuance_receipts" ADD CONSTRAINT "auth_session_issuance_receipts_auth_refresh_tokens_issuance_rec" FOREIGN KEY ("refresh_token_id") REFERENCES "auth_refresh_tokens" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "auth_session_issuance_receipts_auth_sessions_issuance_receipts" FOREIGN KEY ("session_id") REFERENCES "auth_sessions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "auth_session_issuance_receipts_onboarding_trait_offers_issuance" FOREIGN KEY ("trait_offer_id") REFERENCES "onboarding_trait_offers" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, ADD CONSTRAINT "auth_session_issuance_receipts_users_session_issuance_receipts" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;
-- Modify "auth_sessions" table
ALTER TABLE "auth_sessions" ADD CONSTRAINT "auth_sessions_users_auth_sessions" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "authorization_actions" table
ALTER TABLE "authorization_actions" ADD CONSTRAINT "authorization_actions_authorization_resources_actions" FOREIGN KEY ("resource_id") REFERENCES "authorization_resources" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "authorization_states" table
ALTER TABLE "authorization_states" ADD CONSTRAINT "authorization_states_users_authorization_updates" FOREIGN KEY ("updated_by") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;
-- Modify "content_render_artifacts" table
ALTER TABLE "content_render_artifacts" ADD CONSTRAINT "content_render_artifacts_content_render_sources_artifacts" FOREIGN KEY ("render_source_id") REFERENCES "content_render_sources" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "content_render_artifacts_render_source_identity_fkey" FOREIGN KEY ("render_source_id", "render_source_kind", "source_checksum") REFERENCES "content_render_sources" ("id", "source_kind", "source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "contest_announcements" table
ALTER TABLE "contest_announcements" ADD CONSTRAINT "contest_announcements_contests_announcements" FOREIGN KEY ("contest_id") REFERENCES "contests" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_announcements_draft_artifact_identity_fkey" FOREIGN KEY ("current_draft_render_artifact_id", "current_draft_render_source_id", "current_draft_render_source_kind", "current_draft_normalized_source_checksum") REFERENCES "content_render_artifacts" ("id", "render_source_id", "render_source_kind", "source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_announcements_draft_revision_identity_fkey" FOREIGN KEY ("current_draft_revision_id", "current_draft_owner_type", "id", "current_draft_content_kind", "current_draft_render_source_id", "current_draft_render_source_kind", "current_draft_normalized_source_checksum") REFERENCES "contest_content_revisions" ("id", "owner_type", "owner_id", "content_kind", "render_source_id", "render_source_kind", "normalized_source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_announcements_published_artifact_identity_fkey" FOREIGN KEY ("published_render_artifact_id", "published_render_source_id", "published_render_source_kind", "published_normalized_source_checksum") REFERENCES "content_render_artifacts" ("id", "render_source_id", "render_source_kind", "source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_announcements_published_revision_identity_fkey" FOREIGN KEY ("published_revision_id", "published_owner_type", "id", "published_content_kind", "published_render_source_id", "published_render_source_kind", "published_normalized_source_checksum") REFERENCES "contest_content_revisions" ("id", "owner_type", "owner_id", "content_kind", "render_source_id", "render_source_kind", "normalized_source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "contest_clarifications" table
ALTER TABLE "contest_clarifications" ADD CONSTRAINT "contest_clarifications_answer_artifact_identity_fkey" FOREIGN KEY ("current_answer_render_artifact_id", "current_answer_render_source_id", "current_answer_render_source_kind", "current_answer_normalized_source_checksum") REFERENCES "content_render_artifacts" ("id", "render_source_id", "render_source_kind", "source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_clarifications_answer_revision_identity_fkey" FOREIGN KEY ("current_answer_revision_id", "current_answer_owner_type", "id", "current_answer_content_kind", "current_answer_render_source_id", "current_answer_render_source_kind", "current_answer_normalized_source_checksum") REFERENCES "contest_content_revisions" ("id", "owner_type", "owner_id", "content_kind", "render_source_id", "render_source_kind", "normalized_source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_clarifications_contest_problem_identity_fkey" FOREIGN KEY ("contest_problem_id", "contest_id") REFERENCES "contest_problems" ("id", "contest_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_clarifications_contest_problems_clarifications" FOREIGN KEY ("contest_problem_id") REFERENCES "contest_problems" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, ADD CONSTRAINT "contest_clarifications_contests_clarifications" FOREIGN KEY ("contest_id") REFERENCES "contests" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_clarifications_public_artifact_identity_fkey" FOREIGN KEY ("current_public_question_render_artifact_id", "current_public_question_render_source_id", "current_public_question_render_source_kind", "current_public_question_normalized_source_checksum") REFERENCES "content_render_artifacts" ("id", "render_source_id", "render_source_kind", "source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_clarifications_public_revision_identity_fkey" FOREIGN KEY ("current_public_question_revision_id", "current_public_question_owner_type", "id", "current_public_question_content_kind", "current_public_question_render_source_id", "current_public_question_render_source_kind", "current_public_question_normalized_source_checksum") REFERENCES "contest_content_revisions" ("id", "owner_type", "owner_id", "content_kind", "render_source_id", "render_source_kind", "normalized_source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_clarifications_question_artifact_identity_fkey" FOREIGN KEY ("question_render_artifact_id", "question_render_source_id", "question_render_source_kind", "question_normalized_source_checksum") REFERENCES "content_render_artifacts" ("id", "render_source_id", "render_source_kind", "source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_clarifications_question_revision_identity_fkey" FOREIGN KEY ("question_revision_id", "question_owner_type", "id", "question_content_kind", "question_render_source_id", "question_render_source_kind", "question_normalized_source_checksum") REFERENCES "contest_content_revisions" ("id", "owner_type", "owner_id", "content_kind", "render_source_id", "render_source_kind", "normalized_source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_clarifications_users_contest_clarifications" FOREIGN KEY ("requester_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "contest_content_commands" table
ALTER TABLE "contest_content_commands" ADD CONSTRAINT "contest_content_commands_contests_content_commands" FOREIGN KEY ("contest_id") REFERENCES "contests" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_content_commands_users_contest_content_commands" FOREIGN KEY ("actor_user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "contest_content_revisions" table
ALTER TABLE "contest_content_revisions" ADD CONSTRAINT "contest_content_revisions_render_artifact_identity_fkey" FOREIGN KEY ("render_artifact_id", "render_source_id", "render_source_kind", "normalized_source_checksum") REFERENCES "content_render_artifacts" ("id", "render_source_id", "render_source_kind", "source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_content_revisions_render_source_identity_fkey" FOREIGN KEY ("render_source_id", "id", "render_source_kind", "normalized_source_checksum") REFERENCES "content_render_sources" ("id", "owner_revision_id", "source_kind", "source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_content_revisions_users_contest_content_revisions" FOREIGN KEY ("author_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "contest_draft_problems" table
ALTER TABLE "contest_draft_problems" ADD CONSTRAINT "contest_draft_problems_contests_draft_problems" FOREIGN KEY ("contest_id") REFERENCES "contests" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_draft_problems_problems_contest_draft_memberships" FOREIGN KEY ("problem_id") REFERENCES "problems" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "contest_problem_results" table
ALTER TABLE "contest_problem_results" ADD CONSTRAINT "contest_problem_results_contest_problems_scoring_results" FOREIGN KEY ("contest_problem_id") REFERENCES "contest_problems" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_problem_results_contests_problem_results" FOREIGN KEY ("contest_id") REFERENCES "contests" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_problem_results_problems_contest_problem_results" FOREIGN KEY ("problem_id") REFERENCES "problems" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_problem_results_submissions_last_contest_problem_result" FOREIGN KEY ("last_scoring_submission_id") REFERENCES "submissions" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, ADD CONSTRAINT "contest_problem_results_users_contest_problem_results" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "contest_problems" table
ALTER TABLE "contest_problems" ADD CONSTRAINT "contest_problems_contests_contest_problems" FOREIGN KEY ("contest_id") REFERENCES "contests" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_problems_problem_revision_identity_fkey" FOREIGN KEY ("problem_revision_id", "problem_id", "testset_revision_id") REFERENCES "problem_revisions" ("id", "problem_id", "testset_revision_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_problems_problem_revisions_contest_problem_snapshots" FOREIGN KEY ("problem_revision_id") REFERENCES "problem_revisions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_problems_problems_contest_problem_snapshots" FOREIGN KEY ("problem_id") REFERENCES "problems" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_problems_testset_owner_fkey" FOREIGN KEY ("testset_revision_id", "problem_id") REFERENCES "testset_revisions" ("id", "problem_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_problems_testset_revisions_contest_problem_snapshots" FOREIGN KEY ("testset_revision_id") REFERENCES "testset_revisions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "contest_rating_results" table
ALTER TABLE "contest_rating_results" ADD CONSTRAINT "contest_rating_results_contest_rating_runs_results" FOREIGN KEY ("rating_run_id") REFERENCES "contest_rating_runs" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_rating_results_users_contest_rating_results" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "contest_rating_runs" table
ALTER TABLE "contest_rating_runs" ADD CONSTRAINT "contest_rating_runs_contests_rating_runs" FOREIGN KEY ("contest_id") REFERENCES "contests" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_rating_runs_rating_sequence_slots_runs" FOREIGN KEY ("rating_sequence_slot_id") REFERENCES "rating_sequence_slots" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "contest_registration_transitions" table
ALTER TABLE "contest_registration_transitions" ADD CONSTRAINT "contest_registration_transitions_contest_registrations_transiti" FOREIGN KEY ("registration_id") REFERENCES "contest_registrations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_registration_transitions_registration_identity_fkey" FOREIGN KEY ("registration_id", "contest_id", "user_id") REFERENCES "contest_registrations" ("id", "contest_id", "user_id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "contest_registrations" table
ALTER TABLE "contest_registrations" ADD CONSTRAINT "contest_registrations_contests_registrations" FOREIGN KEY ("contest_id") REFERENCES "contests" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_registrations_users_contest_registrations" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "contest_scoring_attempts" table
ALTER TABLE "contest_scoring_attempts" ADD CONSTRAINT "contest_scoring_attempts_contest_problems_scoring_attempts" FOREIGN KEY ("contest_problem_id") REFERENCES "contest_problems" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_scoring_attempts_contests_scoring_attempts" FOREIGN KEY ("contest_id") REFERENCES "contests" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_scoring_attempts_problems_contest_scoring_attempts" FOREIGN KEY ("problem_id") REFERENCES "problems" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_scoring_attempts_submissions_contest_scoring_attempt" FOREIGN KEY ("submission_id") REFERENCES "submissions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_scoring_attempts_users_contest_scoring_attempts" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "contest_standings" table
ALTER TABLE "contest_standings" ADD CONSTRAINT "contest_standings_contests_standings" FOREIGN KEY ("contest_id") REFERENCES "contests" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_standings_users_contest_standings" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "contest_transitions" table
ALTER TABLE "contest_transitions" ADD CONSTRAINT "contest_transitions_contests_transitions" FOREIGN KEY ("contest_id") REFERENCES "contests" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "contest_user_ratings" table
ALTER TABLE "contest_user_ratings" ADD CONSTRAINT "contest_user_ratings_rating_projection_generations_ratings" FOREIGN KEY ("projection_generation_id") REFERENCES "rating_projection_generations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "contest_user_ratings_users_contest_user_ratings" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "contests" table
ALTER TABLE "contests" ADD CONSTRAINT "contests_users_contests" FOREIGN KEY ("author_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "credentials" table
ALTER TABLE "credentials" ADD CONSTRAINT "credentials_users_credentials" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "cultivation_ranking_projections" table
ALTER TABLE "cultivation_ranking_projections" ADD CONSTRAINT "cultivation_ranking_projections_users_cultivation_ranking_proje" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "difficulty_revisions" table
ALTER TABLE "difficulty_revisions" ADD CONSTRAINT "difficulty_revisions_difficulties_revisions" FOREIGN KEY ("difficulty_id") REFERENCES "difficulties" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "element_revisions" table
ALTER TABLE "element_revisions" ADD CONSTRAINT "element_revisions_elements_revisions" FOREIGN KEY ("element_id") REFERENCES "elements" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "federated_identities" table
ALTER TABLE "federated_identities" ADD CONSTRAINT "federated_identities_users_federated_identities" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "judge_attempts" table
ALTER TABLE "judge_attempts" ADD CONSTRAINT "judge_attempts_job_generation_spec_fkey" FOREIGN KEY ("job_id", "submission_id", "generation", "judge_spec_id") REFERENCES "judge_jobs" ("id", "submission_id", "generation", "judge_spec_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "judge_attempts_spec_provenance_fkey" FOREIGN KEY ("judge_spec_id", "runtime_profile_version", "compiler_version", "runtime_bootstrap_version", "supervisor_protocol_version", "sandbox_image_digest", "artifact_manifest_checksum") REFERENCES "judge_specs" ("id", "runtime_profile_version", "compiler_version", "runtime_bootstrap_version", "supervisor_protocol_version", "sandbox_image_digest", "artifact_manifest_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "judge_attempts_worker_boot_fkey" FOREIGN KEY ("worker_id", "worker_boot_id") REFERENCES "judge_worker_instances" ("id", "boot_id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "judge_jobs" table
ALTER TABLE "judge_jobs" ADD CONSTRAINT "judge_jobs_spec_generation_fkey" FOREIGN KEY ("judge_spec_id", "submission_id", "generation") REFERENCES "judge_specs" ("id", "submission_id", "generation") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "judge_specs" table
ALTER TABLE "judge_specs" ADD CONSTRAINT "judge_specs_problem_revision_identity_fkey" FOREIGN KEY ("problem_revision_id", "problem_id", "testset_revision_id") REFERENCES "problem_revisions" ("id", "problem_id", "testset_revision_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "judge_specs_submission_owner_fkey" FOREIGN KEY ("submission_id", "user_id", "problem_id") REFERENCES "submissions" ("id", "user_id", "problem_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "judge_specs_testset_owner_fkey" FOREIGN KEY ("testset_revision_id", "problem_id") REFERENCES "testset_revisions" ("id", "problem_id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "judge_test_results" table
ALTER TABLE "judge_test_results" ADD CONSTRAINT "judge_test_results_attempt_spec_fkey" FOREIGN KEY ("attempt_id", "judge_spec_id") REFERENCES "judge_attempts" ("id", "judge_spec_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "judge_test_results_group_identity_fkey" FOREIGN KEY ("group_id", "testset_revision_id", "group_index", "group_stop_on_failure") REFERENCES "testcase_groups" ("id", "testset_revision_id", "group_index", "stop_on_failure") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "judge_test_results_spec_testset_fkey" FOREIGN KEY ("judge_spec_id", "testset_revision_id") REFERENCES "judge_specs" ("id", "testset_revision_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "judge_test_results_testcase_identity_fkey" FOREIGN KEY ("testcase_revision_id", "testset_revision_id", "group_id", "group_index", "case_order", "test_kind", "group_stop_on_failure") REFERENCES "testcase_revisions" ("id", "testset_revision_id", "group_id", "group_index", "order_index", "kind", "stop_on_failure") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "judge_worker_capabilities" table
ALTER TABLE "judge_worker_capabilities" ADD CONSTRAINT "judge_worker_capabilities_worker_boot_fkey" FOREIGN KEY ("worker_instance_id", "boot_id") REFERENCES "judge_worker_instances" ("id", "boot_id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "material_revision_tags" table
ALTER TABLE "material_revision_tags" ADD CONSTRAINT "material_revision_tags_material_revisions_tags" FOREIGN KEY ("revision_id") REFERENCES "material_revisions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "material_revision_tags_tags_material_revision_tags" FOREIGN KEY ("tag_id") REFERENCES "tags" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "material_revisions" table
ALTER TABLE "material_revisions" ADD CONSTRAINT "material_revisions_difficulties_material_revisions" FOREIGN KEY ("difficulty_id") REFERENCES "difficulties" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "material_revisions_material_categories_revisions" FOREIGN KEY ("category_id") REFERENCES "material_categories" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "material_revisions_materials_revisions" FOREIGN KEY ("material_id") REFERENCES "materials" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "material_revisions_render_artifact_identity_fkey" FOREIGN KEY ("render_artifact_id", "render_source_id", "render_source_kind", "normalized_source_checksum") REFERENCES "content_render_artifacts" ("id", "render_source_id", "render_source_kind", "source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "material_revisions_render_source_identity_fkey" FOREIGN KEY ("render_source_id", "id", "render_source_kind", "normalized_source_checksum") REFERENCES "content_render_sources" ("id", "owner_revision_id", "source_kind", "source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "material_revisions_users_material_revisions" FOREIGN KEY ("author_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "material_slugs" table
ALTER TABLE "material_slugs" ADD CONSTRAINT "material_slugs_materials_slugs" FOREIGN KEY ("material_id") REFERENCES "materials" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "material_tags" table
ALTER TABLE "material_tags" ADD CONSTRAINT "material_tags_material_id" FOREIGN KEY ("material_id") REFERENCES "materials" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, ADD CONSTRAINT "material_tags_tag_id" FOREIGN KEY ("tag_id") REFERENCES "tags" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "material_view_dailies" table
ALTER TABLE "material_view_dailies" ADD CONSTRAINT "material_view_dailies_materials_daily_views" FOREIGN KEY ("material_id") REFERENCES "materials" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "materials" table
ALTER TABLE "materials" ADD CONSTRAINT "materials_current_draft_owner_fkey" FOREIGN KEY ("current_draft_revision_id", "id") REFERENCES "material_revisions" ("id", "material_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "materials_current_slug_owner_fkey" FOREIGN KEY ("current_slug_id", "id", "current_slug_state") REFERENCES "material_slugs" ("id", "material_id", "state") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "materials_difficulties_materials" FOREIGN KEY ("difficulty_id") REFERENCES "difficulties" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, ADD CONSTRAINT "materials_material_categories_legacy_materials" FOREIGN KEY ("category_id") REFERENCES "material_categories" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, ADD CONSTRAINT "materials_published_artifact_source_fkey" FOREIGN KEY ("published_render_artifact_id", "published_render_source_id", "published_render_source_kind", "published_source_checksum") REFERENCES "content_render_artifacts" ("id", "render_source_id", "render_source_kind", "source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "materials_published_revision_owner_fkey" FOREIGN KEY ("published_revision_id", "id") REFERENCES "material_revisions" ("id", "material_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "materials_published_revision_source_fkey" FOREIGN KEY ("published_revision_id", "published_render_source_id", "published_render_source_kind", "published_source_checksum") REFERENCES "material_revisions" ("id", "render_source_id", "render_source_kind", "normalized_source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "materials_users_materials" FOREIGN KEY ("author_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "notification_campaign_confirmations" table
ALTER TABLE "notification_campaign_confirmations" ADD CONSTRAINT "notification_campaign_confirmations_notification_campaigns_conf" FOREIGN KEY ("campaign_id") REFERENCES "notification_campaigns" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "notification_campaign_explicit_users" table
ALTER TABLE "notification_campaign_explicit_users" ADD CONSTRAINT "notification_campaign_explicit_users_notification_campaign_revi" FOREIGN KEY ("campaign_revision_id") REFERENCES "notification_campaign_revisions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "notification_campaign_explicit_users_users_explicit_notificatio" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "notification_campaign_recipients" table
ALTER TABLE "notification_campaign_recipients" ADD CONSTRAINT "notification_campaign_recipients_notification_campaign_revision" FOREIGN KEY ("campaign_revision_id") REFERENCES "notification_campaign_revisions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "notification_campaign_recipients_notification_campaigns_recipie" FOREIGN KEY ("campaign_id") REFERENCES "notification_campaigns" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "notification_campaign_recipients_notifications_campaign_deliver" FOREIGN KEY ("notification_id") REFERENCES "notifications" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, ADD CONSTRAINT "notification_campaign_recipients_users_notification_campaign_de" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "notification_campaign_revisions" table
ALTER TABLE "notification_campaign_revisions" ADD CONSTRAINT "notification_campaign_revisions_notification_campaigns_revision" FOREIGN KEY ("campaign_id") REFERENCES "notification_campaigns" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "notification_campaign_revisions_users_notification_campaign_rev" FOREIGN KEY ("author_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "notification_campaigns" table
ALTER TABLE "notification_campaigns" ADD CONSTRAINT "notification_campaigns_users_created_notification_campaigns" FOREIGN KEY ("created_by") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "notification_preferences" table
ALTER TABLE "notification_preferences" ADD CONSTRAINT "notification_preferences_users_notification_preferences" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "notifications" table
ALTER TABLE "notifications" ADD CONSTRAINT "notifications_notification_campaign_revisions_notifications" FOREIGN KEY ("campaign_revision_id") REFERENCES "notification_campaign_revisions" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, ADD CONSTRAINT "notifications_users_notifications" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "oauth_completion_tickets" table
ALTER TABLE "oauth_completion_tickets" ADD CONSTRAINT "oauth_completion_tickets_auth_sessions_oauth_completion_tickets" FOREIGN KEY ("bound_session_id") REFERENCES "auth_sessions" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, ADD CONSTRAINT "oauth_completion_tickets_oauth_flows_completion_tickets" FOREIGN KEY ("flow_id") REFERENCES "oauth_flows" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "oauth_completion_tickets_users_oauth_completion_tickets" FOREIGN KEY ("bound_user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;
-- Modify "oauth_flows" table
ALTER TABLE "oauth_flows" ADD CONSTRAINT "oauth_flows_auth_sessions_oauth_flows" FOREIGN KEY ("session_id") REFERENCES "auth_sessions" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;
-- Modify "onboarding_trait_offer_talents" table
ALTER TABLE "onboarding_trait_offer_talents" ADD CONSTRAINT "onboarding_trait_offer_talents_onboarding_trait_offers_talents" FOREIGN KEY ("offer_id") REFERENCES "onboarding_trait_offers" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "onboarding_trait_offer_talents_traits_onboarding_offer_talents" FOREIGN KEY ("trait_id") REFERENCES "traits" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "onboarding_trait_offers" table
ALTER TABLE "onboarding_trait_offers" ADD CONSTRAINT "onboarding_trait_offers_cultivation_catalog_revisions_offers" FOREIGN KEY ("catalog_revision_id") REFERENCES "cultivation_catalog_revisions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "onboarding_trait_offers_users_consumed_trait_offers" FOREIGN KEY ("consumed_by_user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;
-- Modify "operation_batches" table
ALTER TABLE "operation_batches" ADD CONSTRAINT "operation_batches_operations_batches" FOREIGN KEY ("operation_id") REFERENCES "operations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "operation_control_receipts" table
ALTER TABLE "operation_control_receipts" ADD CONSTRAINT "operation_control_receipts_operations_control_receipts" FOREIGN KEY ("operation_id") REFERENCES "operations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "password_reset_tokens" table
ALTER TABLE "password_reset_tokens" ADD CONSTRAINT "password_reset_tokens_user_recovery_contacts_password_reset_tok" FOREIGN KEY ("contact_id") REFERENCES "user_recovery_contacts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "password_reset_tokens_users_password_reset_tokens" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "problem_revisions" table
ALTER TABLE "problem_revisions" ADD CONSTRAINT "problem_revisions_difficulty_revision_fkey" FOREIGN KEY ("difficulty_id", "difficulty_revision") REFERENCES "difficulty_revisions" ("difficulty_id", "revision") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "problem_revisions_element_mapping_owner_fkey" FOREIGN KEY ("problem_id", "element_mapping_id") REFERENCES "problems" ("id", "element_mapping_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "problem_revisions_problems_revisions" FOREIGN KEY ("problem_id") REFERENCES "problems" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "problem_revisions_render_source_identity_fkey" FOREIGN KEY ("render_source_id", "id", "render_source_kind", "render_source_checksum") REFERENCES "content_render_sources" ("id", "owner_revision_id", "source_kind", "source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "problem_revisions_tag_mapping_owner_fkey" FOREIGN KEY ("problem_id", "tag_mapping_id") REFERENCES "problems" ("id", "tag_mapping_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "problem_revisions_testset_owner_fkey" FOREIGN KEY ("testset_revision_id", "problem_id") REFERENCES "testset_revisions" ("id", "problem_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "problem_revisions_testset_revisions_problem_revisions" FOREIGN KEY ("testset_revision_id") REFERENCES "testset_revisions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "problem_tags" table
ALTER TABLE "problem_tags" ADD CONSTRAINT "problem_tags_problem_id" FOREIGN KEY ("problem_id") REFERENCES "problems" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, ADD CONSTRAINT "problem_tags_tag_id" FOREIGN KEY ("tag_id") REFERENCES "tags" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "problems" table
ALTER TABLE "problems" ADD CONSTRAINT "problems_difficulties_problems" FOREIGN KEY ("difficulty_id") REFERENCES "difficulties" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "problems_draft_artifact_source_fkey" FOREIGN KEY ("draft_render_artifact_id", "draft_render_source_id", "draft_render_source_kind", "draft_render_source_checksum") REFERENCES "content_render_artifacts" ("id", "render_source_id", "render_source_kind", "source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "problems_draft_revision_owner_fkey" FOREIGN KEY ("draft_revision_id", "id") REFERENCES "problem_revisions" ("id", "problem_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "problems_draft_revision_source_fkey" FOREIGN KEY ("draft_revision_id", "draft_render_source_id", "draft_render_source_kind", "draft_render_source_checksum") REFERENCES "problem_revisions" ("id", "render_source_id", "render_source_kind", "render_source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "problems_published_artifact_source_fkey" FOREIGN KEY ("published_render_artifact_id", "published_render_source_id", "published_render_source_kind", "published_render_source_checksum") REFERENCES "content_render_artifacts" ("id", "render_source_id", "render_source_kind", "source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "problems_published_revision_owner_fkey" FOREIGN KEY ("published_revision_id", "id") REFERENCES "problem_revisions" ("id", "problem_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "problems_published_revision_source_fkey" FOREIGN KEY ("published_revision_id", "published_render_source_id", "published_render_source_kind", "published_render_source_checksum") REFERENCES "problem_revisions" ("id", "render_source_id", "render_source_kind", "render_source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "problems_users_problems" FOREIGN KEY ("author_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "progression_ledger_entries" table
ALTER TABLE "progression_ledger_entries" ADD CONSTRAINT "progression_ledger_entries_reward_rule_revisions_ledger_entries" FOREIGN KEY ("reward_rule_revision_id") REFERENCES "reward_rule_revisions" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, ADD CONSTRAINT "progression_ledger_entries_user_reward_profile_revisions_ledger" FOREIGN KEY ("reward_profile_revision_id") REFERENCES "user_reward_profile_revisions" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;
-- Modify "rating_histories" table
ALTER TABLE "rating_histories" ADD CONSTRAINT "rating_histories_contest_rating_runs_compatibility_histories" FOREIGN KEY ("rating_run_id") REFERENCES "contest_rating_runs" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, ADD CONSTRAINT "rating_histories_contests_rating_histories" FOREIGN KEY ("contest_id") REFERENCES "contests" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "rating_histories_users_rating_histories" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "rating_run_chain_entries" table
ALTER TABLE "rating_run_chain_entries" ADD CONSTRAINT "rating_run_chain_entries_contest_rating_runs_chain_entries" FOREIGN KEY ("rating_run_id") REFERENCES "contest_rating_runs" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "rating_run_chain_entries_rating_run_chains_entries" FOREIGN KEY ("chain_id") REFERENCES "rating_run_chains" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "rating_sequence_slots" table
ALTER TABLE "rating_sequence_slots" ADD CONSTRAINT "rating_sequence_slots_contests_rating_sequence_slots" FOREIGN KEY ("contest_id") REFERENCES "contests" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "rating_states" table
ALTER TABLE "rating_states" ADD CONSTRAINT "rating_states_rating_projection_generations_active_states" FOREIGN KEY ("active_projection_generation_id") REFERENCES "rating_projection_generations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "rating_states_rating_run_chains_active_states" FOREIGN KEY ("active_run_chain_id") REFERENCES "rating_run_chains" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "recovery_contact_command_receipts" table
ALTER TABLE "recovery_contact_command_receipts" ADD CONSTRAINT "recovery_contact_command_receipts_auth_sessions_recovery_contac" FOREIGN KEY ("session_id") REFERENCES "auth_sessions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "recovery_contact_command_receipts_users_recovery_contact_comman" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "recovery_contact_verification_tokens" table
ALTER TABLE "recovery_contact_verification_tokens" ADD CONSTRAINT "recovery_contact_verification_tokens_user_recovery_contacts_ver" FOREIGN KEY ("contact_id") REFERENCES "user_recovery_contacts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "security_email_deliveries" table
ALTER TABLE "security_email_deliveries" ADD CONSTRAINT "security_email_deliveries_password_reset_tokens_email_deliverie" FOREIGN KEY ("password_reset_token_id") REFERENCES "password_reset_tokens" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, ADD CONSTRAINT "security_email_deliveries_recovery_contact_verification_tokens_" FOREIGN KEY ("verification_token_id") REFERENCES "recovery_contact_verification_tokens" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;
-- Modify "submission_judgements" table
ALTER TABLE "submission_judgements" ADD CONSTRAINT "submission_judgements_spec_generation_fkey" FOREIGN KEY ("judge_spec_id", "submission_id", "generation", "testset_revision_id", "problem_revision_id", "judge_spec_checksum", "reward_snapshot_checksum", "runtime_profile_version", "compiler_version", "runtime_bootstrap_version", "supervisor_protocol_version", "sandbox_image_digest", "artifact_manifest_checksum", "release_id", "approved_release_manifest_checksum", "checker_implementation_checksum") REFERENCES "judge_specs" ("id", "submission_id", "generation", "testset_revision_id", "problem_revision_id", "spec_checksum", "reward_snapshot_checksum", "runtime_profile_version", "compiler_version", "runtime_bootstrap_version", "supervisor_protocol_version", "sandbox_image_digest", "artifact_manifest_checksum", "release_id", "approved_release_manifest_checksum", "checker_implementation_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "submission_judgements_winning_attempt_fkey" FOREIGN KEY ("winning_attempt_id", "submission_id", "generation", "judge_spec_id") REFERENCES "judge_attempts" ("id", "submission_id", "generation", "judge_spec_id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "submissions" table
ALTER TABLE "submissions" ADD CONSTRAINT "submissions_active_judgement_fkey" FOREIGN KEY ("active_judgement_id", "id", "active_judgement_generation", "active_verdict") REFERENCES "submission_judgements" ("id", "submission_id", "generation", "verdict") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "submissions_contest_problem_snapshot_fkey" FOREIGN KEY ("contest_problem_id", "contest_id", "problem_id") REFERENCES "contest_problems" ("id", "contest_id", "problem_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "submissions_contests_submissions" FOREIGN KEY ("contest_id") REFERENCES "contests" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, ADD CONSTRAINT "submissions_problems_submissions" FOREIGN KEY ("problem_id") REFERENCES "problems" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "submissions_users_submissions" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "tag_elements" table
ALTER TABLE "tag_elements" ADD CONSTRAINT "tag_elements_element_id" FOREIGN KEY ("element_id") REFERENCES "elements" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, ADD CONSTRAINT "tag_elements_tag_id" FOREIGN KEY ("tag_id") REFERENCES "tags" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "tag_revision_elements" table
ALTER TABLE "tag_revision_elements" ADD CONSTRAINT "tag_revision_elements_element_revision_fkey" FOREIGN KEY ("element_id", "element_revision") REFERENCES "element_revisions" ("element_id", "revision") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "tag_revision_elements_elements_tag_revision_elements" FOREIGN KEY ("element_id") REFERENCES "elements" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "tag_revision_elements_tag_revisions_elements" FOREIGN KEY ("tag_revision_id") REFERENCES "tag_revisions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "tag_revisions" table
ALTER TABLE "tag_revisions" ADD CONSTRAINT "tag_revisions_tags_revisions" FOREIGN KEY ("tag_id") REFERENCES "tags" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "test_cases" table
ALTER TABLE "test_cases" ADD CONSTRAINT "test_cases_problems_test_cases" FOREIGN KEY ("problem_id") REFERENCES "problems" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "testcase_groups" table
ALTER TABLE "testcase_groups" ADD CONSTRAINT "testcase_groups_testset_revisions_groups" FOREIGN KEY ("testset_revision_id") REFERENCES "testset_revisions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "testcase_revisions" table
ALTER TABLE "testcase_revisions" ADD CONSTRAINT "testcase_revisions_group_identity_fkey" FOREIGN KEY ("group_id", "testset_revision_id", "group_index", "stop_on_failure") REFERENCES "testcase_groups" ("id", "testset_revision_id", "group_index", "stop_on_failure") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "testcase_revisions_test_assets_expected_testcase_revisions" FOREIGN KEY ("expected_asset_id") REFERENCES "test_assets" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "testcase_revisions_test_assets_input_testcase_revisions" FOREIGN KEY ("input_asset_id") REFERENCES "test_assets" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "testcase_revisions_test_case_owner_fkey" FOREIGN KEY ("test_case_id", "problem_id") REFERENCES "test_cases" ("id", "problem_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "testcase_revisions_test_cases_revisions" FOREIGN KEY ("test_case_id") REFERENCES "test_cases" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "testcase_revisions_testcase_groups_testcase_revisions" FOREIGN KEY ("group_id") REFERENCES "testcase_groups" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "testcase_revisions_testset_owner_fkey" FOREIGN KEY ("testset_revision_id", "problem_id") REFERENCES "testset_revisions" ("id", "problem_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "testcase_revisions_testset_revisions_testcase_revisions" FOREIGN KEY ("testset_revision_id") REFERENCES "testset_revisions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "testset_revisions" table
ALTER TABLE "testset_revisions" ADD CONSTRAINT "testset_revisions_problems_testset_revisions" FOREIGN KEY ("problem_id") REFERENCES "problems" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "trait_effect_revisions" table
ALTER TABLE "trait_effect_revisions" ADD CONSTRAINT "trait_effect_revisions_traits_effect_revisions" FOREIGN KEY ("trait_id") REFERENCES "traits" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "traits" table
ALTER TABLE "traits" ADD CONSTRAINT "traits_rarities_traits" FOREIGN KEY ("rarity_id") REFERENCES "rarities" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "user_attribute_values" table
ALTER TABLE "user_attribute_values" ADD CONSTRAINT "user_attribute_values_attribute_definition_revisions_derived_va" FOREIGN KEY ("source_revision_id") REFERENCES "attribute_definition_revisions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "user_attribute_values_attribute_definition_revisions_values" FOREIGN KEY ("definition_revision_id") REFERENCES "attribute_definition_revisions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "user_attribute_values_attribute_definitions_values" FOREIGN KEY ("attribute_id") REFERENCES "attribute_definitions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "user_attribute_values_definition_revision_owner_fkey" FOREIGN KEY ("definition_revision_id", "attribute_id") REFERENCES "attribute_definition_revisions" ("id", "definition_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "user_attribute_values_source_revision_owner_fkey" FOREIGN KEY ("source_revision_id", "attribute_id") REFERENCES "attribute_definition_revisions" ("id", "definition_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "user_attribute_values_users_attributes" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "user_difficulty_stats" table
ALTER TABLE "user_difficulty_stats" ADD CONSTRAINT "user_difficulty_stats_difficulties_user_difficulty_stats" FOREIGN KEY ("difficulty_id") REFERENCES "difficulties" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "user_difficulty_stats_users_difficulty_stats" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "user_element_exps" table
ALTER TABLE "user_element_exps" ADD CONSTRAINT "user_element_exps_elements_user_element_exps" FOREIGN KEY ("element_id") REFERENCES "elements" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "user_element_exps_users_user_element_exps" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "user_problem_solve_transitions" table
ALTER TABLE "user_problem_solve_transitions" ADD CONSTRAINT "solve_transitions_from_judgement_fkey" FOREIGN KEY ("from_judgement_id", "from_submission_id", "from_generation", "from_problem_revision_id", "from_reward_facts_checksum") REFERENCES "submission_judgements" ("id", "submission_id", "generation", "problem_revision_id", "reward_snapshot_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "solve_transitions_solve_owner_fkey" FOREIGN KEY ("solve_id", "user_id", "problem_id") REFERENCES "user_problem_solves" ("id", "user_id", "problem_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "solve_transitions_to_judgement_fkey" FOREIGN KEY ("to_judgement_id", "to_submission_id", "to_generation", "to_problem_revision_id", "to_reward_facts_checksum") REFERENCES "submission_judgements" ("id", "submission_id", "generation", "problem_revision_id", "reward_snapshot_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "user_problem_solves" table
ALTER TABLE "user_problem_solves" ADD CONSTRAINT "user_problem_solves_active_judgement_fkey" FOREIGN KEY ("active_judgement_id", "active_submission_id", "active_generation", "active_problem_revision_id", "reward_facts_checksum") REFERENCES "submission_judgements" ("id", "submission_id", "generation", "problem_revision_id", "reward_snapshot_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "user_problem_solves_active_submission_fkey" FOREIGN KEY ("active_submission_id", "user_id", "problem_id") REFERENCES "submissions" ("id", "user_id", "problem_id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "user_problem_solves_active_transition_fkey" FOREIGN KEY ("active_reward_transition_id", "id", "version") REFERENCES "user_problem_solve_transitions" ("id", "solve_id", "to_version") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "user_problem_solves_problem_fkey" FOREIGN KEY ("problem_id") REFERENCES "problems" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "user_problem_solves_user_fkey" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "user_recovery_contacts" table
ALTER TABLE "user_recovery_contacts" ADD CONSTRAINT "user_recovery_contacts_users_recovery_contacts" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "user_reward_profile_revisions" table
ALTER TABLE "user_reward_profile_revisions" ADD CONSTRAINT "user_reward_profile_revisions_onboarding_trait_offers_reward_pr" FOREIGN KEY ("source_offer_id") REFERENCES "onboarding_trait_offers" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, ADD CONSTRAINT "user_reward_profile_revisions_users_reward_profile_revisions" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "user_reward_profile_traits" table
ALTER TABLE "user_reward_profile_traits" ADD CONSTRAINT "user_reward_profile_traits_trait_effect_revisions_profile_trait" FOREIGN KEY ("trait_effect_revision_id") REFERENCES "trait_effect_revisions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "user_reward_profile_traits_traits_reward_profile_traits" FOREIGN KEY ("trait_id") REFERENCES "traits" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "user_reward_profile_traits_user_reward_profile_revisions_traits" FOREIGN KEY ("profile_revision_id") REFERENCES "user_reward_profile_revisions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "user_solved_problems" table
ALTER TABLE "user_solved_problems" ADD CONSTRAINT "user_solved_problems_problems_solvers" FOREIGN KEY ("problem_id") REFERENCES "problems" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "user_solved_problems_users_solved_problems" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "user_stats" table
ALTER TABLE "user_stats" ADD CONSTRAINT "user_stats_users_user_stats" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "user_tag_stats" table
ALTER TABLE "user_tag_stats" ADD CONSTRAINT "user_tag_stats_tags_user_tag_stats" FOREIGN KEY ("tag_id") REFERENCES "tags" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "user_tag_stats_users_tag_stats" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "user_traits" table
ALTER TABLE "user_traits" ADD CONSTRAINT "user_traits_traits_user_traits" FOREIGN KEY ("trait_id") REFERENCES "traits" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "user_traits_users_user_traits" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;

-- Enforce append-only storage outside Ent.
CREATE FUNCTION "public"."judgify_reject_append_only_mutation"()
RETURNS trigger
LANGUAGE plpgsql
AS $judgify_append_only$
BEGIN
  RAISE EXCEPTION 'table % is append-only', TG_TABLE_NAME
    USING ERRCODE = '55000';
  RETURN NULL;
END;
$judgify_append_only$;

CREATE TRIGGER "attribute_definition_revisions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."attribute_definition_revisions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "audit_logs_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."audit_logs"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "content_render_artifacts_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."content_render_artifacts"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "content_render_sources_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."content_render_sources"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "contest_content_revisions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."contest_content_revisions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "contest_problems_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."contest_problems"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "contest_registration_transitions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."contest_registration_transitions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "contest_transitions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."contest_transitions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "cultivation_catalog_revisions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."cultivation_catalog_revisions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "cultivation_ranking_privacy_changes_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."cultivation_ranking_privacy_changes"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "difficulty_revisions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."difficulty_revisions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "element_revisions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."element_revisions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "judge_specs_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."judge_specs"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "judge_test_results_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."judge_test_results"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "judge_worker_capabilities_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."judge_worker_capabilities"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "material_revision_tags_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."material_revision_tags"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "material_revisions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."material_revisions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "notification_campaign_explicit_users_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."notification_campaign_explicit_users"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "notification_campaign_revisions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."notification_campaign_revisions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "onboarding_trait_offer_talents_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."onboarding_trait_offer_talents"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "operation_batches_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."operation_batches"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "operation_control_receipts_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."operation_control_receipts"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "problem_revisions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."problem_revisions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "progression_ledger_entries_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."progression_ledger_entries"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "reward_rule_revisions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."reward_rule_revisions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "solve_reward_processed_transitions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."solve_reward_processed_transitions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "submission_judgements_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."submission_judgements"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "tag_revision_elements_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."tag_revision_elements"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "tag_revisions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."tag_revisions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "test_assets_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."test_assets"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "testcase_groups_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."testcase_groups"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "testcase_revisions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."testcase_revisions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "testset_revisions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."testset_revisions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "trait_effect_revisions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."trait_effect_revisions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "user_problem_solve_transitions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."user_problem_solve_transitions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "user_reward_profile_revisions_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."user_reward_profile_revisions"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

CREATE TRIGGER "user_reward_profile_traits_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."user_reward_profile_traits"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."judgify_reject_append_only_mutation"();

-- Keep runtime roles from mutating append-only records.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'judgify_api') THEN
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."attribute_definition_revisions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."audit_logs" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."content_render_artifacts" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."content_render_sources" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."contest_content_revisions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."contest_problems" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."contest_registration_transitions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."contest_transitions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."cultivation_catalog_revisions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."cultivation_ranking_privacy_changes" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."difficulty_revisions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."element_revisions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."judge_specs" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."judge_test_results" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."judge_worker_capabilities" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."material_revision_tags" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."material_revisions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."notification_campaign_explicit_users" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."notification_campaign_revisions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."onboarding_trait_offer_talents" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."operation_batches" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."operation_control_receipts" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."problem_revisions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."progression_ledger_entries" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."reward_rule_revisions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."solve_reward_processed_transitions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."submission_judgements" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."tag_revision_elements" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."tag_revisions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."test_assets" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."testcase_groups" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."testcase_revisions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."testset_revisions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."trait_effect_revisions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."user_problem_solve_transitions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."user_reward_profile_revisions" FROM judgify_api;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."user_reward_profile_traits" FROM judgify_api;
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'judgify_worker') THEN
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."attribute_definition_revisions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."audit_logs" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."content_render_artifacts" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."content_render_sources" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."contest_content_revisions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."contest_problems" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."contest_registration_transitions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."contest_transitions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."cultivation_catalog_revisions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."cultivation_ranking_privacy_changes" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."difficulty_revisions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."element_revisions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."judge_specs" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."judge_test_results" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."judge_worker_capabilities" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."material_revision_tags" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."material_revisions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."notification_campaign_explicit_users" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."notification_campaign_revisions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."onboarding_trait_offer_talents" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."operation_batches" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."operation_control_receipts" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."problem_revisions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."progression_ledger_entries" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."reward_rule_revisions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."solve_reward_processed_transitions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."submission_judgements" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."tag_revision_elements" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."tag_revisions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."test_assets" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."testcase_groups" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."testcase_revisions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."testset_revisions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."trait_effect_revisions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."user_problem_solve_transitions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."user_reward_profile_revisions" FROM judgify_worker;
    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."user_reward_profile_traits" FROM judgify_worker;
    REVOKE INSERT ON TABLE "public"."difficulty_revisions" FROM judgify_worker;
    REVOKE INSERT ON TABLE "public"."element_revisions" FROM judgify_worker;
    REVOKE INSERT ON TABLE "public"."tag_revision_elements" FROM judgify_worker;
    REVOKE INSERT ON TABLE "public"."tag_revisions" FROM judgify_worker;
  END IF;
END;
$$;

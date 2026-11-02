package ent

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/judgify/internal/ent/generate/attributedefinition"
	"github.com/huynhanx03/judgify/internal/ent/generate/attributedefinitionrevision"
	"github.com/huynhanx03/judgify/internal/ent/generate/recoverycontactverificationtoken"
	"github.com/huynhanx03/judgify/internal/ent/generate/securityemaildelivery"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	"github.com/huynhanx03/judgify/internal/ent/generate/userattributevalue"
	"github.com/huynhanx03/judgify/internal/ent/generate/userrecoverycontact"
)

func TestGeneratedIdentityEnumsPreserveInvitationAndPrivateProfileContracts(t *testing.T) {
	t.Parallel()

	require.Equal(t, user.StatusActive, user.DefaultStatus)
	require.NoError(t, user.StatusValidator(user.StatusInvited))
	require.NoError(t, user.StatusValidator(user.StatusActive))
	require.NoError(t, user.StatusValidator(user.StatusSuspended))
	require.NoError(t, user.StatusValidator(user.StatusDeleted))
	require.Error(t, user.StatusValidator(user.Status("pending")))

	require.Equal(t, user.ProfileVisibilityPrivate, user.DefaultProfileVisibility)
	require.NoError(t, user.ProfileVisibilityValidator(user.ProfileVisibilityPrivate))
	require.NoError(t, user.ProfileVisibilityValidator(user.ProfileVisibilityPublic))
	require.Error(t, user.ProfileVisibilityValidator(user.ProfileVisibility("admin")))

	require.NoError(t, securityemaildelivery.KindValidator(
		securityemaildelivery.KindAccountInvitation,
	))
	require.NoError(t, recoverycontactverificationtoken.PurposeValidator(
		recoverycontactverificationtoken.PurposeInvitation,
	))
	require.NoError(t, userrecoverycontact.PurposeValidator(
		userrecoverycontact.PurposeInvitation,
	))
}

func TestGeneratedProfileSchemaKeepsMetadataRevisionedAndValueProvenanceExplicit(t *testing.T) {
	t.Parallel()

	for _, legacyColumn := range []string{"data_type", "description", "visibility"} {
		require.NotContains(t, attributedefinition.Columns, legacyColumn)
	}
	for _, revisionColumn := range []string{
		"definition_id",
		"revision",
		"data_type",
		"label",
		"description",
		"visibility",
		"validation",
	} {
		require.Contains(t, attributedefinitionrevision.Columns, revisionColumn)
	}
	for _, provenanceColumn := range []string{
		"attribute_id",
		"definition_revision_id",
		"source_revision_id",
		"canonical_value",
		"value",
		"version",
	} {
		require.Contains(t, userattributevalue.Columns, provenanceColumn)
	}
}

package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate/attributedefinition"
	"github.com/huynhanx03/judgify/internal/ent/generate/credential"
	"github.com/huynhanx03/judgify/internal/ent/generate/federatedidentity"
	"github.com/huynhanx03/judgify/internal/ent/generate/role"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
)

var (
	attributeDefinitionQueryColumns = []string{
		attributedefinition.FieldID,
		attributedefinition.FieldCreatedAt,
		attributedefinition.FieldUpdatedAt,
		attributedefinition.FieldKey,
		attributedefinition.FieldKind,
		attributedefinition.FieldRequiredOnOnboarding,
		attributedefinition.FieldStatus,
		attributedefinition.FieldVersion,
	}
	credentialQueryColumns = []string{
		credential.FieldID,
		credential.FieldCreatedAt,
		credential.FieldUpdatedAt,
		credential.FieldUserID,
		credential.FieldType,
		credential.FieldVersion,
		credential.FieldActive,
		credential.FieldLastUsedAt,
	}
	federatedIdentityQueryColumns = []string{
		federatedidentity.FieldID,
		federatedidentity.FieldCreatedAt,
		federatedidentity.FieldUpdatedAt,
		federatedidentity.FieldUserID,
		federatedidentity.FieldProvider,
		federatedidentity.FieldDisplayName,
		federatedidentity.FieldLastAuthenticatedAt,
	}
	roleQueryColumns = []string{
		role.FieldID,
		role.FieldCreatedAt,
		role.FieldUpdatedAt,
		role.FieldKey,
		role.FieldName,
		role.FieldIsSystem,
		role.FieldStatus,
		role.FieldVersion,
	}
	userQueryColumns = []string{
		user.FieldID,
		user.FieldCreatedAt,
		user.FieldUpdatedAt,
		user.FieldUsername,
		user.FieldStatus,
		user.FieldSuspendedAt,
	}
)

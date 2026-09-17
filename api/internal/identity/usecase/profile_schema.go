package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/identity/attributevalue"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const profileSchemaDigestDomain = "judgify/identity/profile-schema/v2"

type profileSchemaService struct {
	repository   ports.ProfileSchemaRepository
	transactions tx.Manager
	clock        func() time.Time
}

// ResolveRequiredValues must be called from the account-creation transaction.
// Its shared fence remains held until that transaction commits, preventing a
// lifecycle command from changing the exact required set after validation.
func (service *profileSchemaService) ResolveRequiredValues(
	ctx context.Context,
	userID uuid.UUID,
	evidence dto.ProfileSchemaEvidence,
) ([]ports.RevisionedUserAttributeValueMutation, error) {
	if service == nil || isNilTokenDependency(service.repository) ||
		ctx == nil || ctx.Err() != nil ||
		userID == uuid.Nil || evidence.ProfileSchemaRevision < 1 ||
		len(evidence.ProfileValues) == 0 || len(evidence.ProfileValues) > 128 {
		return nil, apperr.New(
			apperr.CodeBadRequest,
			"invalid profile schema evidence",
			ports.ErrProfileSchemaConflict,
		)
	}
	fence, err := service.repository.LockShared(ctx)
	if err != nil {
		return nil, err
	}
	definitions, err := service.repository.ListRequired(ctx)
	if err != nil {
		return nil, err
	}
	snapshot, err := projectProfileSchema(fence.Revision, definitions, service.clock())
	if err != nil {
		return nil, err
	}
	if fence.Revision != evidence.ProfileSchemaRevision ||
		!strings.EqualFold(snapshot.Checksum, strings.TrimSpace(evidence.ProfileSchemaChecksum)) ||
		len(snapshot.Fields) != len(evidence.ProfileValues) {
		return nil, profileSchemaConflict()
	}
	byID := make(map[uuid.UUID]dto.ProfileValueInput, len(evidence.ProfileValues))
	for _, input := range evidence.ProfileValues {
		if input.DefinitionID == uuid.Nil || input.DefinitionRevisionID == uuid.Nil ||
			len(input.Value) == 0 || len(input.Value) > attributevalue.MaxCanonicalBytes {
			return nil, apperr.New(
				apperr.CodeBadRequest,
				"invalid required profile value",
				ports.ErrProfileSchemaConflict,
			)
		}
		if _, duplicate := byID[input.DefinitionID]; duplicate {
			return nil, apperr.New(
				apperr.CodeBadRequest,
				"duplicate required profile value",
				ports.ErrProfileSchemaConflict,
			)
		}
		byID[input.DefinitionID] = input
	}
	mutations := make([]ports.RevisionedUserAttributeValueMutation, 0, len(definitions))
	for _, definition := range definitions {
		if definition == nil || definition.ActiveRevision == nil ||
			definition.ActiveRevisionID == nil {
			return nil, ports.ErrProfileSchemaUnavailable
		}
		input, exists := byID[definition.ID]
		if !exists || input.DefinitionRevisionID != *definition.ActiveRevisionID {
			return nil, profileSchemaConflict()
		}
		canonical, canonicalErr := attributevalue.Canonicalize(
			input.Value,
			*definition.ActiveRevision,
		)
		if canonicalErr != nil {
			return nil, apperr.New(
				apperr.CodeBadRequest,
				"required profile value does not match schema",
				canonicalErr,
			).WithDetails(map[string]any{
				"definition_id": definition.ID.String(),
				"reason":        "invalid_profile_value",
			})
		}
		if !bytes.Equal(bytes.TrimSpace(input.Value), canonical) {
			// Accept semantically equivalent JSON representations, but persist
			// and fingerprint only the canonical typed form below.
			input.Value = canonical
		}
		mutations = append(mutations, ports.RevisionedUserAttributeValueMutation{
			UserID:                       userID,
			DefinitionID:                 definition.ID,
			ExpectedDefinitionRevisionID: *definition.ActiveRevisionID,
			ExpectedValueVersion:         0,
			CanonicalValue:               append([]byte(nil), canonical...),
		})
	}
	sort.Slice(mutations, func(left, right int) bool {
		return bytes.Compare(mutations[left].DefinitionID[:], mutations[right].DefinitionID[:]) < 0
	})
	logger.FromContext(ctx).Debug(
		"required profile values resolved",
		zap.String("user_id", userID.String()),
		zap.Int64("profile_schema_revision", fence.Revision),
		zap.Int("profile_attribute_count", len(mutations)),
	)
	return mutations, nil
}

func profileSchemaConflict() error {
	return apperr.New(
		apperr.CodeConflict,
		"profile schema changed; reload and try again",
		ports.ErrProfileSchemaConflict,
	).WithDetails(map[string]any{"reason": "profile_schema_changed"})
}

type profileSchemaFingerprintValue struct {
	Revision int64                          `json:"revision"`
	Checksum string                         `json:"checksum"`
	Values   []profileSchemaFingerprintItem `json:"values"`
}

type profileSchemaFingerprintItem struct {
	DefinitionID         uuid.UUID       `json:"definition_id"`
	DefinitionRevisionID uuid.UUID       `json:"definition_revision_id"`
	Value                json.RawMessage `json:"value"`
}

func canonicalProfileSchemaEvidence(
	evidence dto.ProfileSchemaEvidence,
) (profileSchemaFingerprintValue, error) {
	if evidence.ProfileSchemaRevision < 1 ||
		len(evidence.ProfileSchemaChecksum) != sha256.Size*2 ||
		len(evidence.ProfileValues) == 0 || len(evidence.ProfileValues) > 128 {
		return profileSchemaFingerprintValue{}, ports.ErrProfileSchemaConflict
	}
	checksum := strings.ToLower(strings.TrimSpace(evidence.ProfileSchemaChecksum))
	decodedChecksum, err := hex.DecodeString(checksum)
	if err != nil || len(decodedChecksum) != sha256.Size {
		return profileSchemaFingerprintValue{}, ports.ErrProfileSchemaConflict
	}
	items := make([]profileSchemaFingerprintItem, 0, len(evidence.ProfileValues))
	seen := make(map[uuid.UUID]struct{}, len(evidence.ProfileValues))
	for _, input := range evidence.ProfileValues {
		if input.DefinitionID == uuid.Nil || input.DefinitionRevisionID == uuid.Nil {
			return profileSchemaFingerprintValue{}, ports.ErrProfileSchemaConflict
		}
		if _, duplicate := seen[input.DefinitionID]; duplicate {
			return profileSchemaFingerprintValue{}, ports.ErrProfileSchemaConflict
		}
		seen[input.DefinitionID] = struct{}{}
		value, err := canonicalProfileInputJSON(input.Value)
		if err != nil {
			return profileSchemaFingerprintValue{}, err
		}
		items = append(items, profileSchemaFingerprintItem{
			DefinitionID:         input.DefinitionID,
			DefinitionRevisionID: input.DefinitionRevisionID,
			Value:                value,
		})
	}
	sort.Slice(items, func(left, right int) bool {
		return bytes.Compare(items[left].DefinitionID[:], items[right].DefinitionID[:]) < 0
	})
	return profileSchemaFingerprintValue{
		Revision: evidence.ProfileSchemaRevision,
		Checksum: checksum,
		Values:   items,
	}, nil
}

func canonicalProfileInputJSON(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > attributevalue.MaxCanonicalBytes {
		return nil, ports.ErrProfileSchemaConflict
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, ports.ErrProfileSchemaConflict
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, ports.ErrProfileSchemaConflict
	}
	switch value.(type) {
	case string, bool, json.Number:
	default:
		return nil, ports.ErrProfileSchemaConflict
	}
	canonical, err := json.Marshal(value)
	if err != nil || len(canonical) == 0 {
		return nil, ports.ErrProfileSchemaConflict
	}
	return canonical, nil
}

func NewProfileSchemaService(
	repository ports.ProfileSchemaRepository,
	transactions tx.Manager,
) ports.ProfileSchemaService {
	return &profileSchemaService{
		repository:   repository,
		transactions: transactions,
		clock:        func() time.Time { return time.Now().UTC() },
	}
}

func (service *profileSchemaService) GetOnboardingSchema(
	ctx context.Context,
) (*dto.ProfileSchemaResponse, error) {
	if service == nil || isNilTokenDependency(service.repository) ||
		isNilTokenDependency(service.transactions) ||
		ctx == nil || ctx.Err() != nil {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"profile schema unavailable",
			ports.ErrProfileSchemaUnavailable,
		)
	}
	var response *dto.ProfileSchemaResponse
	err := service.transactions.DoInTx(ctx, func(txCtx context.Context) error {
		fence, err := service.repository.LockShared(txCtx)
		if err != nil {
			return err
		}
		definitions, err := service.repository.ListRequired(txCtx)
		if err != nil {
			return err
		}
		response, err = projectProfileSchema(fence.Revision, definitions, service.clock())
		return err
	})
	if err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Debug(
		"onboarding profile schema loaded",
		zap.Int64("profile_schema_revision", response.Revision),
		zap.Int("profile_attribute_count", len(response.Fields)),
	)
	return response, nil
}

func projectProfileSchema(
	revision int64,
	definitions []*entity.AttributeDefinition,
	now time.Time,
) (*dto.ProfileSchemaResponse, error) {
	if revision < 1 || len(definitions) == 0 || len(definitions) > 128 || now.IsZero() {
		return nil, ports.ErrProfileSchemaUnavailable
	}
	fields := make([]dto.ProfileSchemaField, 0, len(definitions))
	definitionIDs := make(map[uuid.UUID]struct{}, len(definitions))
	keys := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		if definition == nil || definition.ID == uuid.Nil ||
			!attributeDefinitionKeyPattern.MatchString(definition.Key) ||
			(definition.Kind != "system" && definition.Kind != "custom") ||
			definition.Status != "active" || !definition.RequiredOnOnboarding ||
			definition.Version < 1 ||
			definition.ActiveRevisionID == nil || definition.ActiveRevision == nil ||
			definition.ActiveRevision.ID != *definition.ActiveRevisionID ||
			definition.ActiveRevision.DefinitionID != definition.ID ||
			validateAttributeRevision(definition.ActiveRevision) != nil ||
			validateRequiredOnboardingRevision(
				true,
				definition.ActiveRevision.Visibility,
				definition.ActiveRevision.UserEditable,
			) != nil {
			return nil, ports.ErrProfileSchemaUnavailable
		}
		if _, duplicate := definitionIDs[definition.ID]; duplicate {
			return nil, ports.ErrProfileSchemaUnavailable
		}
		if _, duplicate := keys[definition.Key]; duplicate {
			return nil, ports.ErrProfileSchemaUnavailable
		}
		definitionIDs[definition.ID] = struct{}{}
		keys[definition.Key] = struct{}{}
		active := definition.ActiveRevision
		fields = append(fields, dto.ProfileSchemaField{
			DefinitionID:         definition.ID,
			DefinitionRevisionID: active.ID,
			Key:                  definition.Key,
			DataType:             string(active.DataType),
			Label:                active.Label,
			Description:          active.Description,
			Visibility:           string(active.Visibility),
			UserEditable:         active.UserEditable,
			DisplayOrder:         active.DisplayOrder,
			Validation:           append([]byte(nil), active.Validation...),
		})
	}
	sort.Slice(fields, func(left, right int) bool {
		if fields[left].DisplayOrder != fields[right].DisplayOrder {
			return fields[left].DisplayOrder < fields[right].DisplayOrder
		}
		return fields[left].Key < fields[right].Key
	})
	payload, err := json.Marshal(struct {
		Revision int64                    `json:"revision"`
		Fields   []dto.ProfileSchemaField `json:"fields"`
	}{Revision: revision, Fields: fields})
	if err != nil || len(payload) == 0 || len(payload) > 1<<20 {
		return nil, ports.ErrProfileSchemaUnavailable
	}
	framed := make([]byte, 0, len(payload)+len(profileSchemaDigestDomain)+1)
	framed = append(framed, profileSchemaDigestDomain...)
	framed = append(framed, 0)
	framed = append(framed, payload...)
	digest := sha256.Sum256(framed)
	return &dto.ProfileSchemaResponse{
		Revision:   revision,
		Checksum:   hex.EncodeToString(digest[:]),
		Fields:     fields,
		ServerTime: now.UTC(),
	}, nil
}

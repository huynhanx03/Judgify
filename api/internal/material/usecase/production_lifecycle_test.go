package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/material/contract"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

func TestValidateLifecycleTransitionRejectsDriftedOrAmbiguousTargets(t *testing.T) {
	t.Parallel()
	draftID := uuid.New()
	publishedID := uuid.New()
	base := entity.Material{
		ID:                  uuid.New(),
		CurrentDraftID:      draftID,
		PublishedRevisionID: &publishedID,
		Status:              string(entity.MaterialPublished),
		Version:             7,
	}
	tests := []struct {
		name     string
		material entity.Material
		action   ports.MaterialLifecycleAction
		target   uuid.UUID
		wantErr  bool
	}{
		{name: "publish exact current draft", material: base, action: ports.MaterialLifecyclePublish, target: draftID},
		{name: "publish stale draft", material: base, action: ports.MaterialLifecyclePublish, target: uuid.New(), wantErr: true},
		{name: "publish already active revision", material: withMaterialDraft(base, publishedID), action: ports.MaterialLifecyclePublish, target: publishedID, wantErr: true},
		{name: "archive published", material: base, action: ports.MaterialLifecycleArchive},
		{name: "archive draft", material: withMaterialState(base, entity.MaterialDraft), action: ports.MaterialLifecycleArchive, wantErr: true},
		{name: "rollback prior revision", material: base, action: ports.MaterialLifecycleRollback, target: uuid.New()},
		{name: "rollback current publication", material: base, action: ports.MaterialLifecycleRollback, target: publishedID, wantErr: true},
		{name: "rollback current draft", material: base, action: ports.MaterialLifecycleRollback, target: draftID, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateLifecycleTransition(test.material, test.action, test.target)
			if test.wantErr && !errors.Is(err, entity.ErrMaterialInvalidLifecycle) {
				t.Fatalf("validateLifecycleTransition() error = %v", err)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("validateLifecycleTransition() error = %v", err)
			}
		})
	}
}

func TestValidateLifecycleTargetPreservesPublishAndRollbackDirection(t *testing.T) {
	t.Parallel()
	before := workflowLifecycleProjection(4, entity.MaterialPublished)
	tests := []struct {
		name         string
		action       ports.MaterialLifecycleAction
		targetNumber int64
		wantErr      bool
	}{
		{name: "publish newer draft", action: ports.MaterialLifecyclePublish, targetNumber: 5},
		{name: "publish older draft", action: ports.MaterialLifecyclePublish, targetNumber: 3, wantErr: true},
		{name: "rollback prior revision", action: ports.MaterialLifecycleRollback, targetNumber: 3},
		{name: "rollback newer revision", action: ports.MaterialLifecycleRollback, targetNumber: 5, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateLifecycleTarget(
				test.action,
				before,
				entity.MaterialRevision{RevisionNumber: test.targetNumber},
			)
			if test.wantErr && !errors.Is(err, entity.ErrMaterialInvalidLifecycle) {
				t.Fatalf("validateLifecycleTarget() error = %v", err)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("validateLifecycleTarget() error = %v", err)
			}
		})
	}
}

func workflowLifecycleProjection(
	revisionNumber int64,
	status entity.MaterialState,
) ports.MaterialLifecycleProjection {
	revisionID := uuid.New()
	return ports.MaterialLifecycleProjection{
		Version:                7,
		Status:                 status,
		Visibility:             entity.MaterialPublic,
		Slug:                   "cay-fenwick",
		RevisionID:             &revisionID,
		RevisionNumber:         revisionNumber,
		Title:                  "Cây Fenwick",
		CategoryID:             uuid.New(),
		DifficultyID:           uuid.New(),
		RenderSourceID:         uuid.New(),
		RenderArtifactID:       uuid.New(),
		SourceChecksum:         "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		RendererVersion:        "renderer-v1",
		SanitizerPolicyVersion: "sanitizer-v1",
	}
}

type materialReceiptRepositoryStub struct {
	ports.MaterialEditorialRepository
	receipt entity.MaterialCommandReceipt
}

func (stub materialReceiptRepositoryStub) LoadCommandReceipt(
	context.Context,
	uuid.UUID,
) (entity.MaterialCommandReceipt, bool, error) {
	return stub.receipt, true, nil
}

func TestMaterialMutationReceiptRejectsChangedRequestHash(t *testing.T) {
	t.Parallel()
	commandID := uuid.New()
	materialID := uuid.New()
	exactHash, err := materialMutationRequestHash(
		"material.slug_changed.v1",
		struct {
			Slug   string `json:"slug"`
			Reason string `json:"reason"`
		}{Slug: "canonical-slug", Reason: "correct reason"},
	)
	if err != nil {
		t.Fatalf("materialMutationRequestHash() error = %v", err)
	}
	payload, err := json.Marshal(materialMutationReceiptPayload{
		MaterialID:  materialID,
		Version:     8,
		RequestHash: exactHash,
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	service := &productionService{repository: materialReceiptRepositoryStub{
		receipt: entity.MaterialCommandReceipt{
			EventID:       commandID,
			EventType:     "material.slug_changed.v1",
			AggregateType: materialAggregate,
			AggregateID:   materialID,
			Version:       8,
			Payload:       payload,
			OccurredAt:    time.Now().UTC(),
		},
	}}
	receipt, found, err := service.loadMaterialMutationReceipt(
		context.Background(), commandID, "material.slug_changed.v1",
		materialID, 8, exactHash, true,
	)
	if err != nil || !found || receipt == nil || !receipt.IdempotentReplay {
		t.Fatalf("exact receipt = %#v, %t, %v", receipt, found, err)
	}
	changedHash, err := materialMutationRequestHash(
		"material.slug_changed.v1",
		struct {
			Slug   string `json:"slug"`
			Reason string `json:"reason"`
		}{Slug: "canonical-slug", Reason: "changed reason"},
	)
	if err != nil {
		t.Fatalf("materialMutationRequestHash(changed) error = %v", err)
	}
	if _, _, err := service.loadMaterialMutationReceipt(
		context.Background(), commandID, "material.slug_changed.v1",
		materialID, 8, changedHash, true,
	); !errors.Is(err, entity.ErrMaterialCommandConflict) {
		t.Fatalf("changed request error = %v", err)
	}
}

func TestMaterialLifecycleReceiptReplaysOnlyTheExactCommittedRequest(t *testing.T) {
	t.Parallel()
	commandID := uuid.New()
	materialID := uuid.New()
	revisionID := uuid.New()
	exactHash := sha256.Sum256([]byte("exact reviewed lifecycle request"))
	projection := contract.MaterialLifecycleProjectionResponse{
		Version:                8,
		Status:                 string(entity.MaterialPublished),
		Visibility:             string(entity.MaterialPublic),
		Slug:                   "cay-fenwick",
		RevisionID:             &revisionID,
		RevisionNumber:         4,
		Title:                  "Cây Fenwick",
		CategoryID:             uuid.New(),
		DifficultyID:           uuid.New(),
		TagIDs:                 []uuid.UUID{uuid.New()},
		RenderSourceID:         uuid.New(),
		RenderArtifactID:       uuid.New(),
		SourceChecksum:         "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		RendererVersion:        "renderer-v1",
		SanitizerPolicyVersion: "sanitizer-v1",
	}
	payload, err := json.Marshal(lifecycleReceiptPayload{
		MaterialID:  materialID,
		Version:     projection.Version,
		After:       projection,
		RequestHash: fmt.Sprintf("%x", exactHash),
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	service := &productionService{repository: materialReceiptRepositoryStub{
		receipt: entity.MaterialCommandReceipt{
			EventID:       commandID,
			EventType:     "material.published.v1",
			AggregateType: materialAggregate,
			AggregateID:   materialID,
			Version:       projection.Version,
			Payload:       payload,
			OccurredAt:    time.Now().UTC(),
		},
	}}
	command := ports.ApplyMaterialLifecycleCommand{
		Action:           ports.MaterialLifecyclePublish,
		MaterialID:       materialID,
		TargetRevisionID: revisionID,
		ExpectedVersion:  7,
		CommandID:        commandID,
		RequestHash:      exactHash,
	}
	receipt, found, err := service.loadLifecycleReceipt(
		context.Background(), command, fmt.Sprintf("%x", exactHash), true,
	)
	if err != nil || !found || receipt == nil || !receipt.IdempotentReplay ||
		receipt.After.RenderArtifactID != projection.RenderArtifactID {
		t.Fatalf("exact lifecycle receipt = %#v, %t, %v", receipt, found, err)
	}
	changedHash := sha256.Sum256([]byte("changed reviewed lifecycle request"))
	command.RequestHash = changedHash
	if _, _, err := service.loadLifecycleReceipt(
		context.Background(), command, fmt.Sprintf("%x", changedHash), true,
	); !errors.Is(err, entity.ErrMaterialCommandConflict) {
		t.Fatalf("changed lifecycle request error = %v", err)
	}
}

func TestMaterialLifecycleReceiptReadsThePersistedMutationPayload(t *testing.T) {
	t.Parallel()
	commandID := uuid.New()
	materialID := uuid.New()
	revisionID := uuid.New()
	requestHash := sha256.Sum256([]byte("persisted lifecycle request"))
	projection := ports.MaterialLifecycleProjection{
		Version: 2, Status: entity.MaterialInReview, Visibility: entity.MaterialPublic,
		Slug: "e2e-test-material", RevisionID: &revisionID, RevisionNumber: 1,
		Title: "E2E Test Material", CategoryID: uuid.New(), DifficultyID: uuid.New(),
		RenderSourceID: uuid.New(), RenderArtifactID: uuid.New(),
		SourceChecksum:  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		RendererVersion: "renderer-v1", SanitizerPolicyVersion: "sanitizer-v1",
	}
	mutation := materialMutation(
		"material.review_submitted.v1",
		entity.Material{ID: materialID, Version: 2},
		"submit for E2E review",
		map[string]any{},
		lifecycleProjectionDocument(projection),
	)
	mutation.Payload["request_hash"] = fmt.Sprintf("%x", requestHash)
	payload, err := json.Marshal(mutation.Payload)
	if err != nil {
		t.Fatal(err)
	}
	service := &productionService{repository: materialReceiptRepositoryStub{receipt: entity.MaterialCommandReceipt{
		EventID: commandID, EventType: mutation.EventType, AggregateType: materialAggregate,
		AggregateID: materialID, Version: 2, Payload: payload, OccurredAt: time.Now().UTC(),
	}}}
	receipt, found, err := service.loadLifecycleReceipt(context.Background(), ports.ApplyMaterialLifecycleCommand{
		Action: ports.MaterialLifecycleSubmitReview, MaterialID: materialID,
		ExpectedVersion: 1, CommandID: commandID, RequestHash: requestHash,
	}, fmt.Sprintf("%x", requestHash), false)
	if err != nil || !found || receipt == nil {
		t.Fatalf("persisted lifecycle receipt = %#v, %t, %v", receipt, found, err)
	}
}

func TestMaterialLifecycleReceiptRejectsZeroChecksumEvidence(t *testing.T) {
	t.Parallel()
	projection := contract.MaterialLifecycleProjectionResponse{
		Version:                2,
		Status:                 string(entity.MaterialPublished),
		Visibility:             string(entity.MaterialPublic),
		Slug:                   "cay-fenwick",
		RevisionID:             uuidPointer(uuid.New()),
		RevisionNumber:         1,
		Title:                  "Cây Fenwick",
		CategoryID:             uuid.New(),
		DifficultyID:           uuid.New(),
		RenderSourceID:         uuid.New(),
		RenderArtifactID:       uuid.New(),
		SourceChecksum:         "0000000000000000000000000000000000000000000000000000000000000000",
		RendererVersion:        "renderer-v1",
		SanitizerPolicyVersion: "sanitizer-v1",
	}
	if validLifecycleReceiptProjection(projection) {
		t.Fatal("zero checksum lifecycle evidence was accepted")
	}
}

func withMaterialState(
	material entity.Material,
	state entity.MaterialState,
) entity.Material {
	material.Status = string(state)
	return material
}

func withMaterialDraft(
	material entity.Material,
	revisionID uuid.UUID,
) entity.Material {
	material.CurrentDraftID = revisionID
	return material
}

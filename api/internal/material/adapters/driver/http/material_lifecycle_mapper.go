package http

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/application/workflow/materiallifecycle"
	"github.com/huynhanx03/judgify/internal/ports"
)

func toPublishMaterialLifecycleRequest(request *publishMaterialRequest) (materiallifecycle.Request, error) {
	materialID, commandID, err := parseMaterialLifecycleIDs(request.ID, request.CommandID)
	if err != nil {
		return materiallifecycle.Request{}, err
	}
	revisionID, err := uuid.Parse(request.DraftRevisionID)
	if err != nil {
		return materiallifecycle.Request{}, fmt.Errorf("parse draft revision id: %w", err)
	}
	return lifecycleRequest(
		ports.MaterialLifecyclePublish,
		materialID,
		revisionID,
		commandID,
		request.ExpectedVersion,
		request.Reason,
		request.ConfirmationToken,
	), nil
}

func toArchiveMaterialLifecycleRequest(
	action ports.MaterialLifecycleAction,
	request *archiveMaterialRequest,
) (materiallifecycle.Request, error) {
	materialID, commandID, err := parseMaterialLifecycleIDs(request.ID, request.CommandID)
	if err != nil {
		return materiallifecycle.Request{}, err
	}
	return lifecycleRequest(
		action,
		materialID,
		uuid.Nil,
		commandID,
		request.ExpectedVersion,
		request.Reason,
		request.ConfirmationToken,
	), nil
}

func toRollbackMaterialLifecycleRequest(request *rollbackMaterialRequest) (materiallifecycle.Request, error) {
	materialID, commandID, err := parseMaterialLifecycleIDs(request.ID, request.CommandID)
	if err != nil {
		return materiallifecycle.Request{}, err
	}
	revisionID, err := uuid.Parse(request.TargetRevisionID)
	if err != nil {
		return materiallifecycle.Request{}, fmt.Errorf("parse target revision id: %w", err)
	}
	return lifecycleRequest(
		ports.MaterialLifecycleRollback,
		materialID,
		revisionID,
		commandID,
		request.ExpectedVersion,
		request.Reason,
		request.ConfirmationToken,
	), nil
}

func toRenameMaterialCommand(request *renameMaterialRequest, actorID uuid.UUID) (ports.RenameMaterialCommand, error) {
	materialID, commandID, err := parseMaterialLifecycleIDs(request.ID, request.CommandID)
	if err != nil {
		return ports.RenameMaterialCommand{}, err
	}
	return ports.RenameMaterialCommand{
		CommandID: commandID, ActorID: actorID, MaterialID: materialID, Slug: request.Slug,
		ExpectedVersion: request.ExpectedVersion, Reason: request.Reason,
	}, nil
}

func parseMaterialLifecycleIDs(rawMaterialID, rawCommandID string) (uuid.UUID, uuid.UUID, error) {
	materialID, err := parseMaterialID(rawMaterialID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	commandID, err := uuid.Parse(rawCommandID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse material command id: %w", err)
	}
	return materialID, commandID, nil
}

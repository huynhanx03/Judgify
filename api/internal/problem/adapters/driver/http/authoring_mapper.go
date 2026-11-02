package http

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

func toSaveDraftCommand(request *SaveProblemDraftRequest) (ports.SaveDraftCommand, error) {
	problemID, err := uuid.Parse(request.ID)
	if err != nil {
		return ports.SaveDraftCommand{}, fmt.Errorf("parse problem id: %w", err)
	}
	difficultyID, err := uuid.Parse(request.DifficultyID)
	if err != nil {
		return ports.SaveDraftCommand{}, fmt.Errorf("parse difficulty id: %w", err)
	}
	tagIDs, err := parseProblemTagIDs(request.TagIDs)
	if err != nil {
		return ports.SaveDraftCommand{}, err
	}
	groups := make([]ports.DraftTestcaseGroup, len(request.Groups))
	for groupIndex, group := range request.Groups {
		groups[groupIndex].StopOnFailure = group.StopOnFailure
		groups[groupIndex].Cases = make([]ports.DraftTestcase, len(group.Cases))
		for caseIndex, testcase := range group.Cases {
			var testCaseID *uuid.UUID
			if testcase.TestCaseID != nil {
				id, err := uuid.Parse(*testcase.TestCaseID)
				if err != nil {
					return ports.SaveDraftCommand{}, fmt.Errorf("parse testcase id at group %d case %d: %w", groupIndex, caseIndex, err)
				}
				testCaseID = &id
			}
			groups[groupIndex].Cases[caseIndex] = ports.DraftTestcase{TestCaseID: testCaseID, Kind: testcase.Kind, Input: []byte(testcase.Input), ExpectedOutput: []byte(testcase.ExpectedOutput)}
		}
	}
	return ports.SaveDraftCommand{
		ProblemID: problemID, ExpectedVersion: request.ExpectedVersion, CatalogVersion: request.CatalogVersion,
		Title: request.Title, StatementMarkdown: []byte(request.StatementMarkdown),
		Reward:     ports.RewardIntent{DifficultyID: difficultyID, TagIDs: tagIDs},
		Limits:     entity.Limits{CPUTimeMilliseconds: request.CPUTimeMs, WallTimeMilliseconds: request.WallTimeMs, MemoryKilobytes: request.MemoryLimitKb, OutputBytes: request.OutputLimitBytes, ProcessCount: request.ProcessLimit},
		CheckerKey: request.CheckerKey, CheckerVersion: request.CheckerVersion, CheckerConfig: append([]byte(nil), request.CheckerConfig...), AllowedRuntimeKeys: append([]string(nil), request.AllowedRuntimeKeys...), Groups: groups, Reason: request.Reason,
	}, nil
}

func toPublishCommand(request *PublishProblemRequest) (ports.PublishCommand, error) {
	problemID, err := uuid.Parse(request.ID)
	if err != nil {
		return ports.PublishCommand{}, fmt.Errorf("parse problem id: %w", err)
	}
	draftID, err := uuid.Parse(request.DraftRevisionID)
	if err != nil {
		return ports.PublishCommand{}, fmt.Errorf("parse draft revision id: %w", err)
	}
	testsetID, err := uuid.Parse(request.TestsetRevisionID)
	if err != nil {
		return ports.PublishCommand{}, fmt.Errorf("parse testset revision id: %w", err)
	}
	return ports.PublishCommand{ProblemID: problemID, DraftRevisionID: draftID, TestsetRevisionID: testsetID, ExpectedVersion: request.ExpectedVersion, CatalogVersion: request.CatalogVersion, Reason: request.Reason}, nil
}

func toArchiveCommand(request *ArchiveProblemRequest) (ports.ArchiveCommand, error) {
	problemID, err := uuid.Parse(request.ID)
	if err != nil {
		return ports.ArchiveCommand{}, fmt.Errorf("parse problem id: %w", err)
	}
	return ports.ArchiveCommand{ProblemID: problemID, ExpectedVersion: request.ExpectedVersion, Reason: request.Reason}, nil
}

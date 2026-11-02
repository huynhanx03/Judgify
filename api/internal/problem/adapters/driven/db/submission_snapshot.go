package db

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"

	"github.com/google/uuid"

	intake "github.com/huynhanx03/judgify/internal/application/workflow/submissionintake"
	dbent "github.com/huynhanx03/judgify/internal/ent"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
)

type SubmissionSnapshotResolver struct {
	client *dbent.EntClient
}

func NewSubmissionSnapshotResolver(
	client *dbent.EntClient,
) (*SubmissionSnapshotResolver, error) {
	if client == nil {
		return nil, intake.ErrProblemUnavailable
	}
	return &SubmissionSnapshotResolver{client: client}, nil
}

func (resolver *SubmissionSnapshotResolver) Resolve(
	ctx context.Context,
	request intake.ProblemRequest,
) (intake.ProblemSnapshot, error) {
	if resolver == nil || resolver.client == nil || ctx == nil ||
		!validSubmissionProblemRequest(request) {
		return intake.ProblemSnapshot{}, intake.ErrProblemUnavailable
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return intake.ProblemSnapshot{}, err
	}

	query := ordinarySubmissionSnapshotSQL
	arguments := []any{request.ProblemID}
	if request.ExpectedProblemRevisionID != nil {
		query = exactSubmissionSnapshotSQL
		arguments = append(
			arguments,
			*request.ExpectedProblemRevisionID,
			*request.ExpectedTestsetRevisionID,
		)
	}
	rows, err := resolver.client.DB(ctx).Problem.QueryContext(
		ctx,
		query,
		arguments...,
	)
	if err != nil {
		return intake.ProblemSnapshot{}, fmt.Errorf(
			"lock submission problem snapshot: %w",
			err,
		)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return intake.ProblemSnapshot{}, fmt.Errorf(
				"lock submission problem snapshot: %w",
				err,
			)
		}
		return intake.ProblemSnapshot{}, intake.ErrProblemUnavailable
	}

	var result intake.ProblemSnapshot
	var runtimes []byte
	var rewardChecksum []byte
	if err := rows.Scan(
		&result.ProblemID,
		&result.ProblemRevisionID,
		&result.TestsetRevisionID,
		&result.CheckerKey,
		&result.CheckerVersion,
		&result.CheckerConfig,
		&runtimes,
		&result.RewardSnapshot,
		&rewardChecksum,
		&result.CPUTimeMilliseconds,
		&result.WallTimeMilliseconds,
		&result.MemoryKilobytes,
		&result.OutputBytes,
		&result.ProcessCount,
		&result.TestCount,
	); err != nil {
		return intake.ProblemSnapshot{}, fmt.Errorf(
			"scan submission problem snapshot: %w",
			err,
		)
	}
	if rows.Next() {
		return intake.ProblemSnapshot{}, intake.ErrProblemUnavailable
	}
	if err := rows.Err(); err != nil {
		return intake.ProblemSnapshot{}, fmt.Errorf(
			"read submission problem snapshot: %w",
			err,
		)
	}
	if len(rewardChecksum) != len(result.RewardSnapshotChecksum) ||
		json.Unmarshal(runtimes, &result.AllowedRuntimeKeys) != nil {
		return intake.ProblemSnapshot{}, intake.ErrProblemUnavailable
	}
	copy(result.RewardSnapshotChecksum[:], rewardChecksum)
	if !validSubmissionProblemSnapshot(request, result) {
		return intake.ProblemSnapshot{}, intake.ErrProblemUnavailable
	}
	return result, nil
}

const ordinarySubmissionSnapshotSQL = `
SELECT p.id, pr.id, pr.testset_revision_id, pr.checker_key, pr.checker_version,
       pr.checker_config, pr.allowed_runtime_keys,
       jsonb_build_object(
         'difficulty_id', pr.difficulty_id,
         'difficulty_revision', pr.difficulty_revision,
         'base_exp', pr.base_exp,
         'tags', pr.tag_snapshot,
         'elements', pr.element_snapshot
       ),
       pr.reward_snapshot_checksum, pr.cpu_time_limit_ms, pr.wall_time_limit_ms,
       pr.memory_limit_kb, pr.output_limit_bytes, pr.process_limit, ts.test_count
FROM problems AS p
JOIN problem_revisions AS pr
  ON pr.id = p.published_revision_id
 AND pr.problem_id = p.id
JOIN testset_revisions AS ts
  ON ts.id = pr.testset_revision_id
 AND ts.problem_id = p.id
WHERE p.id = $1
  AND p.lifecycle = 'published'
  AND p.deleted_at IS NULL
FOR SHARE OF p`

const exactSubmissionSnapshotSQL = `
SELECT p.id, pr.id, pr.testset_revision_id, pr.checker_key, pr.checker_version,
       pr.checker_config, pr.allowed_runtime_keys,
       jsonb_build_object(
         'difficulty_id', pr.difficulty_id,
         'difficulty_revision', pr.difficulty_revision,
         'base_exp', pr.base_exp,
         'tags', pr.tag_snapshot,
         'elements', pr.element_snapshot
       ),
       pr.reward_snapshot_checksum, pr.cpu_time_limit_ms, pr.wall_time_limit_ms,
       pr.memory_limit_kb, pr.output_limit_bytes, pr.process_limit, ts.test_count
FROM problems AS p
JOIN problem_revisions AS pr
  ON pr.id = $2
 AND pr.problem_id = p.id
 AND pr.testset_revision_id = $3
JOIN testset_revisions AS ts
  ON ts.id = pr.testset_revision_id
 AND ts.problem_id = p.id
WHERE p.id = $1
FOR SHARE OF p`

func validSubmissionProblemRequest(request intake.ProblemRequest) bool {
	if request.ProblemID == uuid.Nil ||
		strings.TrimSpace(request.RuntimeKey) == "" ||
		(request.ExpectedProblemRevisionID == nil) !=
			(request.ExpectedTestsetRevisionID == nil) {
		return false
	}
	if request.ExpectedProblemRevisionID == nil {
		return request.ExpectedCheckerKey == "" &&
			request.ExpectedCheckerVersion == "" &&
			len(request.ExpectedCheckerConfig) == 0 &&
			len(request.ExpectedAllowedRuntimeKeys) == 0 &&
			request.ExpectedSnapshotChecksum == nil
	}
	return *request.ExpectedProblemRevisionID != uuid.Nil &&
		*request.ExpectedTestsetRevisionID != uuid.Nil &&
		request.ExpectedCheckerKey != "" &&
		request.ExpectedCheckerVersion != "" &&
		jsonObject(request.ExpectedCheckerConfig) &&
		validRuntimeSet(request.ExpectedAllowedRuntimeKeys) &&
		request.ExpectedSnapshotChecksum != nil &&
		*request.ExpectedSnapshotChecksum != ([32]byte{})
}

func validSubmissionProblemSnapshot(
	request intake.ProblemRequest,
	result intake.ProblemSnapshot,
) bool {
	if result.ProblemID != request.ProblemID ||
		result.ProblemRevisionID == uuid.Nil ||
		result.TestsetRevisionID == uuid.Nil ||
		result.CheckerKey == "" ||
		result.CheckerVersion == "" ||
		!jsonObject(result.CheckerConfig) ||
		!validRewardSnapshot(result.RewardSnapshot, result.RewardSnapshotChecksum) ||
		!validRuntimeSet(result.AllowedRuntimeKeys) ||
		!containsRuntime(result.AllowedRuntimeKeys, request.RuntimeKey) ||
		result.CPUTimeMilliseconds < 1 ||
		result.WallTimeMilliseconds < result.CPUTimeMilliseconds ||
		result.MemoryKilobytes < 1 ||
		result.OutputBytes < 1 ||
		result.ProcessCount < 1 ||
		result.TestCount < 1 {
		return false
	}
	if request.ExpectedProblemRevisionID == nil {
		return true
	}
	return result.ProblemRevisionID == *request.ExpectedProblemRevisionID &&
		result.TestsetRevisionID == *request.ExpectedTestsetRevisionID &&
		result.CheckerKey == request.ExpectedCheckerKey &&
		result.CheckerVersion == request.ExpectedCheckerVersion &&
		jsonDocumentsEqual(result.CheckerConfig, request.ExpectedCheckerConfig) &&
		slices.Equal(
			result.AllowedRuntimeKeys,
			request.ExpectedAllowedRuntimeKeys,
		) &&
		result.RewardSnapshotChecksum == *request.ExpectedSnapshotChecksum
}

func validRewardSnapshot(value json.RawMessage, checksum [32]byte) bool {
	document := struct {
		DifficultyID       uuid.UUID                             `json:"difficulty_id"`
		DifficultyRevision int64                                 `json:"difficulty_revision"`
		BaseExperience     int64                                 `json:"base_exp"`
		Tags               problementity.TagMappingSnapshot      `json:"tags"`
		Elements           *problementity.ElementMappingSnapshot `json:"elements"`
	}{}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&document) != nil {
		return false
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return false
	}
	return (problementity.RewardSnapshot{
		DifficultyID:       document.DifficultyID,
		DifficultyRevision: document.DifficultyRevision,
		BaseExperience:     document.BaseExperience,
		Tags:               document.Tags,
		Elements:           document.Elements,
		Checksum:           checksum,
	}).Validate() == nil
}

func validRuntimeSet(values []string) bool {
	if len(values) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" || value != strings.TrimSpace(value) {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func containsRuntime(values []string, want string) bool {
	return slices.Contains(values, want)
}

func jsonObject(value json.RawMessage) bool {
	var document any
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	if decoder.Decode(&document) != nil {
		return false
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return false
	}
	_, object := document.(map[string]any)
	return object
}

func jsonDocumentsEqual(left, right json.RawMessage) bool {
	decode := func(value json.RawMessage) (any, error) {
		var document any
		decoder := json.NewDecoder(bytes.NewReader(value))
		decoder.UseNumber()
		if err := decoder.Decode(&document); err != nil {
			return nil, err
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			return nil, errors.New("invalid trailing JSON data")
		}
		return document, nil
	}
	leftDocument, leftErr := decode(left)
	rightDocument, rightErr := decode(right)
	return leftErr == nil &&
		rightErr == nil &&
		reflect.DeepEqual(leftDocument, rightDocument)
}

var _ intake.ProblemResolver = (*SubmissionSnapshotResolver)(nil)

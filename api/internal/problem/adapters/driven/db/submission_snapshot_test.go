package db

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	intake "github.com/huynhanx03/judgify/internal/application/workflow/submissionintake"
	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
)

func TestValidRewardSnapshotVerifiesCanonicalFactsAndChecksum(t *testing.T) {
	t.Parallel()

	reward, encoded := submissionRewardFixture(t)
	require.True(t, validRewardSnapshot(encoded, reward.Checksum))

	tampered := append(json.RawMessage(nil), encoded...)
	var document map[string]any
	require.NoError(t, json.Unmarshal(tampered, &document))
	document["base_exp"] = float64(reward.BaseExperience + 1)
	tampered, err := json.Marshal(document)
	require.NoError(t, err)
	require.False(t, validRewardSnapshot(tampered, reward.Checksum))

	document["unexpected"] = true
	unknownField, err := json.Marshal(document)
	require.NoError(t, err)
	require.False(t, validRewardSnapshot(unknownField, reward.Checksum))
}

func TestValidSubmissionProblemSnapshotEnforcesExactRewardFence(t *testing.T) {
	t.Parallel()

	reward, encoded := submissionRewardFixture(t)
	problemID := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	problemRevisionID := uuid.MustParse("00000000-0000-0000-0000-000000000102")
	testsetRevisionID := uuid.MustParse("00000000-0000-0000-0000-000000000103")
	request := intake.ProblemRequest{
		ProblemID:                  problemID,
		ExpectedProblemRevisionID:  &problemRevisionID,
		ExpectedTestsetRevisionID:  &testsetRevisionID,
		ExpectedCheckerKey:         "token",
		ExpectedCheckerVersion:     "1",
		ExpectedCheckerConfig:      json.RawMessage(`{}`),
		ExpectedAllowedRuntimeKeys: []string{"go"},
		ExpectedSnapshotChecksum:   &reward.Checksum,
		RuntimeKey:                 "go",
	}
	result := intake.ProblemSnapshot{
		ProblemID:              problemID,
		ProblemRevisionID:      problemRevisionID,
		TestsetRevisionID:      testsetRevisionID,
		CheckerKey:             "token",
		CheckerVersion:         "1",
		CheckerConfig:          json.RawMessage(`{}`),
		AllowedRuntimeKeys:     []string{"go"},
		RewardSnapshot:         encoded,
		RewardSnapshotChecksum: reward.Checksum,
		CPUTimeMilliseconds:    1,
		WallTimeMilliseconds:   1,
		MemoryKilobytes:        1,
		OutputBytes:            1,
		ProcessCount:           1,
		TestCount:              1,
	}
	require.True(t, validSubmissionProblemSnapshot(request, result))

	request.ExpectedSnapshotChecksum = &[32]byte{1}
	require.False(t, validSubmissionProblemSnapshot(request, result))
}

func submissionRewardFixture(t *testing.T) (problementity.RewardSnapshot, json.RawMessage) {
	t.Helper()

	reward, err := problementity.BuildRewardSnapshot(
		problementity.RewardMappingIdentity{
			TagMappingID:     uuid.MustParse("00000000-0000-0000-0000-000000000201"),
			ElementMappingID: uuid.MustParse("00000000-0000-0000-0000-000000000202"),
		},
		1,
		problementity.RewardFacts{
			Difficulty: problementity.DifficultyRewardFact{
				ID:             uuid.MustParse("00000000-0000-0000-0000-000000000203"),
				Revision:       1,
				BaseExperience: 100,
			},
			Tags: []problementity.TaxonomyRevisionFact{{
				ID:       uuid.MustParse("00000000-0000-0000-0000-000000000204"),
				Revision: 1,
			}},
			Elements: []problementity.TaxonomyRevisionFact{{
				ID:       uuid.MustParse("00000000-0000-0000-0000-000000000205"),
				Revision: 1,
			}},
		},
	)
	require.NoError(t, err)
	encoded, err := json.Marshal(struct {
		DifficultyID       uuid.UUID                             `json:"difficulty_id"`
		DifficultyRevision int64                                 `json:"difficulty_revision"`
		BaseExperience     int64                                 `json:"base_exp"`
		Tags               problementity.TagMappingSnapshot      `json:"tags"`
		Elements           *problementity.ElementMappingSnapshot `json:"elements"`
	}{
		DifficultyID:       reward.DifficultyID,
		DifficultyRevision: reward.DifficultyRevision,
		BaseExperience:     reward.BaseExperience,
		Tags:               reward.Tags,
		Elements:           reward.Elements,
	})
	require.NoError(t, err)
	return reward, encoded
}

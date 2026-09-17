package domain

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

func TestLimitsValidateAcceptsEveryBoundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		limits Limits
	}{
		{
			name: "all minimums and wall equals cpu",
			limits: Limits{
				CPUTimeMilliseconds:  problemcontract.MinimumCPUTimeLimitMilliseconds,
				WallTimeMilliseconds: problemcontract.MinimumCPUTimeLimitMilliseconds,
				MemoryKilobytes:      problemcontract.MinimumMemoryLimitKilobytes,
				OutputBytes:          problemcontract.MinimumOutputLimitBytes,
				ProcessCount:         problemcontract.MinimumProcessLimit,
			},
		},
		{
			name: "all maximums",
			limits: Limits{
				CPUTimeMilliseconds:  problemcontract.MaximumCPUTimeLimitMilliseconds,
				WallTimeMilliseconds: problemcontract.MaximumWallTimeLimitMilliseconds,
				MemoryKilobytes:      problemcontract.MaximumMemoryLimitKilobytes,
				OutputBytes:          problemcontract.MaximumOutputLimitBytes,
				ProcessCount:         problemcontract.MaximumProcessLimit,
			},
		},
		{
			name: "cpu maximum and wall equals cpu",
			limits: Limits{
				CPUTimeMilliseconds:  problemcontract.MaximumCPUTimeLimitMilliseconds,
				WallTimeMilliseconds: problemcontract.MaximumCPUTimeLimitMilliseconds,
				MemoryKilobytes:      problemcontract.MinimumMemoryLimitKilobytes,
				OutputBytes:          problemcontract.MinimumOutputLimitBytes,
				ProcessCount:         problemcontract.MinimumProcessLimit,
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.limits.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestLimitsValidateRejectsEveryOutOfRangeBoundary(t *testing.T) {
	t.Parallel()

	valid := Limits{
		CPUTimeMilliseconds:  100,
		WallTimeMilliseconds: 200,
		MemoryKilobytes:      problemcontract.MinimumMemoryLimitKilobytes,
		OutputBytes:          problemcontract.MinimumOutputLimitBytes,
		ProcessCount:         problemcontract.MinimumProcessLimit,
	}
	tests := []struct {
		name   string
		mutate func(*Limits)
		field  string
	}{
		{"cpu below minimum", func(value *Limits) {
			value.CPUTimeMilliseconds = problemcontract.MinimumCPUTimeLimitMilliseconds - 1
		}, "cpu_time_limit_ms"},
		{"cpu above maximum", func(value *Limits) {
			value.CPUTimeMilliseconds = problemcontract.MaximumCPUTimeLimitMilliseconds + 1
			value.WallTimeMilliseconds = problemcontract.MaximumWallTimeLimitMilliseconds
		}, "cpu_time_limit_ms"},
		{"wall below cpu", func(value *Limits) {
			value.WallTimeMilliseconds = value.CPUTimeMilliseconds - 1
		}, "wall_time_limit_ms"},
		{"wall above maximum", func(value *Limits) {
			value.WallTimeMilliseconds = problemcontract.MaximumWallTimeLimitMilliseconds + 1
		}, "wall_time_limit_ms"},
		{"memory below minimum", func(value *Limits) {
			value.MemoryKilobytes = problemcontract.MinimumMemoryLimitKilobytes - 1
		}, "memory_limit_kb"},
		{"memory above maximum", func(value *Limits) {
			value.MemoryKilobytes = problemcontract.MaximumMemoryLimitKilobytes + 1
		}, "memory_limit_kb"},
		{"output below minimum", func(value *Limits) {
			value.OutputBytes = problemcontract.MinimumOutputLimitBytes - 1
		}, "output_limit_bytes"},
		{"output above maximum", func(value *Limits) {
			value.OutputBytes = problemcontract.MaximumOutputLimitBytes + 1
		}, "output_limit_bytes"},
		{"processes below minimum", func(value *Limits) {
			value.ProcessCount = problemcontract.MinimumProcessLimit - 1
		}, "process_limit"},
		{"processes above maximum", func(value *Limits) {
			value.ProcessCount = problemcontract.MaximumProcessLimit + 1
		}, "process_limit"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := valid
			test.mutate(&value)
			err := value.Validate()
			var validationError *ValidationError
			if !errors.As(err, &validationError) {
				t.Fatalf("Validate() error = %v, want ValidationError", err)
			}
			if validationError.Field != test.field || validationError.Reason != "out_of_range" {
				t.Fatalf("Validate() error = %#v, want field %q/out_of_range", validationError, test.field)
			}
		})
	}
}

func TestCanonicalizeCheckerConfigIsDeterministicOwnedAndBounded(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{"z":1,"a":{"d":true,"c":[null,"x"]}}`)
	canonical, err := CanonicalizeCheckerConfig(input)
	if err != nil {
		t.Fatalf("CanonicalizeCheckerConfig() error = %v", err)
	}
	const want = `{"a":{"c":[null,"x"],"d":true},"z":1}`
	if string(canonical) != want {
		t.Fatalf("canonical config = %s, want %s", canonical, want)
	}
	again, err := CanonicalizeCheckerConfig(canonical)
	if err != nil {
		t.Fatalf("second CanonicalizeCheckerConfig() error = %v", err)
	}
	if !bytes.Equal(again, canonical) {
		t.Fatalf("canonicalization is not idempotent: first=%s second=%s", canonical, again)
	}

	for index := range input {
		input[index] = 'x'
	}
	if string(canonical) != want {
		t.Fatalf("canonical output aliases caller input: got %s", canonical)
	}

	maximumInput := json.RawMessage(`{"value":"` +
		strings.Repeat("x", problemcontract.MaximumCheckerConfigBytes-len(`{"value":""}`)) +
		`"}`)
	if len(maximumInput) != problemcontract.MaximumCheckerConfigBytes {
		t.Fatalf("test setup input length = %d", len(maximumInput))
	}
	if _, err := CanonicalizeCheckerConfig(maximumInput); err != nil {
		t.Fatalf("maximum-sized canonical object rejected: %v", err)
	}
}

func TestCanonicalizeCheckerConfigRejectsDuplicateDepthAndInvalidShapes(t *testing.T) {
	t.Parallel()

	if _, err := CanonicalizeCheckerConfig(nestedJSONObject(32)); err != nil {
		t.Fatalf("maximum JSON depth rejected: %v", err)
	}
	tests := []struct {
		name  string
		value json.RawMessage
	}{
		{"empty input", nil},
		{"top level duplicate", json.RawMessage(`{"a":1,"a":2}`)},
		{"nested duplicate", json.RawMessage(`{"nested":{"a":1,"a":2}}`)},
		{"too deep", nestedJSONObject(33)},
		{"array root", json.RawMessage(`[]`)},
		{"scalar root", json.RawMessage(`true`)},
		{"trailing value", json.RawMessage(`{} {}`)},
		{"malformed", json.RawMessage(`{"a":`)},
		{"over byte limit", json.RawMessage(`{"value":"` +
			strings.Repeat("x", problemcontract.MaximumCheckerConfigBytes) + `"}`)},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := CanonicalizeCheckerConfig(test.value)
			if !errors.Is(err, ErrNonCanonicalJSON) {
				t.Fatalf("CanonicalizeCheckerConfig() error = %v, want ErrNonCanonicalJSON", err)
			}
		})
	}
}

func TestRuntimeKeysMustBeSortedUniqueAndProtocolSafe(t *testing.T) {
	t.Parallel()

	maximum := make([]string, problemcontract.MaximumAllowedRuntimes)
	for index := range maximum {
		maximum[index] = fmt.Sprintf("runtime%02d", index)
	}
	maximumLengthKey := "r" + strings.Repeat("1", problemcontract.MaximumRuntimeKeyBytes-1)
	valid := [][]string{
		{"go1.25"},
		{"go1.25", "python3.13", "rust1.90"},
		maximum,
		{maximumLengthKey},
	}
	for _, values := range valid {
		if err := validateRuntimeKeys(values); err != nil {
			t.Errorf("validateRuntimeKeys(%q) error = %v", values, err)
		}
	}

	tests := []struct {
		name   string
		values []string
	}{
		{"empty", nil},
		{"too many", append(append([]string(nil), maximum...), "runtime99")},
		{"unsorted", []string{"python3.13", "go1.25"}},
		{"duplicate", []string{"go1.25", "go1.25"}},
		{"uppercase", []string{"Go1.25"}},
		{"leading separator", []string{".go1.25"}},
		{"space", []string{"go 1.25"}},
		{"too long", []string{"r" + strings.Repeat("1", problemcontract.MaximumRuntimeKeyBytes)}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := validateRuntimeKeys(test.values); err == nil {
				t.Fatalf("validateRuntimeKeys(%q) unexpectedly succeeded", test.values)
			}
		})
	}
}

func TestRewardSnapshotsRequireSortedUniqueCatalogItemsAndExactWeights(t *testing.T) {
	t.Parallel()

	valid := validTask2Reward(t)
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid reward Validate() error = %v", err)
	}
	first, err := valid.ComputeChecksum()
	if err != nil {
		t.Fatalf("ComputeChecksum() error = %v", err)
	}
	second, err := valid.ComputeChecksum()
	if err != nil || first != second {
		t.Fatalf("reward checksum is not deterministic: first=%x second=%x err=%v", first, second, err)
	}

	tests := []struct {
		name   string
		mutate func(*RewardSnapshot)
	}{
		{"tag items unsorted", func(value *RewardSnapshot) {
			value.Tags.Items[0], value.Tags.Items[1] = value.Tags.Items[1], value.Tags.Items[0]
		}},
		{"tag items duplicate", func(value *RewardSnapshot) {
			value.Tags.Items[1].TagID = value.Tags.Items[0].TagID
		}},
		{"tag item nil", func(value *RewardSnapshot) {
			value.Tags.Items[0].TagID = uuid.Nil
		}},
		{"tag revision below minimum", func(value *RewardSnapshot) {
			value.Tags.Items[0].TagRevision = 0
		}},
		{"tag revision above maximum", func(value *RewardSnapshot) {
			value.Tags.Items[0].TagRevision = problemcontract.MaximumRewardMappingRevision + 1
		}},
		{"element items unsorted", func(value *RewardSnapshot) {
			value.Elements.Items[0], value.Elements.Items[1] =
				value.Elements.Items[1], value.Elements.Items[0]
		}},
		{"element items duplicate", func(value *RewardSnapshot) {
			value.Elements.Items[1].ElementID = value.Elements.Items[0].ElementID
		}},
		{"element weight zero", func(value *RewardSnapshot) {
			value.Elements.Items[0].WeightBasisPoints = 0
			value.Elements.Items[1].WeightBasisPoints = problemcontract.MaximumRewardWeightBasisPoints
		}},
		{"element weight above maximum", func(value *RewardSnapshot) {
			value.Elements.Items[0].WeightBasisPoints = problemcontract.MaximumRewardWeightBasisPoints + 1
		}},
		{"element weight total below exact", func(value *RewardSnapshot) {
			value.Elements.Items[0].WeightBasisPoints--
		}},
		{"element weight total above exact", func(value *RewardSnapshot) {
			value.Elements.Items[0].WeightBasisPoints++
		}},
		{"empty non-nil element mapping", func(value *RewardSnapshot) {
			value.Elements.Items = nil
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := cloneTask2Reward(valid)
			test.mutate(&value)
			if _, err := value.ComputeChecksum(); err == nil {
				t.Fatal("ComputeChecksum() unexpectedly succeeded")
			}
		})
	}
}

func TestRewardSnapshotCollectionBoundaries(t *testing.T) {
	t.Parallel()

	singleElement := validTask2Reward(t)
	singleElement.Tags.Items = nil
	singleElement.Elements.Items = []ElementSnapshotItem{{
		ElementID:         task2UUID(1999),
		ElementRevision:   1,
		WeightBasisPoints: problemcontract.MaximumRewardWeightBasisPoints,
	}}
	singleChecksum, err := singleElement.ComputeChecksum()
	if err != nil {
		t.Fatalf("single maximum-weight element rejected: %v", err)
	}
	singleElement.Checksum = singleChecksum
	if err := singleElement.Validate(); err != nil {
		t.Fatalf("empty tags/single element reward Validate() error = %v", err)
	}

	reward := validTask2Reward(t)
	reward.Tags.Items = make([]TagSnapshotItem, problemcontract.MaximumTags)
	for index := range reward.Tags.Items {
		reward.Tags.Items[index] = TagSnapshotItem{
			TagID:       task2UUID(uint64(1000 + index)),
			TagRevision: problemcontract.MaximumRewardMappingRevision,
		}
	}
	reward.Elements.Items = make([]ElementSnapshotItem, problemcontract.MaximumElements)
	for index := range reward.Elements.Items {
		weight := problemcontract.MaximumRewardWeightBasisPoints / problemcontract.MaximumElements
		if index == len(reward.Elements.Items)-1 {
			weight += problemcontract.MaximumRewardWeightBasisPoints -
				weight*problemcontract.MaximumElements
		}
		reward.Elements.Items[index] = ElementSnapshotItem{
			ElementID:         task2UUID(uint64(2000 + index)),
			ElementRevision:   problemcontract.MaximumRewardMappingRevision,
			WeightBasisPoints: weight,
		}
	}
	checksum, err := reward.ComputeChecksum()
	if err != nil {
		t.Fatalf("maximum collection boundaries rejected: %v", err)
	}
	reward.Checksum = checksum
	if err := reward.Validate(); err != nil {
		t.Fatalf("maximum collection reward Validate() error = %v", err)
	}

	tooManyTags := cloneTask2Reward(reward)
	tooManyTags.Tags.Items = append(tooManyTags.Tags.Items, TagSnapshotItem{
		TagID: task2UUID(9999), TagRevision: 1,
	})
	if _, err := tooManyTags.ComputeChecksum(); err == nil {
		t.Fatal("tag collection above maximum unexpectedly accepted")
	}

	tooManyElements := cloneTask2Reward(reward)
	tooManyElements.Elements.Items = append(tooManyElements.Elements.Items, ElementSnapshotItem{
		ElementID: task2UUID(9999), ElementRevision: 1, WeightBasisPoints: 1,
	})
	if _, err := tooManyElements.ComputeChecksum(); err == nil {
		t.Fatal("element collection above maximum unexpectedly accepted")
	}
}

func TestRewardSnapshotAcceptsAndRejectsEveryScalarBoundary(t *testing.T) {
	t.Parallel()

	minimum := validTask2Reward(t)
	minimum.DifficultyRevision = 1
	minimum.BaseExperience = 0
	minimum.Tags.Revision = 1
	minimum.Tags.Items[0].TagRevision = 1
	minimum.Elements.Revision = 1
	minimum.Elements.Items = []ElementSnapshotItem{
		{
			ElementID:         task2UUID(2100),
			ElementRevision:   1,
			WeightBasisPoints: 1,
		},
		{
			ElementID:         task2UUID(2101),
			ElementRevision:   1,
			WeightBasisPoints: problemcontract.MaximumRewardWeightBasisPoints - 1,
		},
	}
	checksum, err := minimum.ComputeChecksum()
	if err != nil {
		t.Fatalf("minimum scalar boundaries rejected: %v", err)
	}
	minimum.Checksum = checksum
	if err := minimum.Validate(); err != nil {
		t.Fatalf("minimum scalar reward Validate() error = %v", err)
	}

	maximum := cloneTask2Reward(minimum)
	maximum.DifficultyRevision = problemcontract.MaximumDifficultyRevision
	maximum.BaseExperience = problemcontract.MaximumBaseExperience
	maximum.Tags.Revision = problemcontract.MaximumRewardMappingRevision
	for index := range maximum.Tags.Items {
		maximum.Tags.Items[index].TagRevision = problemcontract.MaximumRewardMappingRevision
	}
	maximum.Elements.Revision = problemcontract.MaximumRewardMappingRevision
	for index := range maximum.Elements.Items {
		maximum.Elements.Items[index].ElementRevision = problemcontract.MaximumRewardMappingRevision
	}
	checksum, err = maximum.ComputeChecksum()
	if err != nil {
		t.Fatalf("maximum scalar boundaries rejected: %v", err)
	}
	maximum.Checksum = checksum
	if err := maximum.Validate(); err != nil {
		t.Fatalf("maximum scalar reward Validate() error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*RewardSnapshot)
	}{
		{"difficulty revision below minimum", func(value *RewardSnapshot) {
			value.DifficultyRevision = 0
		}},
		{"difficulty revision above maximum", func(value *RewardSnapshot) {
			value.DifficultyRevision = problemcontract.MaximumDifficultyRevision + 1
		}},
		{"base experience below minimum", func(value *RewardSnapshot) {
			value.BaseExperience = -1
		}},
		{"base experience above maximum", func(value *RewardSnapshot) {
			value.BaseExperience = problemcontract.MaximumBaseExperience + 1
		}},
		{"tag mapping revision below minimum", func(value *RewardSnapshot) {
			value.Tags.Revision = 0
		}},
		{"tag mapping revision above maximum", func(value *RewardSnapshot) {
			value.Tags.Revision = problemcontract.MaximumRewardMappingRevision + 1
		}},
		{"element mapping revision below minimum", func(value *RewardSnapshot) {
			value.Elements.Revision = 0
		}},
		{"element mapping revision above maximum", func(value *RewardSnapshot) {
			value.Elements.Revision = problemcontract.MaximumRewardMappingRevision + 1
		}},
		{"element item revision below minimum", func(value *RewardSnapshot) {
			value.Elements.Items[0].ElementRevision = 0
		}},
		{"element item revision above maximum", func(value *RewardSnapshot) {
			value.Elements.Items[0].ElementRevision = problemcontract.MaximumRewardMappingRevision + 1
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := cloneTask2Reward(minimum)
			test.mutate(&value)
			if _, err := value.ComputeChecksum(); err == nil {
				t.Fatal("ComputeChecksum() unexpectedly succeeded")
			}
		})
	}
}

func TestRewardChecksumDetectsTamperingAndDistinguishesNilElements(t *testing.T) {
	t.Parallel()

	valid := validTask2Reward(t)
	tampered := cloneTask2Reward(valid)
	tampered.BaseExperience++
	if err := tampered.Validate(); !errors.Is(err, ErrRewardChecksum) {
		t.Fatalf("tampered reward Validate() error = %v, want ErrRewardChecksum", err)
	}

	withoutElements := cloneTask2Reward(valid)
	withoutElements.Elements = nil
	withoutElementsChecksum, err := withoutElements.ComputeChecksum()
	if err != nil {
		t.Fatalf("nil-element reward ComputeChecksum() error = %v", err)
	}
	if withoutElementsChecksum == valid.Checksum {
		t.Fatal("nil and populated element snapshots produced the same checksum")
	}
	withoutElements.Checksum = withoutElementsChecksum
	if err := withoutElements.Validate(); err != nil {
		t.Fatalf("nil-element reward Validate() error = %v", err)
	}
}

func TestRevisionContentChecksumIsDeterministicAndExcludesRowMetadata(t *testing.T) {
	t.Parallel()

	revision := validTask2Revision(t)
	first, err := revision.ComputeContentChecksum()
	if err != nil {
		t.Fatalf("ComputeContentChecksum() error = %v", err)
	}
	second, err := revision.ComputeContentChecksum()
	if err != nil || first != second {
		t.Fatalf("content checksum is not deterministic: first=%x second=%x err=%v", first, second, err)
	}

	metadataOnly := cloneTask2Revision(revision)
	metadataOnly.ID = task2UUID(9001)
	metadataOnly.ProblemID = task2UUID(9002)
	metadataOnly.RevisionNumber = problemcontract.MaximumProblemRevisionNumber
	metadataOnly.RenderSourceID = task2UUID(9003)
	metadataOnly.CreatedBy = task2UUID(9004)
	metadataOnly.CreatedAt = metadataOnly.CreatedAt.Add(24 * time.Hour)
	checksum, err := metadataOnly.ComputeContentChecksum()
	if err != nil {
		t.Fatalf("metadata-only ComputeContentChecksum() error = %v", err)
	}
	if checksum != revision.ContentChecksum {
		t.Fatalf("row metadata changed content checksum: got %x want %x", checksum, revision.ContentChecksum)
	}
	metadataOnly.ContentChecksum = revision.ContentChecksum
	if err := metadataOnly.Validate(); err != nil {
		t.Fatalf("metadata-only revision Validate() error = %v", err)
	}
}

func TestRevisionAcceptsExactAuthoredAndProtocolBoundaries(t *testing.T) {
	t.Parallel()

	revision := validTask2Revision(t)
	revision.RevisionNumber = problemcontract.MaximumProblemRevisionNumber
	revision.Title = strings.Repeat("t", problemcontract.MaximumTitleBytes)
	revision.StatementMarkdown = bytes.Repeat([]byte{'s'}, problemcontract.MaximumStatementMarkdownBytes)
	normalized, err := contentrender.NormalizeMarkdown(revision.StatementMarkdown)
	if err != nil {
		t.Fatalf("maximum statement NormalizeMarkdown() error = %v", err)
	}
	revision.RenderSourceChecksum = normalized.Checksum
	revision.CheckerKey = "c" + strings.Repeat("1", problemcontract.MaximumCheckerKeyBytes-1)
	revision.CheckerVersion = "v" + strings.Repeat("1", problemcontract.MaximumCheckerVersionBytes-1)
	revision.AllowedRuntimeKeys = []string{
		"r" + strings.Repeat("1", problemcontract.MaximumRuntimeKeyBytes-1),
	}
	checksum, err := revision.ComputeContentChecksum()
	if err != nil {
		t.Fatalf("maximum revision boundaries ComputeContentChecksum() error = %v", err)
	}
	revision.ContentChecksum = checksum
	if err := revision.Validate(); err != nil {
		t.Fatalf("maximum revision boundaries Validate() error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Revision)
	}{
		{"title above maximum", func(value *Revision) {
			value.Title = strings.Repeat("t", problemcontract.MaximumTitleBytes+1)
		}},
		{"statement above maximum", func(value *Revision) {
			value.StatementMarkdown = bytes.Repeat(
				[]byte{'s'},
				problemcontract.MaximumStatementMarkdownBytes+1,
			)
		}},
		{"checker key above maximum", func(value *Revision) {
			value.CheckerKey = "c" + strings.Repeat("1", problemcontract.MaximumCheckerKeyBytes)
		}},
		{"checker version above maximum", func(value *Revision) {
			value.CheckerVersion = "v" + strings.Repeat("1", problemcontract.MaximumCheckerVersionBytes)
		}},
		{"runtime key above maximum", func(value *Revision) {
			value.AllowedRuntimeKeys = []string{
				"r" + strings.Repeat("1", problemcontract.MaximumRuntimeKeyBytes),
			}
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := validTask2Revision(t)
			test.mutate(&value)
			if _, err := value.ComputeContentChecksum(); err == nil {
				t.Fatal("ComputeContentChecksum() unexpectedly succeeded")
			}
		})
	}
}

func TestRevisionValidateDetectsSemanticTampering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*testing.T, *Revision)
	}{
		{"title", func(_ *testing.T, value *Revision) { value.Title += "!" }},
		{"statement and matching render checksum", func(t *testing.T, value *Revision) {
			value.StatementMarkdown = []byte("# Changed\n")
			normalized, err := contentrender.NormalizeMarkdown(value.StatementMarkdown)
			if err != nil {
				t.Fatalf("NormalizeMarkdown() error = %v", err)
			}
			value.RenderSourceChecksum = normalized.Checksum
		}},
		{"reward and matching reward checksum", func(t *testing.T, value *Revision) {
			value.Reward.BaseExperience++
			checksum, err := value.Reward.ComputeChecksum()
			if err != nil {
				t.Fatalf("reward ComputeChecksum() error = %v", err)
			}
			value.Reward.Checksum = checksum
		}},
		{"limits", func(_ *testing.T, value *Revision) { value.Limits.OutputBytes++ }},
		{"checker key", func(_ *testing.T, value *Revision) { value.CheckerKey = "token_checker" }},
		{"checker version", func(_ *testing.T, value *Revision) { value.CheckerVersion = "v2" }},
		{"checker config", func(t *testing.T, value *Revision) {
			config, err := CanonicalizeCheckerConfig(json.RawMessage(`{"mode":"tokens"}`))
			if err != nil {
				t.Fatalf("CanonicalizeCheckerConfig() error = %v", err)
			}
			value.CheckerConfig = config
		}},
		{"testset", func(_ *testing.T, value *Revision) { value.TestsetRevisionID = task2UUID(9911) }},
		{"runtime", func(_ *testing.T, value *Revision) {
			value.AllowedRuntimeKeys = []string{"go1.25", "rust1.90"}
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			revision := validTask2Revision(t)
			test.mutate(t, &revision)
			if err := revision.Validate(); !errors.Is(err, ErrRevisionChecksum) {
				t.Fatalf("tampered revision Validate() error = %v, want ErrRevisionChecksum", err)
			}
		})
	}
}

func TestRevisionValidateRejectsNonCanonicalOrInvalidPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Revision)
		target error
	}{
		{"nil identity", func(value *Revision) { value.ID = uuid.Nil }, ErrInvalidProblemRevision},
		{"zero revision", func(value *Revision) { value.RevisionNumber = 0 }, ErrInvalidProblemRevision},
		{"revision above maximum", func(value *Revision) {
			value.RevisionNumber = problemcontract.MaximumProblemRevisionNumber + 1
		}, ErrInvalidProblemRevision},
		{"blank title", func(value *Revision) { value.Title = " \t" }, ErrInvalidProblemRevision},
		{"noncanonical markdown", func(value *Revision) {
			value.StatementMarkdown = []byte("line\r\n")
		}, ErrInvalidProblemRevision},
		{"wrong render kind", func(value *Revision) {
			value.RenderSourceKind = "other"
		}, ErrInvalidProblemRevision},
		{"wrong render checksum", func(value *Revision) {
			value.RenderSourceChecksum[0] ^= 0xff
		}, ErrInvalidProblemRevision},
		{"invalid checker identifier", func(value *Revision) {
			value.CheckerKey = "Token Checker"
		}, ErrInvalidProblemRevision},
		{"noncanonical checker JSON", func(value *Revision) {
			value.CheckerConfig = json.RawMessage(`{"z":1, "a":2}`)
		}, ErrNonCanonicalJSON},
		{"duplicate runtime", func(value *Revision) {
			value.AllowedRuntimeKeys = []string{"go1.25", "go1.25"}
		}, ErrInvalidProblemRevision},
		{"nil testset", func(value *Revision) {
			value.TestsetRevisionID = uuid.Nil
		}, ErrInvalidProblemRevision},
		{"zero creation time", func(value *Revision) {
			value.CreatedAt = time.Time{}
		}, ErrInvalidProblemRevision},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			revision := validTask2Revision(t)
			test.mutate(&revision)
			if err := revision.Validate(); !errors.Is(err, test.target) {
				t.Fatalf("Validate() error = %v, want errors.Is(%v)", err, test.target)
			}
		})
	}
}

func nestedJSONObject(depth int) json.RawMessage {
	value := []byte(`{}`)
	for range depth {
		value = append(append([]byte(`{"x":`), value...), '}')
	}
	return json.RawMessage(value)
}

func validTask2Revision(t *testing.T) Revision {
	t.Helper()

	statement := []byte("# Sum\n\nCompute the sum.\n")
	normalized, err := contentrender.NormalizeMarkdown(statement)
	if err != nil {
		t.Fatalf("NormalizeMarkdown() error = %v", err)
	}
	config, err := CanonicalizeCheckerConfig(json.RawMessage(`{"mode":"exact","trim":true}`))
	if err != nil {
		t.Fatalf("CanonicalizeCheckerConfig() error = %v", err)
	}
	reward := validTask2Reward(t)
	revision := Revision{
		ID:                   task2UUID(1),
		ProblemID:            task2UUID(2),
		RevisionNumber:       1,
		Title:                "Sum",
		StatementMarkdown:    append([]byte(nil), normalized.Bytes...),
		RenderSourceID:       task2UUID(3),
		RenderSourceKind:     problemcontract.RenderSourceKindProblemStatement,
		RenderSourceChecksum: normalized.Checksum,
		Reward:               reward,
		Limits: Limits{
			CPUTimeMilliseconds:  1_000,
			WallTimeMilliseconds: 2_000,
			MemoryKilobytes:      256 << 10,
			OutputBytes:          64 << 10,
			ProcessCount:         1,
		},
		CheckerKey:         "exact",
		CheckerVersion:     "v1",
		CheckerConfig:      config,
		TestsetRevisionID:  task2UUID(4),
		AllowedRuntimeKeys: []string{"go1.25", "python3.13"},
		CreatedBy:          task2UUID(5),
		CreatedAt:          time.Unix(1_700_000_000, 0).UTC(),
	}
	checksum, err := revision.ComputeContentChecksum()
	if err != nil {
		t.Fatalf("ComputeContentChecksum() error = %v", err)
	}
	revision.ContentChecksum = checksum
	return revision
}

func validTask2Reward(t *testing.T) RewardSnapshot {
	t.Helper()

	reward := RewardSnapshot{
		DifficultyID:       task2UUID(100),
		DifficultyRevision: 1,
		BaseExperience:     100,
		Tags: TagMappingSnapshot{
			MappingID: task2UUID(101),
			Revision:  1,
			Items: []TagSnapshotItem{
				{TagID: task2UUID(102), TagRevision: 1},
				{TagID: task2UUID(103), TagRevision: 2},
			},
		},
		Elements: &ElementMappingSnapshot{
			MappingID: task2UUID(104),
			Revision:  1,
			Items: []ElementSnapshotItem{
				{ElementID: task2UUID(105), ElementRevision: 1, WeightBasisPoints: 4_000},
				{ElementID: task2UUID(106), ElementRevision: 1, WeightBasisPoints: 6_000},
			},
		},
	}
	checksum, err := reward.ComputeChecksum()
	if err != nil {
		t.Fatalf("reward ComputeChecksum() error = %v", err)
	}
	reward.Checksum = checksum
	return reward
}

func cloneTask2Reward(value RewardSnapshot) RewardSnapshot {
	cloned := value
	cloned.Tags.Items = append([]TagSnapshotItem(nil), value.Tags.Items...)
	if value.Elements != nil {
		elements := *value.Elements
		elements.Items = append([]ElementSnapshotItem(nil), value.Elements.Items...)
		cloned.Elements = &elements
	}
	return cloned
}

func cloneTask2Revision(value Revision) Revision {
	cloned := value
	cloned.StatementMarkdown = append([]byte(nil), value.StatementMarkdown...)
	cloned.CheckerConfig = append(json.RawMessage(nil), value.CheckerConfig...)
	cloned.AllowedRuntimeKeys = append([]string(nil), value.AllowedRuntimeKeys...)
	cloned.Reward = cloneTask2Reward(value.Reward)
	return cloned
}

func task2UUID(value uint64) uuid.UUID {
	var id uuid.UUID
	binary.BigEndian.PutUint64(id[8:], value)
	return id
}

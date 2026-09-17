package domain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

func TestIdentityTestAssetSupportsEmptyContentAndOwnsDecodedBytes(t *testing.T) {
	t.Parallel()

	empty := task2IdentityAsset(10, nil)
	if err := empty.ValidateMetadata(); err != nil {
		t.Fatalf("empty identity asset ValidateMetadata() error = %v", err)
	}
	if err := (IdentityAssetVerifier{}).Verify(context.Background(), empty); err != nil {
		t.Fatalf("empty identity asset Verify() error = %v", err)
	}
	decoded, err := (IdentityAssetDecoder{}).Decode(context.Background(), empty)
	if err != nil {
		t.Fatalf("empty identity asset Decode() error = %v", err)
	}
	if len(decoded) != 0 {
		t.Fatalf("decoded empty asset length = %d", len(decoded))
	}

	asset := task2IdentityAsset(11, []byte("owned"))
	decoded, err = (IdentityAssetDecoder{}).Decode(context.Background(), asset)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	decoded[0] = 'X'
	if string(asset.Content) != "owned" {
		t.Fatalf("decoded bytes alias stored content: content=%q", asset.Content)
	}

	maximum := task2IdentityAsset(12, make([]byte, problemcontract.MaximumTestAssetBytes))
	if err := maximum.ValidateMetadata(); err != nil {
		t.Fatalf("maximum-sized identity asset ValidateMetadata() error = %v", err)
	}
}

func TestTestAssetJSONSerializationNeverExposesInternalIdentityOrContent(t *testing.T) {
	t.Parallel()

	asset := task2IdentityAsset(13, []byte("hidden input: 99 1\n"))
	encoded, err := json.Marshal(asset)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if string(encoded) != "{}" {
		t.Fatalf("json.Marshal() = %s, want empty object", encoded)
	}
	for _, forbidden := range []string{
		"hidden",
		"aGlkZGVu",
		asset.ID.String(),
		hex.EncodeToString(asset.SHA256[:]),
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("serialized asset contains %q: %s", forbidden, encoded)
		}
	}
}

func TestIdentityTestAssetRejectsEveryIntegrityAndMetadataViolation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*TestAsset)
		target error
	}{
		{"nil id", func(asset *TestAsset) { asset.ID = uuid.Nil }, ErrInvalidTestAsset},
		{"zero checksum", func(asset *TestAsset) { asset.SHA256 = [32]byte{} }, ErrInvalidTestAsset},
		{"unknown encoding", func(asset *TestAsset) { asset.Encoding = "gzip" }, ErrInvalidTestAsset},
		{"negative uncompressed size", func(asset *TestAsset) { asset.UncompressedSize = -1 }, ErrInvalidTestAsset},
		{"uncompressed size above maximum", func(asset *TestAsset) {
			asset.UncompressedSize = problemcontract.MaximumTestAssetBytes + 1
		}, ErrInvalidTestAsset},
		{"encoded bytes above maximum", func(asset *TestAsset) {
			asset.Content = make([]byte, problemcontract.MaximumTestAssetBytes+1)
		}, ErrInvalidTestAsset},
		{"zero creation time", func(asset *TestAsset) { asset.CreatedAt = time.Time{} }, ErrInvalidTestAsset},
		{"declared size mismatch", func(asset *TestAsset) { asset.UncompressedSize++ }, ErrTestAssetIntegrity},
		{"checksum mismatch", func(asset *TestAsset) { asset.Content[0] ^= 0xff }, ErrTestAssetIntegrity},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			asset := task2IdentityAsset(12, []byte("payload"))
			test.mutate(&asset)
			if err := asset.ValidateMetadata(); !errors.Is(err, test.target) {
				t.Fatalf("ValidateMetadata() error = %v, want errors.Is(%v)", err, test.target)
			}
		})
	}
}

func TestZstdTestAssetMetadataFailsClosedWithoutACompressionVerifier(t *testing.T) {
	t.Parallel()

	valid := task2ZstdAsset(20, []byte{0x28, 0xb5, 0x2f, 0xfd}, 0)
	if err := valid.ValidateMetadata(); err != nil {
		t.Fatalf("bounded non-empty zstd metadata rejected: %v", err)
	}
	if err := (IdentityAssetVerifier{}).Verify(context.Background(), valid); !errors.Is(err, ErrInvalidTestAsset) {
		t.Fatalf("IdentityAssetVerifier.Verify(zstd) error = %v, want ErrInvalidTestAsset", err)
	}
	if _, err := (IdentityAssetDecoder{}).Decode(context.Background(), valid); !errors.Is(err, ErrInvalidTestAsset) {
		t.Fatalf("IdentityAssetDecoder.Decode(zstd) error = %v, want ErrInvalidTestAsset", err)
	}

	tests := []struct {
		name   string
		mutate func(*TestAsset)
	}{
		{"empty encoded frame", func(asset *TestAsset) { asset.Content = nil }},
		{"nil id", func(asset *TestAsset) { asset.ID = uuid.Nil }},
		{"zero checksum", func(asset *TestAsset) { asset.SHA256 = [32]byte{} }},
		{"negative uncompressed size", func(asset *TestAsset) { asset.UncompressedSize = -1 }},
		{"uncompressed size above maximum", func(asset *TestAsset) {
			asset.UncompressedSize = problemcontract.MaximumTestAssetBytes + 1
		}},
		{"encoded size above maximum", func(asset *TestAsset) {
			asset.Content = make([]byte, problemcontract.MaximumTestAssetBytes+1)
		}},
		{"zero creation time", func(asset *TestAsset) { asset.CreatedAt = time.Time{} }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			asset := valid
			asset.Content = append([]byte(nil), valid.Content...)
			test.mutate(&asset)
			if err := asset.ValidateMetadata(); !errors.Is(err, ErrInvalidTestAsset) {
				t.Fatalf("ValidateMetadata() error = %v, want ErrInvalidTestAsset", err)
			}
		})
	}
}

func TestIdentityAssetVerifierHonorsContextAndInputContract(t *testing.T) {
	t.Parallel()

	asset := task2IdentityAsset(30, []byte("payload"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (IdentityAssetVerifier{}).Verify(ctx, asset); !errors.Is(err, context.Canceled) {
		t.Fatalf("Verify(canceled context) error = %v, want context.Canceled", err)
	}
	// Held in a variable rather than written inline so the nil-context guard
	// stays covered without tripping the linter's literal-nil check.
	var missingContext context.Context
	if err := (IdentityAssetVerifier{}).Verify(missingContext, asset); !errors.Is(err, ErrInvalidTestAsset) {
		t.Fatalf("Verify(nil context) error = %v, want ErrInvalidTestAsset", err)
	}
}

func TestTestsetValidateStructureAcceptsExactCountSizeAndGroupBoundaries(t *testing.T) {
	t.Parallel()

	minimum := task2TestsetWithGroupCount(t, 1)
	if minimum.TestCount != 1 || minimum.TotalUncompressedBytes != 0 {
		t.Fatalf(
			"minimum testset count/bytes = %d/%d, want 1/0",
			minimum.TestCount,
			minimum.TotalUncompressedBytes,
		)
	}
	if err := minimum.ValidateStructure(); err != nil {
		t.Fatalf("minimum count/size ValidateStructure() error = %v", err)
	}

	maximum := task2MaximumTestset(t)
	if maximum.TestCount != problemcontract.MaximumTestCases {
		t.Fatalf("test count = %d, want %d", maximum.TestCount, problemcontract.MaximumTestCases)
	}
	if maximum.TotalUncompressedBytes != problemcontract.MaximumTestsetUncompressedBytes {
		t.Fatalf(
			"total bytes = %d, want %d",
			maximum.TotalUncompressedBytes,
			problemcontract.MaximumTestsetUncompressedBytes,
		)
	}
	if err := maximum.ValidateStructure(); err != nil {
		t.Fatalf("maximum count/size ValidateStructure() error = %v", err)
	}

	maximumGroups := task2TestsetWithGroupCount(t, problemcontract.MaximumTestcaseGroups)
	if err := maximumGroups.ValidateStructure(); err != nil {
		t.Fatalf("maximum group count ValidateStructure() error = %v", err)
	}
}

func TestTestsetValidateStructureRejectsInvalidIdentityOrderingCountsAndVisibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*TestsetRevision)
		target error
	}{
		{"nil id", func(value *TestsetRevision) { value.ID = uuid.Nil }, ErrInvalidTestsetRevision},
		{"nil problem", func(value *TestsetRevision) { value.ProblemID = uuid.Nil }, ErrInvalidTestsetRevision},
		{"zero revision", func(value *TestsetRevision) { value.RevisionNumber = 0 }, ErrInvalidTestsetRevision},
		{"revision above maximum", func(value *TestsetRevision) {
			value.RevisionNumber = problemcontract.MaximumTestsetRevisionNumber + 1
		}, ErrInvalidTestsetRevision},
		{"zero declared test count", func(value *TestsetRevision) { value.TestCount = 0 }, ErrInvalidTestsetRevision},
		{"declared test count above maximum", func(value *TestsetRevision) {
			value.TestCount = problemcontract.MaximumTestCases + 1
		}, ErrInvalidTestsetRevision},
		{"negative declared bytes", func(value *TestsetRevision) {
			value.TotalUncompressedBytes = -1
		}, ErrInvalidTestsetRevision},
		{"declared bytes above maximum", func(value *TestsetRevision) {
			value.TotalUncompressedBytes = problemcontract.MaximumTestsetUncompressedBytes + 1
		}, ErrInvalidTestsetRevision},
		{"nil creator", func(value *TestsetRevision) { value.CreatedBy = uuid.Nil }, ErrInvalidTestsetRevision},
		{"zero creation time", func(value *TestsetRevision) {
			value.CreatedAt = time.Time{}
		}, ErrInvalidTestsetRevision},
		{"no groups", func(value *TestsetRevision) { value.Groups = nil }, ErrInvalidTestsetRevision},
		{"too many groups", func(value *TestsetRevision) {
			value.Groups = make([]TestcaseGroup, problemcontract.MaximumTestcaseGroups+1)
		}, ErrInvalidTestsetRevision},
		{"nil group id", func(value *TestsetRevision) {
			value.Groups[0].ID = uuid.Nil
		}, ErrNonContiguousTests},
		{"wrong group testset", func(value *TestsetRevision) {
			value.Groups[0].TestsetRevisionID = task2UUID(9001)
		}, ErrNonContiguousTests},
		{"noncontiguous group index", func(value *TestsetRevision) {
			value.Groups[1].GroupIndex = 3
		}, ErrNonContiguousTests},
		{"zero group creation time", func(value *TestsetRevision) {
			value.Groups[0].CreatedAt = time.Time{}
		}, ErrNonContiguousTests},
		{"empty group", func(value *TestsetRevision) {
			value.Groups[0].Cases = nil
		}, ErrNonContiguousTests},
		{"nil testcase revision id", func(value *TestsetRevision) {
			value.Groups[0].Cases[0].ID = uuid.Nil
		}, ErrNonContiguousTests},
		{"nil testcase lineage id", func(value *TestsetRevision) {
			value.Groups[0].Cases[0].TestCaseID = uuid.Nil
		}, ErrNonContiguousTests},
		{"wrong testcase testset", func(value *TestsetRevision) {
			value.Groups[0].Cases[0].TestsetRevisionID = task2UUID(9002)
		}, ErrNonContiguousTests},
		{"wrong testcase problem", func(value *TestsetRevision) {
			value.Groups[0].Cases[0].ProblemID = task2UUID(9003)
		}, ErrNonContiguousTests},
		{"wrong testcase group", func(value *TestsetRevision) {
			value.Groups[0].Cases[0].GroupIndex = 1
		}, ErrNonContiguousTests},
		{"noncontiguous testcase order", func(value *TestsetRevision) {
			value.Groups[0].Cases[0].OrderIndex = 1
		}, ErrNonContiguousTests},
		{"group stop policy mismatch", func(value *TestsetRevision) {
			value.Groups[0].Cases[0].StopOnFailure = !value.Groups[0].StopOnFailure
		}, ErrNonContiguousTests},
		{"unknown test kind", func(value *TestsetRevision) {
			value.Groups[0].Cases[0].Kind = "private"
		}, ErrNonContiguousTests},
		{"weight below exact one", func(value *TestsetRevision) {
			value.Groups[0].Cases[0].Weight = 0
		}, ErrNonContiguousTests},
		{"weight above exact one", func(value *TestsetRevision) {
			value.Groups[0].Cases[0].Weight = 2
		}, ErrNonContiguousTests},
		{"zero testcase creation time", func(value *TestsetRevision) {
			value.Groups[0].Cases[0].CreatedAt = time.Time{}
		}, ErrNonContiguousTests},
		{"duplicate testcase revision id", func(value *TestsetRevision) {
			value.Groups[1].Cases[0].ID = value.Groups[0].Cases[0].ID
		}, ErrInvalidTestsetRevision},
		{"duplicate testcase lineage id", func(value *TestsetRevision) {
			value.Groups[1].Cases[0].TestCaseID = value.Groups[0].Cases[0].TestCaseID
		}, ErrInvalidTestsetRevision},
		{"corrupt input asset", func(value *TestsetRevision) {
			value.Groups[0].Cases[0].InputAsset.Content[0] ^= 0xff
		}, ErrTestAssetIntegrity},
		{"no hidden tests", func(value *TestsetRevision) {
			value.Groups[1].Cases[0].Kind = problemcontract.TestKindSample
		}, ErrHiddenTestRequired},
		{"declared count mismatch", func(value *TestsetRevision) {
			value.TestCount--
		}, ErrInvalidTestsetRevision},
		{"declared byte count mismatch", func(value *TestsetRevision) {
			value.TotalUncompressedBytes++
		}, ErrInvalidTestsetRevision},
		{"zero checksum", func(value *TestsetRevision) {
			value.Checksum = [32]byte{}
		}, ErrTestsetChecksum},
		{"checksum mismatch", func(value *TestsetRevision) {
			value.Checksum[0] ^= 0xff
		}, ErrTestsetChecksum},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			testset := validTask2Testset(t)
			test.mutate(&testset)
			if err := testset.ValidateStructure(); !errors.Is(err, test.target) {
				t.Fatalf("ValidateStructure() error = %v, want errors.Is(%v)", err, test.target)
			}
		})
	}
}

func TestTestsetChecksumIsDeterministicAndExcludesGeneratedRowMetadata(t *testing.T) {
	t.Parallel()

	testset := validTask2Testset(t)
	first, err := testset.ComputeChecksum()
	if err != nil {
		t.Fatalf("ComputeChecksum() error = %v", err)
	}
	second, err := testset.ComputeChecksum()
	if err != nil || first != second {
		t.Fatalf("testset checksum is not deterministic: first=%x second=%x err=%v", first, second, err)
	}

	metadataOnly := cloneTask2Testset(testset)
	metadataOnly.ID = task2UUID(9100)
	metadataOnly.RevisionNumber = problemcontract.MaximumTestsetRevisionNumber
	metadataOnly.CreatedBy = task2UUID(9101)
	metadataOnly.CreatedAt = metadataOnly.CreatedAt.Add(48 * time.Hour)
	for groupIndex := range metadataOnly.Groups {
		group := &metadataOnly.Groups[groupIndex]
		group.ID = task2UUID(uint64(9200 + groupIndex))
		group.TestsetRevisionID = metadataOnly.ID
		group.CreatedAt = group.CreatedAt.Add(time.Hour)
		for caseIndex := range group.Cases {
			testcase := &group.Cases[caseIndex]
			testcase.ID = task2UUID(uint64(9300 + groupIndex*10 + caseIndex))
			testcase.TestsetRevisionID = metadataOnly.ID
			testcase.CreatedAt = testcase.CreatedAt.Add(time.Hour)
			testcase.InputAsset.ID = task2UUID(uint64(9400 + groupIndex*10 + caseIndex*2))
			testcase.ExpectedAsset.ID = task2UUID(uint64(9401 + groupIndex*10 + caseIndex*2))
			testcase.InputAsset.CreatedAt = testcase.InputAsset.CreatedAt.Add(time.Hour)
			testcase.ExpectedAsset.CreatedAt = testcase.ExpectedAsset.CreatedAt.Add(time.Hour)
		}
	}
	checksum, err := metadataOnly.ComputeChecksum()
	if err != nil {
		t.Fatalf("metadata-only ComputeChecksum() error = %v", err)
	}
	if checksum != testset.Checksum {
		t.Fatalf("generated row metadata changed checksum: got %x want %x", checksum, testset.Checksum)
	}
	metadataOnly.Checksum = testset.Checksum
	if err := metadataOnly.ValidateStructure(); err != nil {
		t.Fatalf("metadata-only ValidateStructure() error = %v", err)
	}
}

func TestTestsetChecksumBindsEverySemanticFact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*TestsetRevision)
	}{
		{"problem", func(value *TestsetRevision) {
			value.ProblemID = task2UUID(9500)
			for groupIndex := range value.Groups {
				for caseIndex := range value.Groups[groupIndex].Cases {
					value.Groups[groupIndex].Cases[caseIndex].ProblemID = value.ProblemID
				}
			}
		}},
		{"testcase lineage", func(value *TestsetRevision) {
			value.Groups[0].Cases[0].TestCaseID = task2UUID(9501)
		}},
		{"kind", func(value *TestsetRevision) {
			value.Groups[0].Cases[0].Kind = problemcontract.TestKindHidden
		}},
		{"group policy", func(value *TestsetRevision) {
			value.Groups[0].StopOnFailure = true
			value.Groups[0].Cases[0].StopOnFailure = true
		}},
		{"input content digest", func(value *TestsetRevision) {
			asset := &value.Groups[0].Cases[0].InputAsset
			asset.Content = []byte("9 9\n")
			asset.UncompressedSize = int64(len(asset.Content))
			asset.SHA256 = sha256.Sum256(asset.Content)
		}},
		{"expected content digest", func(value *TestsetRevision) {
			asset := &value.Groups[0].Cases[0].ExpectedAsset
			asset.Content = []byte("4\n")
			asset.UncompressedSize = int64(len(asset.Content))
			asset.SHA256 = sha256.Sum256(asset.Content)
		}},
		{"group and case order", func(value *TestsetRevision) {
			value.Groups[0], value.Groups[1] = value.Groups[1], value.Groups[0]
			for groupIndex := range value.Groups {
				value.Groups[groupIndex].GroupIndex = groupIndex
				for caseIndex := range value.Groups[groupIndex].Cases {
					value.Groups[groupIndex].Cases[caseIndex].GroupIndex = groupIndex
					value.Groups[groupIndex].Cases[caseIndex].OrderIndex = caseIndex
				}
			}
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			testset := validTask2Testset(t)
			test.mutate(&testset)
			checksum, err := testset.ComputeChecksum()
			if err != nil {
				t.Fatalf("tampered ComputeChecksum() error = %v", err)
			}
			if checksum == testset.Checksum {
				t.Fatal("semantic mutation did not change testset checksum")
			}
			if err := testset.ValidateStructure(); !errors.Is(err, ErrTestsetChecksum) {
				t.Fatalf("tampered ValidateStructure() error = %v, want ErrTestsetChecksum", err)
			}
		})
	}
}

func TestTestsetChecksumUsesUncompressedContentIdentityNotStorageEncoding(t *testing.T) {
	t.Parallel()

	testset := validTask2Testset(t)
	storageChanged := cloneTask2Testset(testset)
	for groupIndex := range storageChanged.Groups {
		for caseIndex := range storageChanged.Groups[groupIndex].Cases {
			testcase := &storageChanged.Groups[groupIndex].Cases[caseIndex]
			for _, asset := range []*TestAsset{&testcase.InputAsset, &testcase.ExpectedAsset} {
				asset.ID = task2UUID(uint64(9700 + groupIndex*100 + caseIndex*10))
				asset.Encoding = problemcontract.AssetEncodingZstd
				asset.Content = []byte{0x28, 0xb5, 0x2f, 0xfd}
			}
		}
	}
	checksum, err := storageChanged.ComputeChecksum()
	if err != nil {
		t.Fatalf("storage-changed ComputeChecksum() error = %v", err)
	}
	if checksum != testset.Checksum {
		t.Fatalf("storage encoding changed content identity: got %x want %x", checksum, testset.Checksum)
	}
}

func TestTestsetVerifyAssetsDeduplicatesAndDetectsIdentityCorruption(t *testing.T) {
	t.Parallel()

	testset := validTask2Testset(t)
	testset.Groups[1].Cases[0].InputAsset = cloneTask2Asset(testset.Groups[0].Cases[0].InputAsset)
	testset.Groups[1].Cases[0].ExpectedAsset = cloneTask2Asset(testset.Groups[0].Cases[0].ExpectedAsset)

	verifier := &task2RecordingVerifier{calls: make(map[uuid.UUID]int)}
	if err := testset.VerifyAssets(context.Background(), verifier); err != nil {
		t.Fatalf("VerifyAssets() error = %v", err)
	}
	if verifier.total != 2 {
		t.Fatalf("verifier calls = %d, want exactly 2 unique assets", verifier.total)
	}
	for id, count := range verifier.calls {
		if count != 1 {
			t.Fatalf("asset %s verified %d times, want once", id, count)
		}
	}

	corrupt := cloneTask2Testset(testset)
	corrupt.Groups[1].Cases[0].InputAsset.SHA256[0] ^= 0xff
	verifier = &task2RecordingVerifier{calls: make(map[uuid.UUID]int)}
	if err := corrupt.VerifyAssets(context.Background(), verifier); !errors.Is(err, ErrTestAssetIntegrity) {
		t.Fatalf("VerifyAssets(corrupt identity) error = %v, want ErrTestAssetIntegrity", err)
	}
}

func TestTestsetVerifyAssetsFailsClosedOnInvalidInputsAndVerifierErrors(t *testing.T) {
	t.Parallel()

	testset := validTask2Testset(t)
	// Held in a variable rather than written inline so the nil-context guard
	// stays covered without tripping the linter's literal-nil check.
	var missingContext context.Context
	if err := testset.VerifyAssets(missingContext, &task2RecordingVerifier{}); !errors.Is(err, ErrInvalidTestAsset) {
		t.Fatalf("VerifyAssets(nil context) error = %v, want ErrInvalidTestAsset", err)
	}
	if err := testset.VerifyAssets(context.Background(), nil); !errors.Is(err, ErrInvalidTestAsset) {
		t.Fatalf("VerifyAssets(nil verifier) error = %v, want ErrInvalidTestAsset", err)
	}

	marker := errors.New("decoder unavailable")
	verifier := &task2RecordingVerifier{failure: marker}
	if err := testset.VerifyAssets(context.Background(), verifier); !errors.Is(err, marker) {
		t.Fatalf("VerifyAssets(verifier error) = %v, want marker", err)
	}
}

func TestBuildPublicSamplesNeverDecodesOrSerializesHiddenAssets(t *testing.T) {
	t.Parallel()

	testset := validTask2Testset(t)
	hidden := testset.Groups[1].Cases[0]
	hidden.InputAsset = task2IdentityAsset(9801, []byte("hidden-input-secret"))
	hidden.ExpectedAsset = task2IdentityAsset(9802, []byte("hidden-output-secret"))
	testset.Groups[1].Cases[0] = hidden

	decoder := &task2PublicDecoder{
		hidden: map[uuid.UUID]struct{}{
			hidden.InputAsset.ID:    {},
			hidden.ExpectedAsset.ID: {},
		},
	}
	samples, err := BuildPublicSamples(context.Background(), testset, decoder)
	if err != nil {
		t.Fatalf("BuildPublicSamples() error = %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("sample count = %d, want 1", len(samples))
	}
	if len(decoder.decoded) != 2 {
		t.Fatalf("decoded asset count = %d, want only sample input/output", len(decoder.decoded))
	}
	if slices.Contains(decoder.decoded, hidden.InputAsset.ID) ||
		slices.Contains(decoder.decoded, hidden.ExpectedAsset.ID) {
		t.Fatalf("hidden asset reached decoder: %v", decoder.decoded)
	}

	payload, err := json.Marshal(samples)
	if err != nil {
		t.Fatalf("json.Marshal(samples) error = %v", err)
	}
	for _, secret := range []string{
		"hidden-input-secret",
		"hidden-output-secret",
		hidden.InputAsset.ID.String(),
		hidden.ExpectedAsset.ID.String(),
		hex.EncodeToString(hidden.InputAsset.SHA256[:]),
		hex.EncodeToString(hidden.ExpectedAsset.SHA256[:]),
	} {
		if strings.Contains(string(payload), secret) {
			t.Fatalf("public payload leaked hidden value %q: %s", secret, payload)
		}
	}

	var documents []map[string]any
	if err := json.Unmarshal(payload, &documents); err != nil {
		t.Fatalf("json.Unmarshal(samples) error = %v", err)
	}
	wantKeys := []string{"expected_output", "group_index", "input", "order_index"}
	for _, document := range documents {
		keys := make([]string, 0, len(document))
		for key := range document {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		if !slices.Equal(keys, wantKeys) {
			t.Fatalf("public sample keys = %v, want exact safe projection %v", keys, wantKeys)
		}
	}
}

func TestBuildPublicSamplesRejectsUnsafeDecodedContentAndDecoderFailures(t *testing.T) {
	t.Parallel()

	// Held in a variable rather than written inline so the nil-context guard
	// stays covered without tripping the linter's literal-nil check.
	var missingContext context.Context
	if _, err := BuildPublicSamples(missingContext, TestsetRevision{}, IdentityAssetDecoder{}); !errors.Is(err, ErrUnsafeSampleProjection) {
		t.Fatalf("BuildPublicSamples(nil context) error = %v, want ErrUnsafeSampleProjection", err)
	}
	if _, err := BuildPublicSamples(context.Background(), TestsetRevision{}, nil); !errors.Is(err, ErrUnsafeSampleProjection) {
		t.Fatalf("BuildPublicSamples(nil decoder) error = %v, want ErrUnsafeSampleProjection", err)
	}

	marker := errors.New("asset unavailable")
	testset := validTask2Testset(t)
	if _, err := BuildPublicSamples(context.Background(), testset, &task2PublicDecoder{failure: marker}); !errors.Is(err, marker) ||
		!errors.Is(err, ErrUnsafeSampleProjection) {
		t.Fatalf("BuildPublicSamples(decoder failure) error = %v, want joined safe error and marker", err)
	}

	tests := []struct {
		name    string
		content []byte
	}{
		{"invalid utf8", []byte{0xff}},
		{"nul byte", []byte{'a', 0, 'b'}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			testset := validTask2Testset(t)
			decoder := &task2PublicDecoder{override: test.content}
			_, err := BuildPublicSamples(context.Background(), testset, decoder)
			if !errors.Is(err, ErrUnsafeSampleProjection) {
				t.Fatalf("BuildPublicSamples() error = %v, want ErrUnsafeSampleProjection", err)
			}
		})
	}
}

func TestPublicSampleSupportsEmptyUTF8Content(t *testing.T) {
	t.Parallel()

	testset := validTask2Testset(t)
	testset.Groups[0].Cases[0].InputAsset = task2IdentityAsset(9901, nil)
	testset.Groups[0].Cases[0].ExpectedAsset = task2IdentityAsset(9902, nil)
	samples, err := BuildPublicSamples(context.Background(), testset, IdentityAssetDecoder{})
	if err != nil {
		t.Fatalf("BuildPublicSamples() error = %v", err)
	}
	if len(samples) != 1 || samples[0].Input != "" || samples[0].ExpectedOutput != "" {
		t.Fatalf("empty public sample = %#v", samples)
	}
}

func validTask2Testset(t *testing.T) TestsetRevision {
	t.Helper()

	createdAt := time.Unix(1_700_000_000, 0).UTC()
	testset := TestsetRevision{
		ID:             task2UUID(10000),
		ProblemID:      task2UUID(10001),
		RevisionNumber: 1,
		CreatedBy:      task2UUID(10002),
		CreatedAt:      createdAt,
		Groups: []TestcaseGroup{
			{
				ID:                task2UUID(10003),
				TestsetRevisionID: task2UUID(10000),
				GroupIndex:        0,
				StopOnFailure:     false,
				CreatedAt:         createdAt,
				Cases: []TestcaseRevision{
					{
						ID:                task2UUID(10004),
						TestCaseID:        task2UUID(10005),
						TestsetRevisionID: task2UUID(10000),
						ProblemID:         task2UUID(10001),
						Kind:              problemcontract.TestKindSample,
						GroupIndex:        0,
						OrderIndex:        0,
						InputAsset:        task2IdentityAsset(10006, []byte("1 2\n")),
						ExpectedAsset:     task2IdentityAsset(10007, []byte("3\n")),
						Weight:            1,
						StopOnFailure:     false,
						CreatedAt:         createdAt,
					},
				},
			},
			{
				ID:                task2UUID(10008),
				TestsetRevisionID: task2UUID(10000),
				GroupIndex:        1,
				StopOnFailure:     true,
				CreatedAt:         createdAt,
				Cases: []TestcaseRevision{
					{
						ID:                task2UUID(10009),
						TestCaseID:        task2UUID(10010),
						TestsetRevisionID: task2UUID(10000),
						ProblemID:         task2UUID(10001),
						Kind:              problemcontract.TestKindHidden,
						GroupIndex:        1,
						OrderIndex:        0,
						InputAsset:        task2IdentityAsset(10011, []byte("99 1\n")),
						ExpectedAsset:     task2IdentityAsset(10012, []byte("100\n")),
						Weight:            1,
						StopOnFailure:     true,
						CreatedAt:         createdAt,
					},
				},
			},
		},
	}
	for _, group := range testset.Groups {
		for _, testcase := range group.Cases {
			testset.TestCount++
			testset.TotalUncompressedBytes +=
				testcase.InputAsset.UncompressedSize + testcase.ExpectedAsset.UncompressedSize
		}
	}
	checksum, err := testset.ComputeChecksum()
	if err != nil {
		t.Fatalf("ComputeChecksum() error = %v", err)
	}
	testset.Checksum = checksum
	return testset
}

func task2MaximumTestset(t *testing.T) TestsetRevision {
	t.Helper()

	createdAt := time.Unix(1_700_000_000, 0).UTC()
	testset := TestsetRevision{
		ID:             task2UUID(11000),
		ProblemID:      task2UUID(11001),
		RevisionNumber: problemcontract.MaximumTestsetRevisionNumber,
		TestCount:      problemcontract.MaximumTestCases,
		TotalUncompressedBytes: int64(problemcontract.MaximumTestCases) *
			2 * int64(problemcontract.MaximumTestAssetBytes),
		CreatedBy: task2UUID(11002),
		CreatedAt: createdAt,
		Groups: []TestcaseGroup{{
			ID:                task2UUID(11003),
			TestsetRevisionID: task2UUID(11000),
			GroupIndex:        0,
			StopOnFailure:     true,
			CreatedAt:         createdAt,
			Cases:             make([]TestcaseRevision, problemcontract.MaximumTestCases),
		}},
	}
	for index := range testset.Groups[0].Cases {
		testset.Groups[0].Cases[index] = TestcaseRevision{
			ID:                task2UUID(uint64(12000 + index)),
			TestCaseID:        task2UUID(uint64(13000 + index)),
			TestsetRevisionID: testset.ID,
			ProblemID:         testset.ProblemID,
			Kind:              problemcontract.TestKindHidden,
			GroupIndex:        0,
			OrderIndex:        index,
			InputAsset: task2ZstdAsset(
				uint64(14000+index*2),
				[]byte{byte(index), 1},
				problemcontract.MaximumTestAssetBytes,
			),
			ExpectedAsset: task2ZstdAsset(
				uint64(14001+index*2),
				[]byte{byte(index), 2},
				problemcontract.MaximumTestAssetBytes,
			),
			Weight:        1,
			StopOnFailure: true,
			CreatedAt:     createdAt,
		}
	}
	checksum, err := testset.ComputeChecksum()
	if err != nil {
		t.Fatalf("maximum testset ComputeChecksum() error = %v", err)
	}
	testset.Checksum = checksum
	return testset
}

func task2TestsetWithGroupCount(t *testing.T, count int) TestsetRevision {
	t.Helper()

	createdAt := time.Unix(1_700_000_000, 0).UTC()
	testset := TestsetRevision{
		ID:             task2UUID(15000),
		ProblemID:      task2UUID(15001),
		RevisionNumber: 1,
		TestCount:      count,
		CreatedBy:      task2UUID(15002),
		CreatedAt:      createdAt,
		Groups:         make([]TestcaseGroup, count),
	}
	for index := range testset.Groups {
		input := task2IdentityAsset(uint64(16000+index*2), nil)
		expected := task2IdentityAsset(uint64(16001+index*2), nil)
		testset.Groups[index] = TestcaseGroup{
			ID:                task2UUID(uint64(17000 + index)),
			TestsetRevisionID: testset.ID,
			GroupIndex:        index,
			StopOnFailure:     index%2 == 0,
			CreatedAt:         createdAt,
			Cases: []TestcaseRevision{{
				ID:                task2UUID(uint64(18000 + index)),
				TestCaseID:        task2UUID(uint64(19000 + index)),
				TestsetRevisionID: testset.ID,
				ProblemID:         testset.ProblemID,
				Kind:              problemcontract.TestKindHidden,
				GroupIndex:        index,
				OrderIndex:        0,
				InputAsset:        input,
				ExpectedAsset:     expected,
				Weight:            1,
				StopOnFailure:     index%2 == 0,
				CreatedAt:         createdAt,
			}},
		}
	}
	checksum, err := testset.ComputeChecksum()
	if err != nil {
		t.Fatalf("group-boundary testset ComputeChecksum() error = %v", err)
	}
	testset.Checksum = checksum
	return testset
}

func task2IdentityAsset(id uint64, content []byte) TestAsset {
	owned := append([]byte(nil), content...)
	return TestAsset{
		ID:               task2UUID(id),
		SHA256:           sha256.Sum256(owned),
		Encoding:         problemcontract.AssetEncodingIdentity,
		Content:          owned,
		UncompressedSize: int64(len(owned)),
		CreatedAt:        time.Unix(1_700_000_000, 0).UTC(),
	}
}

func task2ZstdAsset(id uint64, encoded []byte, uncompressedSize int64) TestAsset {
	// The digest is the identity of the hypothetical uncompressed bytes. A
	// vendor-specific verifier is deliberately required to prove it.
	digest := sha256.Sum256([]byte(fmt.Sprintf("uncompressed:%d:%d", id, uncompressedSize)))
	return TestAsset{
		ID:               task2UUID(id),
		SHA256:           digest,
		Encoding:         problemcontract.AssetEncodingZstd,
		Content:          append([]byte(nil), encoded...),
		UncompressedSize: uncompressedSize,
		CreatedAt:        time.Unix(1_700_000_000, 0).UTC(),
	}
}

func cloneTask2Asset(value TestAsset) TestAsset {
	cloned := value
	cloned.Content = append([]byte(nil), value.Content...)
	return cloned
}

func cloneTask2Testset(value TestsetRevision) TestsetRevision {
	cloned := value
	cloned.Groups = make([]TestcaseGroup, len(value.Groups))
	for groupIndex, group := range value.Groups {
		cloned.Groups[groupIndex] = group
		cloned.Groups[groupIndex].Cases = make([]TestcaseRevision, len(group.Cases))
		for caseIndex, testcase := range group.Cases {
			cloned.Groups[groupIndex].Cases[caseIndex] = testcase
			cloned.Groups[groupIndex].Cases[caseIndex].InputAsset = cloneTask2Asset(testcase.InputAsset)
			cloned.Groups[groupIndex].Cases[caseIndex].ExpectedAsset = cloneTask2Asset(testcase.ExpectedAsset)
		}
	}
	return cloned
}

type task2RecordingVerifier struct {
	calls   map[uuid.UUID]int
	total   int
	failure error
}

func (verifier *task2RecordingVerifier) Verify(ctx context.Context, asset TestAsset) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	verifier.total++
	if verifier.calls != nil {
		verifier.calls[asset.ID]++
	}
	if verifier.failure != nil {
		return verifier.failure
	}
	return asset.ValidateMetadata()
}

type task2PublicDecoder struct {
	hidden   map[uuid.UUID]struct{}
	decoded  []uuid.UUID
	override []byte
	failure  error
}

func (decoder *task2PublicDecoder) Decode(ctx context.Context, asset TestAsset) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, forbidden := decoder.hidden[asset.ID]; forbidden {
		panic("hidden asset passed to public decoder")
	}
	decoder.decoded = append(decoder.decoded, asset.ID)
	if decoder.failure != nil {
		return nil, decoder.failure
	}
	if decoder.override != nil {
		return append([]byte(nil), decoder.override...), nil
	}
	if !utf8.Valid(asset.Content) {
		return nil, ErrTestAssetIntegrity
	}
	return append([]byte(nil), asset.Content...), nil
}

var _ AssetVerifier = (*task2RecordingVerifier)(nil)
var _ AssetDecoder = (*task2PublicDecoder)(nil)

// Ensure the maximum-size test remains allocation-light: encoded zstd
// fixtures are tiny and only their bounded uncompressed metadata is large.
func TestMaximumTestsetFixtureDoesNotMaterializeUncompressedPayloads(t *testing.T) {
	t.Parallel()

	testset := task2MaximumTestset(t)
	var encodedBytes int
	for _, group := range testset.Groups {
		for _, testcase := range group.Cases {
			encodedBytes += len(testcase.InputAsset.Content) + len(testcase.ExpectedAsset.Content)
		}
	}
	if encodedBytes > problemcontract.MaximumTestCases*4 {
		t.Fatalf("encoded fixture unexpectedly materialized large payloads: %d bytes", encodedBytes)
	}
	if bytes.Equal(testset.Checksum[:], make([]byte, sha256.Size)) {
		t.Fatal("maximum testset checksum is zero")
	}
}

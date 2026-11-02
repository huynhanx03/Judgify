package domain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

var (
	ErrInvalidTestAsset       = errors.New("problem: invalid test asset")
	ErrTestAssetIntegrity     = errors.New("problem: test asset integrity mismatch")
	ErrInvalidTestsetRevision = errors.New("problem: invalid immutable testset")
	ErrTestsetChecksum        = errors.New("problem: testset checksum mismatch")
	ErrHiddenTestRequired     = errors.New("problem: at least one hidden test is required")
	ErrNonContiguousTests     = errors.New("problem: testcase ordering is not contiguous")
	ErrUnsafeSampleProjection = errors.New("problem: unsafe public sample projection")
)

// TestAsset stores encoded bytes internally. Every field is excluded from JSON
// so an accidental domain-entity serialization cannot expose hidden bytes,
// asset identities, checksums, or storage metadata. Authorized adapters must
// always use an explicit projection.
type TestAsset struct {
	ID               uuid.UUID `json:"-"`
	SHA256           [32]byte  `json:"-"`
	Encoding         string    `json:"-"`
	Content          []byte    `json:"-"`
	UncompressedSize int64     `json:"-"`
	CreatedAt        time.Time `json:"-"`
}

// ValidateMetadata bounds allocation before any optional decompression. For
// identity assets it can also verify the complete uncompressed checksum.
func (asset TestAsset) ValidateMetadata() error {
	if err := asset.ValidateReferenceMetadata(); err != nil ||
		len(asset.Content) > problemcontract.MaximumTestAssetBytes {
		return ErrInvalidTestAsset
	}
	switch asset.Encoding {
	case problemcontract.AssetEncodingIdentity:
		if int64(len(asset.Content)) != asset.UncompressedSize ||
			sha256.Sum256(asset.Content) != asset.SHA256 {
			return ErrTestAssetIntegrity
		}
	case problemcontract.AssetEncodingZstd:
		// Even an empty uncompressed stream has a non-empty zstd frame.
		if len(asset.Content) == 0 {
			return ErrInvalidTestAsset
		}
	default:
		return ErrInvalidTestAsset
	}
	return nil
}

// ValidateReferenceMetadata verifies the immutable identity used by testset
// checksums without requiring encoded content to be loaded. Judge planning
// uses this to keep a 512 MiB testset from becoming one large allocation; the
// exact asset bytes are fetched and fully verified immediately before a case.
func (asset TestAsset) ValidateReferenceMetadata() error {
	if asset.ID == uuid.Nil || asset.SHA256 == ([32]byte{}) ||
		!problemcontract.IsAssetEncoding(asset.Encoding) ||
		asset.UncompressedSize < 0 ||
		asset.UncompressedSize > problemcontract.MaximumTestAssetBytes ||
		asset.CreatedAt.IsZero() {
		return ErrInvalidTestAsset
	}
	return nil
}

// AssetVerifier validates an encoded asset while streaming into a bounded
// sink. The identity implementation is local; zstd support is supplied by a
// narrow adapter so the domain does not depend on a compression vendor.
type AssetVerifier interface {
	Verify(ctx context.Context, asset TestAsset) error
}

type IdentityAssetVerifier struct{}

func (IdentityAssetVerifier) Verify(ctx context.Context, asset TestAsset) error {
	if ctx == nil {
		return ErrInvalidTestAsset
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if asset.Encoding != problemcontract.AssetEncodingIdentity {
		return ErrInvalidTestAsset
	}
	return asset.ValidateMetadata()
}

// TestcaseRevision is the authoritative immutable testcase snapshot. TestCaseID
// is the stable lineage identity retained across authored testset revisions.
type TestcaseRevision struct {
	ID                uuid.UUID
	TestCaseID        uuid.UUID
	TestsetRevisionID uuid.UUID
	ProblemID         uuid.UUID
	Kind              string
	GroupIndex        int
	OrderIndex        int
	InputAsset        TestAsset
	ExpectedAsset     TestAsset
	Weight            int
	StopOnFailure     bool
	CreatedAt         time.Time
}

// TestcaseGroup makes ordering and group policy explicit. The duplicated
// StopOnFailure value on each revision is checked for equality and is also
// protected by a composite database foreign key.
type TestcaseGroup struct {
	ID                uuid.UUID
	TestsetRevisionID uuid.UUID
	GroupIndex        int
	StopOnFailure     bool
	CreatedAt         time.Time
	Cases             []TestcaseRevision
}

type TestsetRevision struct {
	ID                     uuid.UUID
	ProblemID              uuid.UUID
	RevisionNumber         int64
	Checksum               [32]byte
	TestCount              int
	TotalUncompressedBytes int64
	CreatedBy              uuid.UUID
	CreatedAt              time.Time
	Groups                 []TestcaseGroup
}

func (testset TestsetRevision) ValidateStructure() error {
	return testset.validateStructure(true)
}

// ValidateReferenceStructure proves ordering, policy, content identities, and
// the testset checksum without loading asset bodies. It is reserved for the
// trusted judge planner; each body is still decoded and hash-verified before
// its testcase executes.
func (testset TestsetRevision) ValidateReferenceStructure() error {
	return testset.validateStructure(false)
}

func (testset TestsetRevision) validateStructure(requireContent bool) error {
	if testset.ID == uuid.Nil || testset.ProblemID == uuid.Nil ||
		testset.RevisionNumber < 1 ||
		testset.RevisionNumber > problemcontract.MaximumTestsetRevisionNumber ||
		testset.TestCount < 1 || testset.TestCount > problemcontract.MaximumTestCases ||
		testset.TotalUncompressedBytes < 0 ||
		testset.TotalUncompressedBytes > problemcontract.MaximumTestsetUncompressedBytes ||
		testset.CreatedBy == uuid.Nil || testset.CreatedAt.IsZero() ||
		len(testset.Groups) == 0 ||
		len(testset.Groups) > problemcontract.MaximumTestcaseGroups {
		return ErrInvalidTestsetRevision
	}

	seenRevisionIDs := make(map[uuid.UUID]struct{}, testset.TestCount)
	seenCaseIDs := make(map[uuid.UUID]struct{}, testset.TestCount)
	actualCount := 0
	var actualBytes int64
	hidden := 0
	for groupPosition, group := range testset.Groups {
		if group.ID == uuid.Nil || group.TestsetRevisionID != testset.ID ||
			group.GroupIndex != groupPosition || group.CreatedAt.IsZero() ||
			len(group.Cases) == 0 {
			return ErrNonContiguousTests
		}
		for casePosition, test := range group.Cases {
			if test.ID == uuid.Nil || test.TestCaseID == uuid.Nil ||
				test.TestsetRevisionID != testset.ID ||
				test.ProblemID != testset.ProblemID ||
				test.GroupIndex != groupPosition ||
				test.OrderIndex != casePosition ||
				test.StopOnFailure != group.StopOnFailure ||
				!problemcontract.IsTestKind(test.Kind) ||
				test.Weight != 1 || test.CreatedAt.IsZero() {
				return ErrNonContiguousTests
			}
			if _, duplicate := seenRevisionIDs[test.ID]; duplicate {
				return ErrInvalidTestsetRevision
			}
			if _, duplicate := seenCaseIDs[test.TestCaseID]; duplicate {
				return ErrInvalidTestsetRevision
			}
			seenRevisionIDs[test.ID] = struct{}{}
			seenCaseIDs[test.TestCaseID] = struct{}{}
			if err := validateStructuredAsset(test.InputAsset, requireContent); err != nil {
				return err
			}
			if err := validateStructuredAsset(test.ExpectedAsset, requireContent); err != nil {
				return err
			}
			if test.Kind == problemcontract.TestKindHidden {
				hidden++
			}
			actualCount++
			actualBytes += test.InputAsset.UncompressedSize + test.ExpectedAsset.UncompressedSize
			if actualBytes > problemcontract.MaximumTestsetUncompressedBytes {
				return ErrInvalidTestsetRevision
			}
		}
	}
	if hidden == 0 {
		return ErrHiddenTestRequired
	}
	if actualCount != testset.TestCount ||
		actualBytes != testset.TotalUncompressedBytes {
		return ErrInvalidTestsetRevision
	}
	computed, err := testset.ComputeChecksum()
	if err != nil {
		return err
	}
	if testset.Checksum == ([32]byte{}) || testset.Checksum != computed {
		return errors.Join(ErrInvalidTestsetRevision, ErrTestsetChecksum)
	}
	return nil
}

func validateStructuredAsset(asset TestAsset, requireContent bool) error {
	if requireContent {
		return asset.ValidateMetadata()
	}
	return asset.ValidateReferenceMetadata()
}

// VerifyAssets checks each content-addressed asset once even when snapshots
// intentionally reuse it across cases.
func (testset TestsetRevision) VerifyAssets(ctx context.Context, verifier AssetVerifier) error {
	if ctx == nil || verifier == nil {
		return ErrInvalidTestAsset
	}
	seen := make(map[uuid.UUID]TestAsset, testset.TestCount*2)
	for _, group := range testset.Groups {
		for _, test := range group.Cases {
			for _, asset := range [...]TestAsset{test.InputAsset, test.ExpectedAsset} {
				if previous, exists := seen[asset.ID]; exists {
					if !sameTestAsset(previous, asset) {
						return ErrTestAssetIntegrity
					}
					continue
				}
				if err := verifier.Verify(ctx, asset); err != nil {
					return err
				}
				seen[asset.ID] = cloneTestAsset(asset)
			}
		}
	}
	return nil
}

// ComputeChecksum is independent of generated row IDs. It binds the stable
// testcase lineage, deterministic order, policy, and content digests.
func (testset TestsetRevision) ComputeChecksum() ([32]byte, error) {
	if testset.ProblemID == uuid.Nil || len(testset.Groups) == 0 ||
		len(testset.Groups) > problemcontract.MaximumTestcaseGroups {
		return [32]byte{}, ErrInvalidTestsetRevision
	}
	digest := sha256.New()
	writeChecksumPart(digest, []byte("judgify.problem-testset.v1"))
	writeChecksumPart(digest, testset.ProblemID[:])
	writeChecksumInt64(digest, int64(len(testset.Groups)))
	count := 0
	for groupPosition, group := range testset.Groups {
		if group.GroupIndex != groupPosition || len(group.Cases) == 0 {
			return [32]byte{}, ErrNonContiguousTests
		}
		writeChecksumInt64(digest, int64(group.GroupIndex))
		writeChecksumBool(digest, group.StopOnFailure)
		writeChecksumInt64(digest, int64(len(group.Cases)))
		for casePosition, test := range group.Cases {
			if test.TestCaseID == uuid.Nil ||
				test.TestsetRevisionID != testset.ID ||
				test.ProblemID != testset.ProblemID ||
				test.GroupIndex != groupPosition ||
				test.OrderIndex != casePosition ||
				test.StopOnFailure != group.StopOnFailure ||
				!problemcontract.IsTestKind(test.Kind) ||
				test.Weight != 1 {
				return [32]byte{}, ErrNonContiguousTests
			}
			if err := test.InputAsset.ValidateReferenceMetadata(); err != nil {
				return [32]byte{}, err
			}
			if err := test.ExpectedAsset.ValidateReferenceMetadata(); err != nil {
				return [32]byte{}, err
			}
			writeChecksumPart(digest, test.TestCaseID[:])
			writeChecksumPart(digest, []byte(test.Kind))
			writeChecksumInt64(digest, int64(test.GroupIndex))
			writeChecksumInt64(digest, int64(test.OrderIndex))
			writeChecksumPart(digest, test.InputAsset.SHA256[:])
			writeChecksumInt64(digest, test.InputAsset.UncompressedSize)
			writeChecksumPart(digest, test.ExpectedAsset.SHA256[:])
			writeChecksumInt64(digest, test.ExpectedAsset.UncompressedSize)
			writeChecksumInt64(digest, int64(test.Weight))
			writeChecksumBool(digest, test.StopOnFailure)
			count++
			if count > problemcontract.MaximumTestCases {
				return [32]byte{}, ErrInvalidTestsetRevision
			}
		}
	}
	if count == 0 {
		return [32]byte{}, ErrInvalidTestsetRevision
	}
	return checksumFromHash(digest), nil
}

// AssetDecoder returns verified uncompressed bytes. Implementations must apply
// the same allocation bounds as publication verification.
type AssetDecoder interface {
	Decode(ctx context.Context, asset TestAsset) ([]byte, error)
}

type IdentityAssetDecoder struct{}

func (IdentityAssetDecoder) Decode(ctx context.Context, asset TestAsset) ([]byte, error) {
	if err := (IdentityAssetVerifier{}).Verify(ctx, asset); err != nil {
		return nil, err
	}
	return append([]byte(nil), asset.Content...), nil
}

// PublicSample deliberately has no testcase, asset, checksum, visibility, or
// provenance identifiers.
type PublicSample struct {
	GroupIndex     int    `json:"group_index"`
	OrderIndex     int    `json:"order_index"`
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
}

// BuildPublicSamples resolves only sample assets. Hidden rows are never passed
// to the decoder, which makes accidental hidden-content access testable.
func BuildPublicSamples(
	ctx context.Context,
	testset TestsetRevision,
	decoder AssetDecoder,
) ([]PublicSample, error) {
	if ctx == nil || decoder == nil {
		return nil, ErrUnsafeSampleProjection
	}
	samples := make([]PublicSample, 0)
	for _, group := range testset.Groups {
		for _, test := range group.Cases {
			if test.Kind != problemcontract.TestKindSample {
				continue
			}
			input, err := decoder.Decode(ctx, test.InputAsset)
			if err != nil {
				return nil, errors.Join(ErrUnsafeSampleProjection, err)
			}
			expected, err := decoder.Decode(ctx, test.ExpectedAsset)
			if err != nil {
				return nil, errors.Join(ErrUnsafeSampleProjection, err)
			}
			if int64(len(input)) != test.InputAsset.UncompressedSize ||
				sha256.Sum256(input) != test.InputAsset.SHA256 ||
				int64(len(expected)) != test.ExpectedAsset.UncompressedSize ||
				sha256.Sum256(expected) != test.ExpectedAsset.SHA256 ||
				!utf8.Valid(input) || !utf8.Valid(expected) ||
				bytes.IndexByte(input, 0) >= 0 || bytes.IndexByte(expected, 0) >= 0 {
				return nil, ErrUnsafeSampleProjection
			}
			samples = append(samples, PublicSample{
				GroupIndex: group.GroupIndex, OrderIndex: test.OrderIndex,
				Input: string(input), ExpectedOutput: string(expected),
			})
		}
	}
	return samples, nil
}

func sameTestAsset(first, second TestAsset) bool {
	return first.ID == second.ID &&
		first.SHA256 == second.SHA256 &&
		first.Encoding == second.Encoding &&
		first.UncompressedSize == second.UncompressedSize &&
		first.CreatedAt.Equal(second.CreatedAt) &&
		bytes.Equal(first.Content, second.Content)
}

func cloneTestAsset(asset TestAsset) TestAsset {
	asset.Content = append([]byte(nil), asset.Content...)
	return asset
}

func writeChecksumBool(digest hash.Hash, value bool) {
	if value {
		writeChecksumPart(digest, []byte{1})
		return
	}
	writeChecksumPart(digest, []byte{0})
}

func (testset TestsetRevision) String() string {
	return fmt.Sprintf(
		"testset(problem=%s revision=%d tests=%d)",
		testset.ProblemID,
		testset.RevisionNumber,
		testset.TestCount,
	)
}

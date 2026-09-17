package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

var (
	ErrInvalidProblemRevision = errors.New("problem: invalid immutable revision")
	ErrRevisionChecksum       = errors.New("problem: revision checksum mismatch")
	ErrRewardChecksum         = errors.New("problem: reward snapshot checksum mismatch")
	ErrNonCanonicalJSON       = errors.New("problem: JSON is not canonical")
)

// ValidationError carries only a stable field/reason pair. It deliberately
// never formats authored source, checker configuration, or hidden data.
type ValidationError struct {
	Field  string
	Reason string
}

func (err *ValidationError) Error() string {
	if err == nil {
		return ErrInvalidProblemRevision.Error()
	}
	return fmt.Sprintf("%s: %s (%s)", ErrInvalidProblemRevision, err.Field, err.Reason)
}

func (*ValidationError) Unwrap() error { return ErrInvalidProblemRevision }

// Limits is the immutable per-test resource policy copied into a revision.
type Limits struct {
	CPUTimeMilliseconds  int
	WallTimeMilliseconds int
	MemoryKilobytes      int
	OutputBytes          int
	ProcessCount         int
}

func (limits Limits) Validate() error {
	switch {
	case limits.CPUTimeMilliseconds < problemcontract.MinimumCPUTimeLimitMilliseconds ||
		limits.CPUTimeMilliseconds > problemcontract.MaximumCPUTimeLimitMilliseconds:
		return invalidRevisionField("cpu_time_limit_ms", "out_of_range")
	case limits.WallTimeMilliseconds < limits.CPUTimeMilliseconds ||
		limits.WallTimeMilliseconds > problemcontract.MaximumWallTimeLimitMilliseconds:
		return invalidRevisionField("wall_time_limit_ms", "out_of_range")
	case limits.MemoryKilobytes < problemcontract.MinimumMemoryLimitKilobytes ||
		limits.MemoryKilobytes > problemcontract.MaximumMemoryLimitKilobytes:
		return invalidRevisionField("memory_limit_kb", "out_of_range")
	case limits.OutputBytes < problemcontract.MinimumOutputLimitBytes ||
		limits.OutputBytes > problemcontract.MaximumOutputLimitBytes:
		return invalidRevisionField("output_limit_bytes", "out_of_range")
	case limits.ProcessCount < problemcontract.MinimumProcessLimit ||
		limits.ProcessCount > problemcontract.MaximumProcessLimit:
		return invalidRevisionField("process_limit", "out_of_range")
	default:
		return nil
	}
}

type TagSnapshotItem struct {
	TagID       uuid.UUID `json:"tag_id"`
	TagRevision int64     `json:"tag_revision"`
}

type TagMappingSnapshot struct {
	MappingID uuid.UUID         `json:"mapping_id"`
	Revision  int64             `json:"revision"`
	Items     []TagSnapshotItem `json:"items"`
}

type ElementSnapshotItem struct {
	ElementID         uuid.UUID `json:"element_id"`
	ElementRevision   int64     `json:"element_revision"`
	WeightBasisPoints int       `json:"weight_bps"`
}

type ElementMappingSnapshot struct {
	MappingID uuid.UUID             `json:"mapping_id"`
	Revision  int64                 `json:"revision"`
	Items     []ElementSnapshotItem `json:"items"`
}

// RewardSnapshot freezes every taxonomy/reward fact needed by downstream
// projections. Catalog revision validation is performed through a port because
// the current taxonomy tables remain independently owned.
type RewardSnapshot struct {
	DifficultyID       uuid.UUID
	DifficultyRevision int64
	BaseExperience     int64
	Tags               TagMappingSnapshot
	Elements           *ElementMappingSnapshot
	Checksum           [32]byte
}

func (snapshot RewardSnapshot) Validate() error {
	if snapshot.DifficultyID == uuid.Nil ||
		snapshot.DifficultyRevision < 1 ||
		snapshot.DifficultyRevision > problemcontract.MaximumDifficultyRevision {
		return invalidRevisionField("difficulty", "invalid_snapshot")
	}
	if snapshot.BaseExperience < 0 ||
		snapshot.BaseExperience > problemcontract.MaximumBaseExperience {
		return invalidRevisionField("base_exp", "out_of_range")
	}
	if err := validateTagMapping(snapshot.Tags); err != nil {
		return err
	}
	if snapshot.Elements != nil {
		if err := validateElementMapping(*snapshot.Elements); err != nil {
			return err
		}
	}
	computed, err := snapshot.ComputeChecksum()
	if err != nil {
		return err
	}
	if snapshot.Checksum == ([32]byte{}) || snapshot.Checksum != computed {
		return errors.Join(ErrInvalidProblemRevision, ErrRewardChecksum)
	}
	return nil
}

// ComputeChecksum returns the v1 canonical reward digest without mutating the
// receiver. Callers set Checksum explicitly before persistence.
func (snapshot RewardSnapshot) ComputeChecksum() ([32]byte, error) {
	if snapshot.DifficultyID == uuid.Nil ||
		snapshot.DifficultyRevision < 1 ||
		snapshot.DifficultyRevision > problemcontract.MaximumDifficultyRevision ||
		snapshot.BaseExperience < 0 ||
		snapshot.BaseExperience > problemcontract.MaximumBaseExperience {
		return [32]byte{}, invalidRevisionField("reward_snapshot", "invalid")
	}
	if err := validateTagMapping(snapshot.Tags); err != nil {
		return [32]byte{}, err
	}
	if snapshot.Elements != nil {
		if err := validateElementMapping(*snapshot.Elements); err != nil {
			return [32]byte{}, err
		}
	}

	digest := sha256.New()
	writeChecksumPart(digest, []byte("judgify.problem-reward-snapshot.v1"))
	writeChecksumPart(digest, snapshot.DifficultyID[:])
	writeChecksumInt64(digest, snapshot.DifficultyRevision)
	writeChecksumInt64(digest, snapshot.BaseExperience)
	writeChecksumPart(digest, snapshot.Tags.MappingID[:])
	writeChecksumInt64(digest, snapshot.Tags.Revision)
	writeChecksumInt64(digest, int64(len(snapshot.Tags.Items)))
	for _, item := range snapshot.Tags.Items {
		writeChecksumPart(digest, item.TagID[:])
		writeChecksumInt64(digest, item.TagRevision)
	}
	if snapshot.Elements == nil {
		writeChecksumPart(digest, nil)
	} else {
		writeChecksumPart(digest, []byte{1})
		writeChecksumPart(digest, snapshot.Elements.MappingID[:])
		writeChecksumInt64(digest, snapshot.Elements.Revision)
		writeChecksumInt64(digest, int64(len(snapshot.Elements.Items)))
		for _, item := range snapshot.Elements.Items {
			writeChecksumPart(digest, item.ElementID[:])
			writeChecksumInt64(digest, item.ElementRevision)
			writeChecksumInt64(digest, int64(item.WeightBasisPoints))
		}
	}
	return checksumFromHash(digest), nil
}

// Revision is one insert-only authored and judge-policy snapshot.
type Revision struct {
	ID                   uuid.UUID
	ProblemID            uuid.UUID
	RevisionNumber       int64
	Title                string
	StatementMarkdown    []byte
	RenderSourceID       uuid.UUID
	RenderSourceKind     string
	RenderSourceChecksum [32]byte
	Reward               RewardSnapshot
	Limits               Limits
	CheckerKey           string
	CheckerVersion       string
	CheckerConfig        json.RawMessage
	TestsetRevisionID    uuid.UUID
	AllowedRuntimeKeys   []string
	CreatedBy            uuid.UUID
	CreatedAt            time.Time
	ContentChecksum      [32]byte
}

func (revision Revision) Validate() error {
	if revision.ID == uuid.Nil || revision.ProblemID == uuid.Nil {
		return invalidRevisionField("identity", "nil_uuid")
	}
	if revision.RevisionNumber < 1 ||
		revision.RevisionNumber > problemcontract.MaximumProblemRevisionNumber {
		return invalidRevisionField("revision_number", "out_of_range")
	}
	if err := validateBoundedText("title", revision.Title, problemcontract.MaximumTitleBytes, true); err != nil {
		return err
	}
	normalized, err := contentrender.NormalizeMarkdown(revision.StatementMarkdown)
	if err != nil || len(revision.StatementMarkdown) == 0 ||
		len(revision.StatementMarkdown) > problemcontract.MaximumStatementMarkdownBytes ||
		strings.TrimSpace(string(revision.StatementMarkdown)) == "" ||
		!bytes.Equal(normalized.Bytes, revision.StatementMarkdown) {
		return invalidRevisionField("statement_markdown", "not_canonical")
	}
	if revision.RenderSourceID == uuid.Nil ||
		revision.RenderSourceKind != problemcontract.RenderSourceKindProblemStatement ||
		revision.RenderSourceChecksum == ([32]byte{}) ||
		revision.RenderSourceChecksum != normalized.Checksum {
		return invalidRevisionField("render_source", "identity_mismatch")
	}
	if err := revision.Reward.Validate(); err != nil {
		return err
	}
	if err := revision.Limits.Validate(); err != nil {
		return err
	}
	if !validProtocolIdentifier(revision.CheckerKey, problemcontract.MaximumCheckerKeyBytes) {
		return invalidRevisionField("checker_key", "invalid")
	}
	if !validProtocolIdentifier(revision.CheckerVersion, problemcontract.MaximumCheckerVersionBytes) {
		return invalidRevisionField("checker_version", "invalid")
	}
	canonicalConfig, err := canonicalJSONObject(revision.CheckerConfig, problemcontract.MaximumCheckerConfigBytes)
	if err != nil || !bytes.Equal(canonicalConfig, revision.CheckerConfig) {
		return errors.Join(ErrInvalidProblemRevision, ErrNonCanonicalJSON)
	}
	if revision.TestsetRevisionID == uuid.Nil {
		return invalidRevisionField("testset_revision_id", "nil_uuid")
	}
	if err := validateRuntimeKeys(revision.AllowedRuntimeKeys); err != nil {
		return err
	}
	if revision.CreatedBy == uuid.Nil || revision.CreatedAt.IsZero() {
		return invalidRevisionField("provenance", "invalid")
	}
	computed, err := revision.ComputeContentChecksum()
	if err != nil {
		return err
	}
	if revision.ContentChecksum == ([32]byte{}) || revision.ContentChecksum != computed {
		return errors.Join(ErrInvalidProblemRevision, ErrRevisionChecksum)
	}
	return nil
}

// ComputeContentChecksum binds authored source and every executable/reward
// policy fact, but intentionally excludes row identity and timestamps.
func (revision Revision) ComputeContentChecksum() ([32]byte, error) {
	if err := validateBoundedText("title", revision.Title, problemcontract.MaximumTitleBytes, true); err != nil {
		return [32]byte{}, err
	}
	normalized, err := contentrender.NormalizeMarkdown(revision.StatementMarkdown)
	if err != nil || len(revision.StatementMarkdown) == 0 ||
		len(revision.StatementMarkdown) > problemcontract.MaximumStatementMarkdownBytes ||
		strings.TrimSpace(string(revision.StatementMarkdown)) == "" ||
		!bytes.Equal(normalized.Bytes, revision.StatementMarkdown) {
		return [32]byte{}, invalidRevisionField("statement_markdown", "not_canonical")
	}
	if revision.RenderSourceKind != problemcontract.RenderSourceKindProblemStatement ||
		revision.RenderSourceChecksum != normalized.Checksum {
		return [32]byte{}, invalidRevisionField("render_source", "identity_mismatch")
	}
	rewardChecksum, err := revision.Reward.ComputeChecksum()
	if err != nil || revision.Reward.Checksum != rewardChecksum {
		return [32]byte{}, errors.Join(ErrInvalidProblemRevision, ErrRewardChecksum)
	}
	if err := revision.Limits.Validate(); err != nil {
		return [32]byte{}, err
	}
	if !validProtocolIdentifier(revision.CheckerKey, problemcontract.MaximumCheckerKeyBytes) ||
		!validProtocolIdentifier(revision.CheckerVersion, problemcontract.MaximumCheckerVersionBytes) {
		return [32]byte{}, invalidRevisionField("checker", "invalid")
	}
	canonicalConfig, err := canonicalJSONObject(revision.CheckerConfig, problemcontract.MaximumCheckerConfigBytes)
	if err != nil || !bytes.Equal(canonicalConfig, revision.CheckerConfig) {
		return [32]byte{}, errors.Join(ErrInvalidProblemRevision, ErrNonCanonicalJSON)
	}
	if revision.TestsetRevisionID == uuid.Nil {
		return [32]byte{}, invalidRevisionField("testset_revision_id", "nil_uuid")
	}
	if err := validateRuntimeKeys(revision.AllowedRuntimeKeys); err != nil {
		return [32]byte{}, err
	}

	digest := sha256.New()
	writeChecksumPart(digest, []byte("judgify.problem-revision-content.v1"))
	writeChecksumPart(digest, []byte(revision.Title))
	writeChecksumPart(digest, normalized.Bytes)
	writeChecksumPart(digest, []byte(revision.RenderSourceKind))
	writeChecksumPart(digest, revision.RenderSourceChecksum[:])
	writeChecksumPart(digest, rewardChecksum[:])
	writeChecksumInt64(digest, int64(revision.Limits.CPUTimeMilliseconds))
	writeChecksumInt64(digest, int64(revision.Limits.WallTimeMilliseconds))
	writeChecksumInt64(digest, int64(revision.Limits.MemoryKilobytes))
	writeChecksumInt64(digest, int64(revision.Limits.OutputBytes))
	writeChecksumInt64(digest, int64(revision.Limits.ProcessCount))
	writeChecksumPart(digest, []byte(revision.CheckerKey))
	writeChecksumPart(digest, []byte(revision.CheckerVersion))
	writeChecksumPart(digest, canonicalConfig)
	writeChecksumPart(digest, revision.TestsetRevisionID[:])
	writeChecksumInt64(digest, int64(len(revision.AllowedRuntimeKeys)))
	for _, runtimeKey := range revision.AllowedRuntimeKeys {
		writeChecksumPart(digest, []byte(runtimeKey))
	}
	return checksumFromHash(digest), nil
}

// CanonicalizeCheckerConfig validates a bounded JSON object, rejects duplicate
// keys/excessive nesting, and returns owned deterministic bytes.
func CanonicalizeCheckerConfig(value json.RawMessage) (json.RawMessage, error) {
	canonical, err := canonicalJSONObject(value, problemcontract.MaximumCheckerConfigBytes)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(canonical), nil
}

func validateTagMapping(snapshot TagMappingSnapshot) error {
	if snapshot.MappingID == uuid.Nil || snapshot.Revision < 1 ||
		snapshot.Revision > problemcontract.MaximumRewardMappingRevision ||
		len(snapshot.Items) > problemcontract.MaximumTags {
		return invalidRevisionField("tag_snapshot", "invalid")
	}
	previous := uuid.Nil
	for index, item := range snapshot.Items {
		if item.TagID == uuid.Nil || item.TagRevision < 1 ||
			item.TagRevision > problemcontract.MaximumRewardMappingRevision {
			return invalidRevisionField("tag_snapshot", "invalid_item")
		}
		if index > 0 && bytes.Compare(previous[:], item.TagID[:]) >= 0 {
			return invalidRevisionField("tag_snapshot", "not_strictly_sorted")
		}
		previous = item.TagID
	}
	return nil
}

func validateElementMapping(snapshot ElementMappingSnapshot) error {
	if snapshot.MappingID == uuid.Nil || snapshot.Revision < 1 ||
		snapshot.Revision > problemcontract.MaximumRewardMappingRevision ||
		len(snapshot.Items) == 0 || len(snapshot.Items) > problemcontract.MaximumElements {
		return invalidRevisionField("element_snapshot", "invalid")
	}
	previous := uuid.Nil
	totalWeight := 0
	for index, item := range snapshot.Items {
		if item.ElementID == uuid.Nil || item.ElementRevision < 1 ||
			item.ElementRevision > problemcontract.MaximumRewardMappingRevision ||
			item.WeightBasisPoints < 1 ||
			item.WeightBasisPoints > problemcontract.MaximumRewardWeightBasisPoints {
			return invalidRevisionField("element_snapshot", "invalid_item")
		}
		if index > 0 && bytes.Compare(previous[:], item.ElementID[:]) >= 0 {
			return invalidRevisionField("element_snapshot", "not_strictly_sorted")
		}
		totalWeight += item.WeightBasisPoints
		if totalWeight > problemcontract.MaximumRewardWeightBasisPoints {
			return invalidRevisionField("element_snapshot", "weight_overflow")
		}
		previous = item.ElementID
	}
	if totalWeight != problemcontract.MaximumRewardWeightBasisPoints {
		return invalidRevisionField("element_snapshot", "weight_total")
	}
	return nil
}

func validateRuntimeKeys(values []string) error {
	if len(values) == 0 || len(values) > problemcontract.MaximumAllowedRuntimes ||
		!slices.IsSorted(values) {
		return invalidRevisionField("allowed_runtime_keys", "invalid")
	}
	for index, value := range values {
		if !validProtocolIdentifier(value, problemcontract.MaximumRuntimeKeyBytes) ||
			(index > 0 && value == values[index-1]) {
			return invalidRevisionField("allowed_runtime_keys", "invalid_item")
		}
	}
	return nil
}

func validateBoundedText(field, value string, maximumBytes int, rejectBlank bool) error {
	if maximumBytes < 1 || !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 ||
		len(value) > maximumBytes || (rejectBlank && strings.TrimSpace(value) == "") {
		return invalidRevisionField(field, "invalid_text")
	}
	return nil
}

func validProtocolIdentifier(value string, maximumBytes int) bool {
	if len(value) == 0 || len(value) > maximumBytes {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			(index > 0 && (character == '.' || character == '_' || character == '-')) {
			continue
		}
		return false
	}
	return true
}

func canonicalJSONObject(value json.RawMessage, maximumBytes int) ([]byte, error) {
	if len(value) == 0 || len(value) > maximumBytes {
		return nil, ErrNonCanonicalJSON
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoded, err := decodeCanonicalJSONValue(decoder, 0)
	if err != nil {
		return nil, errors.Join(ErrNonCanonicalJSON, err)
	}
	if _, ok := decoded.(map[string]any); !ok {
		return nil, ErrNonCanonicalJSON
	}
	if token, err := decoder.Token(); !errors.Is(err, io.EOF) || token != nil {
		return nil, ErrNonCanonicalJSON
	}
	canonical, err := json.Marshal(decoded)
	if err != nil || len(canonical) > maximumBytes {
		return nil, ErrNonCanonicalJSON
	}
	return canonical, nil
}

func decodeCanonicalJSONValue(decoder *json.Decoder, depth int) (any, error) {
	const maximumJSONDepth = 32
	if depth > maximumJSONDepth {
		return nil, errors.New("maximum JSON depth exceeded")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return token, nil
	}
	switch delimiter {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, errors.New("invalid JSON object key")
			}
			if _, duplicate := object[key]; duplicate {
				return nil, errors.New("duplicate JSON object key")
			}
			item, err := decodeCanonicalJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = item
		}
		if closeToken, err := decoder.Token(); err != nil || closeToken != json.Delim('}') {
			return nil, errors.New("unterminated JSON object")
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			item, err := decodeCanonicalJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, item)
		}
		if closeToken, err := decoder.Token(); err != nil || closeToken != json.Delim(']') {
			return nil, errors.New("unterminated JSON array")
		}
		return array, nil
	default:
		return nil, errors.New("unexpected JSON delimiter")
	}
}

func invalidRevisionField(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}

func writeChecksumPart(digest hash.Hash, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = digest.Write(length[:])
	_, _ = digest.Write(value)
}

func writeChecksumInt64(digest hash.Hash, value int64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(value))
	writeChecksumPart(digest, encoded[:])
}

func checksumFromHash(digest hash.Hash) [32]byte {
	var checksum [32]byte
	copy(checksum[:], digest.Sum(nil))
	return checksum
}

// Package contract owns stable, transport-independent Submission integration
// contracts and checksum preimages.
package contract

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash"
	"io"
	"regexp"

	"github.com/google/uuid"
)

const (
	AggregateSubmission        = "submission"
	EventSubmissionQueuedV1    = "submission.queued.v1"
	EventSubmissionCancelledV1 = "submission.cancelled.v1"

	judgeSpecChecksumDomain = "judgify.judge-spec.v2"
)

var ErrInvalidJudgeSpecChecksumInput = errors.New(
	"submission contract: invalid judge specification",
)

var (
	judgeSpecIdentifierPattern = regexp.MustCompile(
		`^[a-z0-9][a-z0-9._-]{0,127}$`,
	)
	judgeSpecDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// SubmissionQueuedV1 is intentionally browser-safe. It identifies durable
// work without exposing source, hidden tests, checker configuration, or
// execution internals.
type SubmissionQueuedV1 struct {
	SubmissionID uuid.UUID `json:"submission_id"`
	Generation   int64     `json:"generation"`
	JudgeSpecID  uuid.UUID `json:"judge_spec_id"`
	JobID        uuid.UUID `json:"job_id"`
	Lane         string    `json:"lane"`
	QueuedAt     string    `json:"queued_at"`
	CID          string    `json:"cid"`
}

// SubmissionCancelledV1 is the terminal work-state fact emitted when an
// operator cancellation commits. It intentionally excludes the operator
// reason and actor identity; those remain in the immutable audit journal.
type SubmissionCancelledV1 struct {
	SubmissionID uuid.UUID `json:"submission_id"`
	Generation   int64     `json:"generation"`
	JobID        uuid.UUID `json:"job_id"`
	Phase        string    `json:"phase"`
	OccurredAt   string    `json:"occurred_at"`
	CID          string    `json:"cid"`
}

// JudgeSpecChecksumInput contains every immutable field written to one
// judge_specs row. Consumers recompute the same checksum before execution.
type JudgeSpecChecksumInput struct {
	SubmissionID uuid.UUID
	UserID       uuid.UUID
	ProblemID    uuid.UUID
	Generation   int64

	ProblemRevisionID uuid.UUID
	TestsetRevisionID uuid.UUID

	Language                string
	RuntimeKey              string
	RuntimeProfileVersion   string
	CompilerVersion         string
	RuntimeBootstrapVersion string
	SandboxImageDigest      string
	CompileCommandID        string
	RunCommandID            string

	ArtifactManifestVersion  string
	ArtifactManifestChecksum string
	ArtifactManifest         json.RawMessage
	SupervisorProtocol       string

	CheckerKey     string
	CheckerVersion string
	CheckerConfig  json.RawMessage

	ReleaseID                       string
	ApprovedReleaseManifestChecksum string
	CheckerImplementationChecksum   string
	CapabilityProfileHash           string
	RewardSnapshot                  json.RawMessage
	RewardSnapshotChecksum          [32]byte
	CPUTimeMilliseconds             int
	WallTimeMilliseconds            int
	MemoryKilobytes                 int
	OutputBytes                     int
	ProcessCount                    int
	CompileCPUMilliseconds          int
	CompileWallMilliseconds         int
	TotalWallMilliseconds           int
	SourceSHA256                    [32]byte
	JudgeSpecProtocol               string
}

func ComputeJudgeSpecChecksum(
	input JudgeSpecChecksumInput,
) ([32]byte, error) {
	artifactManifest, err := canonicalJSON(input.ArtifactManifest, true)
	if err != nil {
		return [32]byte{}, ErrInvalidJudgeSpecChecksumInput
	}
	checkerConfig, err := canonicalJSON(input.CheckerConfig, false)
	if err != nil {
		return [32]byte{}, ErrInvalidJudgeSpecChecksumInput
	}
	rewardSnapshot, err := canonicalJSON(input.RewardSnapshot, false)
	if err != nil || !validJudgeSpecChecksumInput(input) {
		return [32]byte{}, ErrInvalidJudgeSpecChecksumInput
	}

	encoder := judgeSpecChecksumEncoder{digest: sha256.New()}
	encoder.string(judgeSpecChecksumDomain)
	encoder.uuid(input.SubmissionID)
	encoder.uuid(input.UserID)
	encoder.uuid(input.ProblemID)
	encoder.int64(input.Generation)
	encoder.uuid(input.ProblemRevisionID)
	encoder.uuid(input.TestsetRevisionID)
	for _, value := range []string{
		input.Language,
		input.RuntimeKey,
		input.RuntimeProfileVersion,
		input.CompilerVersion,
		input.RuntimeBootstrapVersion,
		input.SandboxImageDigest,
		input.CompileCommandID,
		input.RunCommandID,
		input.ArtifactManifestVersion,
		input.ArtifactManifestChecksum,
	} {
		encoder.string(value)
	}
	encoder.bytes(artifactManifest)
	encoder.string(input.SupervisorProtocol)
	encoder.string(input.CheckerKey)
	encoder.string(input.CheckerVersion)
	encoder.bytes(checkerConfig)
	encoder.string(input.ReleaseID)
	encoder.string(input.ApprovedReleaseManifestChecksum)
	encoder.string(input.CheckerImplementationChecksum)
	encoder.string(input.CapabilityProfileHash)
	encoder.bytes(rewardSnapshot)
	encoder.bytes(input.RewardSnapshotChecksum[:])
	for _, value := range []int{
		input.CPUTimeMilliseconds,
		input.WallTimeMilliseconds,
		input.MemoryKilobytes,
		input.OutputBytes,
		input.ProcessCount,
		input.CompileCPUMilliseconds,
		input.CompileWallMilliseconds,
		input.TotalWallMilliseconds,
	} {
		encoder.int64(int64(value))
	}
	encoder.bytes(input.SourceSHA256[:])
	encoder.string(input.JudgeSpecProtocol)
	return encoder.sum(), nil
}

func validJudgeSpecChecksumInput(input JudgeSpecChecksumInput) bool {
	if input.SubmissionID == uuid.Nil ||
		input.UserID == uuid.Nil ||
		input.ProblemID == uuid.Nil ||
		input.Generation < 1 ||
		input.ProblemRevisionID == uuid.Nil ||
		input.TestsetRevisionID == uuid.Nil ||
		input.RewardSnapshotChecksum == ([32]byte{}) ||
		input.SourceSHA256 == ([32]byte{}) ||
		input.CPUTimeMilliseconds < 1 ||
		input.WallTimeMilliseconds < input.CPUTimeMilliseconds ||
		input.MemoryKilobytes < 1 ||
		input.OutputBytes < 1 ||
		input.ProcessCount < 1 ||
		input.CompileCPUMilliseconds < 1 ||
		input.CompileWallMilliseconds < input.CompileCPUMilliseconds ||
		input.TotalWallMilliseconds < input.CompileWallMilliseconds {
		return false
	}
	for _, value := range []string{
		input.Language,
		input.RuntimeKey,
		input.RuntimeProfileVersion,
		input.CompilerVersion,
		input.RuntimeBootstrapVersion,
		input.CompileCommandID,
		input.RunCommandID,
		input.ArtifactManifestVersion,
		input.SupervisorProtocol,
		input.CheckerKey,
		input.CheckerVersion,
		input.ReleaseID,
		input.JudgeSpecProtocol,
	} {
		if !judgeSpecIdentifierPattern.MatchString(value) {
			return false
		}
	}
	for _, digest := range []string{
		input.SandboxImageDigest,
		input.ArtifactManifestChecksum,
		input.ApprovedReleaseManifestChecksum,
		input.CheckerImplementationChecksum,
		input.CapabilityProfileHash,
	} {
		if !judgeSpecDigestPattern.MatchString(digest) {
			return false
		}
	}
	return true
}

func canonicalJSON(
	value json.RawMessage,
	array bool,
) (json.RawMessage, error) {
	var document any
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, ErrInvalidJudgeSpecChecksumInput
	}
	switch typed := document.(type) {
	case []any:
		if !array || len(typed) == 0 {
			return nil, ErrInvalidJudgeSpecChecksumInput
		}
	case map[string]any:
		if array {
			return nil, ErrInvalidJudgeSpecChecksumInput
		}
	default:
		return nil, ErrInvalidJudgeSpecChecksumInput
	}
	canonical, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	return canonical, nil
}

type judgeSpecChecksumEncoder struct {
	digest hash.Hash
}

func (encoder judgeSpecChecksumEncoder) bytes(value []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = encoder.digest.Write(size[:])
	_, _ = encoder.digest.Write(value)
}

func (encoder judgeSpecChecksumEncoder) string(value string) {
	encoder.bytes([]byte(value))
}

func (encoder judgeSpecChecksumEncoder) uuid(value uuid.UUID) {
	encoder.bytes(value[:])
}

func (encoder judgeSpecChecksumEncoder) int64(value int64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(value))
	encoder.bytes(encoded[:])
}

func (encoder judgeSpecChecksumEncoder) sum() [32]byte {
	var result [32]byte
	copy(result[:], encoder.digest.Sum(nil))
	return result
}

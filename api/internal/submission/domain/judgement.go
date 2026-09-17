package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

var ErrInvalidJudgement = errors.New("submission: invalid judgement")

const MaximumJudgeDiagnosticBytes = 16 << 10

// Judgement is the domain form of immutable generation history.
type Judgement struct {
	ID                              uuid.UUID
	SubmissionID                    uuid.UUID
	Generation                      int64
	JudgeSpecID                     uuid.UUID
	ProblemRevisionID               uuid.UUID
	TestsetRevisionID               uuid.UUID
	Verdict                         Verdict
	PassedCount                     int
	TotalCount                      int
	MaximumTimeMilliseconds         *int
	MaximumMemoryKilobytes          *int
	CompileDiagnosticSummary        *string
	JudgeSpecChecksum               [32]byte
	RewardSnapshot                  json.RawMessage
	RewardSnapshotChecksum          [32]byte
	RuntimeProfileVersion           string
	CompilerVersion                 string
	RuntimeBootstrapVersion         string
	SupervisorProtocolVersion       string
	SandboxImageDigest              string
	ArtifactManifestChecksum        string
	ReleaseID                       string
	ApprovedReleaseManifestChecksum string
	CheckerImplementationChecksum   string
	WinningAttemptID                *uuid.UUID
	CompletedAt                     time.Time
	OriginCID                       string
}

func (judgement Judgement) Validate() error {
	if judgement.ID == uuid.Nil || judgement.SubmissionID == uuid.Nil ||
		judgement.Generation < 1 || judgement.JudgeSpecID == uuid.Nil ||
		judgement.ProblemRevisionID == uuid.Nil || judgement.TestsetRevisionID == uuid.Nil ||
		!judgement.Verdict.Valid() || judgement.PassedCount < 0 ||
		judgement.TotalCount < 0 || judgement.PassedCount > judgement.TotalCount {
		return ErrInvalidJudgement
	}
	if judgement.TotalCount > problemcontract.MaximumTestCases {
		return ErrInvalidJudgement
	}
	if judgement.Verdict == VerdictAccepted &&
		(judgement.TotalCount == 0 || judgement.PassedCount != judgement.TotalCount) {
		return ErrInvalidJudgement
	}
	if judgement.Verdict.Contestant() &&
		(judgement.WinningAttemptID == nil || *judgement.WinningAttemptID == uuid.Nil) {
		return ErrInvalidJudgement
	}
	if judgement.WinningAttemptID != nil && *judgement.WinningAttemptID == uuid.Nil {
		return ErrInvalidJudgement
	}
	if judgement.MaximumTimeMilliseconds != nil &&
		(*judgement.MaximumTimeMilliseconds < 0 ||
			*judgement.MaximumTimeMilliseconds > 24*60*60*1_000) {
		return ErrInvalidJudgement
	}
	if judgement.MaximumMemoryKilobytes != nil &&
		(*judgement.MaximumMemoryKilobytes < 0 ||
			*judgement.MaximumMemoryKilobytes > problemcontract.MaximumMemoryLimitKilobytes) {
		return ErrInvalidJudgement
	}
	if judgement.CompileDiagnosticSummary != nil &&
		!validJudgeDiagnostic(*judgement.CompileDiagnosticSummary) {
		return ErrInvalidJudgement
	}
	if judgement.JudgeSpecChecksum == ([32]byte{}) ||
		judgement.RewardSnapshotChecksum == ([32]byte{}) ||
		len(judgement.RewardSnapshot) == 0 || len(judgement.RewardSnapshot) > 64<<10 ||
		!json.Valid(judgement.RewardSnapshot) ||
		!jsonObject(judgement.RewardSnapshot) ||
		!validProvenanceText(judgement.RuntimeProfileVersion, 128) ||
		!validProvenanceText(judgement.CompilerVersion, 128) ||
		!validProvenanceText(judgement.RuntimeBootstrapVersion, 128) ||
		!validProvenanceText(judgement.SupervisorProtocolVersion, 128) ||
		!validSHA256Digest(judgement.SandboxImageDigest) ||
		!validSHA256Digest(judgement.ArtifactManifestChecksum) ||
		!validProvenanceText(judgement.ReleaseID, 128) ||
		!validSHA256Digest(judgement.ApprovedReleaseManifestChecksum) ||
		!validSHA256Digest(judgement.CheckerImplementationChecksum) ||
		judgement.CompletedAt.IsZero() || !validProvenanceText(judgement.OriginCID, 128) {
		return ErrInvalidJudgement
	}
	return nil
}

func (judgement Judgement) BecameActive() bool {
	return judgement.Verdict.Contestant()
}

func jsonObject(value json.RawMessage) bool {
	var object map[string]json.RawMessage
	return json.Unmarshal(value, &object) == nil && object != nil
}

func validSHA256Digest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, character := range value[len("sha256:"):] {
		if !bytes.ContainsRune([]byte("0123456789abcdef"), character) {
			return false
		}
	}
	return true
}

func validProvenanceText(value string, maximumBytes int) bool {
	if strings.TrimSpace(value) == "" || len(value) > maximumBytes || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			return false
		}
	}
	return true
}

func validJudgeDiagnostic(value string) bool {
	if len(value) == 0 || len(value) > MaximumJudgeDiagnosticBytes || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character != '\n' && character != '\r' && character != '\t' &&
			(unicode.IsControl(character) || unicode.In(character, unicode.Cf)) {
			return false
		}
	}
	return true
}

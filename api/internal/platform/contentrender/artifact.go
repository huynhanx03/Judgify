package contentrender

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	rendercontract "github.com/huynhanx03/judgify/internal/platform/contentrender/contract"
)

var ErrArtifactSourceMismatch = errors.New("content render: artifact source identity mismatch")

const (
	StatusReady    = rendercontract.StatusReady
	StatusRejected = rendercontract.StatusRejected

	DiagnosticInvalidUTF8       = rendercontract.DiagnosticInvalidUTF8
	DiagnosticAmbiguousEncoding = rendercontract.DiagnosticAmbiguousEncoding
	DiagnosticSourceTooLarge    = rendercontract.DiagnosticSourceTooLarge
	DiagnosticOutputTooLarge    = rendercontract.DiagnosticOutputTooLarge
	DiagnosticNestingTooDeep    = rendercontract.DiagnosticNestingTooDeep
	DiagnosticRenderRejected    = rendercontract.DiagnosticRenderRejected
)

// Diagnostic contains only a bounded stable code. It never contains source,
// rendered HTML, parser excerpts, URLs, or hidden/user-authored content.
type Diagnostic struct {
	Code string `json:"code"`
}

// Artifact is a pure renderer result. ID remains nil until an append-only
// repository allocates/persists the durable row.
type Artifact struct {
	ID                     uuid.UUID
	RendererVersion        string
	SanitizerPolicyVersion string
	SanitizedHTML          string
	ArtifactChecksum       [32]byte
	Status                 string
	Diagnostics            []Diagnostic
	CreatedBy              *uuid.UUID
	CreatedAt              time.Time
}

// Validate enforces the ready/rejected storage shape independently of Ent.
func (artifact Artifact) Validate() error {
	if !validPipelineVersion(artifact.RendererVersion) || !validPipelineVersion(artifact.SanitizerPolicyVersion) {
		return errors.New("content render: artifact pipeline version is empty")
	}
	if artifact.CreatedBy != nil && *artifact.CreatedBy == uuid.Nil {
		return ErrInvalidCreatedBy
	}
	switch artifact.Status {
	case StatusReady:
		if artifact.ArtifactChecksum == ([32]byte{}) || artifact.Diagnostics == nil || len(artifact.Diagnostics) != 0 ||
			len(artifact.SanitizedHTML) > DefaultMaxRenderedHTMLBytes || !utf8.ValidString(artifact.SanitizedHTML) {
			return errors.New("content render: invalid ready artifact")
		}
	case StatusRejected:
		if artifact.SanitizedHTML != "" || artifact.ArtifactChecksum != ([32]byte{}) ||
			len(artifact.Diagnostics) == 0 || len(artifact.Diagnostics) > DefaultMaxDiagnostics {
			return errors.New("content render: invalid rejected artifact")
		}
		for _, diagnostic := range artifact.Diagnostics {
			if !validDiagnosticCode(diagnostic.Code) {
				return errors.New("content render: unsafe artifact diagnostic")
			}
		}
	default:
		return errors.New("content render: unknown artifact status")
	}
	return nil
}

// ValidateFor cryptographically rebinds a ready artifact to the exact source
// tuple before persistence or pointer selection.
func (artifact Artifact) ValidateFor(source Source) error {
	if err := source.Validate(); err != nil {
		return err
	}
	if err := artifact.Validate(); err != nil {
		return err
	}
	if artifact.Status == StatusRejected {
		return nil
	}
	request := RenderRequest{
		SourceID:       source.ID,
		SourceKind:     source.Kind,
		SourceChecksum: source.Checksum,
	}
	expected := checksumArtifact(
		request,
		artifact.RendererVersion,
		artifact.SanitizerPolicyVersion,
		[]byte(artifact.SanitizedHTML),
	)
	if expected != artifact.ArtifactChecksum {
		return ErrArtifactSourceMismatch
	}
	return nil
}

func validPipelineVersion(value string) bool {
	if len(value) == 0 || len(value) > rendercontract.MaximumPipelineVersionBytes {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func rejectedArtifact(rendererVersion, sanitizerVersion string, maximumDiagnostics int, codes ...string) Artifact {
	if maximumDiagnostics < 1 {
		maximumDiagnostics = 1
	}
	if len(codes) > maximumDiagnostics {
		codes = codes[:maximumDiagnostics]
	}
	diagnostics := make([]Diagnostic, 0, len(codes))
	for _, code := range codes {
		if !validDiagnosticCode(code) {
			code = DiagnosticRenderRejected
		}
		diagnostics = append(diagnostics, Diagnostic{Code: code})
	}
	if len(diagnostics) == 0 {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagnosticRenderRejected})
	}
	return Artifact{
		RendererVersion:        rendererVersion,
		SanitizerPolicyVersion: sanitizerVersion,
		Status:                 StatusRejected,
		Diagnostics:            diagnostics,
	}
}

func validDiagnosticCode(code string) bool {
	return rendercontract.IsDiagnosticCode(code)
}

func checksumArtifact(request RenderRequest, rendererVersion, sanitizerVersion string, sanitizedHTML []byte) [32]byte {
	digest := sha256.New()
	writeChecksumPart(digest, []byte("judgify.content-render-artifact.v1"))
	writeChecksumPart(digest, request.SourceID[:])
	writeChecksumPart(digest, []byte(request.SourceKind))
	writeChecksumPart(digest, request.SourceChecksum[:])
	writeChecksumPart(digest, []byte(rendererVersion))
	writeChecksumPart(digest, []byte(sanitizerVersion))
	writeChecksumPart(digest, sanitizedHTML)
	var checksum [32]byte
	copy(checksum[:], digest.Sum(nil))
	return checksum
}

func writeChecksumPart(digest hash.Hash, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = digest.Write(length[:])
	_, _ = digest.Write(value)
}

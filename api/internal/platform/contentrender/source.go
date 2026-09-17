// Package contentrender owns canonical authored-content normalization and the
// deterministic, sanitized artifact contract shared by all business modules.
package contentrender

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"

	rendercontract "github.com/huynhanx03/judgify/internal/platform/contentrender/contract"
)

const (
	// NormalizationPolicyVersion identifies the exact byte-normalization policy
	// used before source checksums are computed.
	NormalizationPolicyVersion = "utf8-bom-crlf-nfc-v1"

	// DefaultMaxSourceBytes bounds both normalization work and Markdown parsing.
	DefaultMaxSourceBytes = 1 << 20
)

var (
	ErrInvalidSourceID        = errors.New("content render: invalid source id")
	ErrInvalidOwnerRevision   = errors.New("content render: invalid owner revision id")
	ErrInvalidCreatedBy       = errors.New("content render: invalid creator id")
	ErrInvalidSourceKind      = errors.New("content render: invalid source kind")
	ErrInvalidUTF8            = errors.New("content render: source is not valid UTF-8")
	ErrAmbiguousEncoding      = errors.New("content render: source contains an ambiguous encoding marker")
	ErrSourceTooLarge         = errors.New("content render: source exceeds the byte limit")
	ErrSourceNotNormalized    = errors.New("content render: source bytes are not canonical")
	ErrSourceChecksumMismatch = errors.New("content render: source checksum mismatch")
)

// SourceKind is a closed discriminator for one authored-content boundary.
type SourceKind string

const (
	ProblemStatement         SourceKind = rendercontract.SourceKindProblemStatement
	ContestDescription       SourceKind = rendercontract.SourceKindContestDescription
	ContestAnnouncement      SourceKind = rendercontract.SourceKindContestAnnouncement
	ContestClarification     SourceKind = rendercontract.SourceKindContestClarification
	MaterialBody             SourceKind = rendercontract.SourceKindMaterialBody
	NotificationCampaignBody SourceKind = rendercontract.SourceKindNotificationCampaignBody
)

// Source is the immutable parent-registry identity stored independently from
// domain-owned raw revisions. It deliberately does not carry authored bytes.
type Source struct {
	ID              uuid.UUID
	OwnerRevisionID uuid.UUID
	Kind            SourceKind
	Checksum        [32]byte
	CreatedBy       *uuid.UUID
	CreatedAt       time.Time
}

// Validate rejects an incomplete or open-ended registry identity.
func (source Source) Validate() error {
	if source.ID == uuid.Nil {
		return ErrInvalidSourceID
	}
	if source.OwnerRevisionID == uuid.Nil {
		return ErrInvalidOwnerRevision
	}
	if !source.Kind.Valid() {
		return ErrInvalidSourceKind
	}
	if source.Checksum == ([32]byte{}) {
		return errors.New("content render: empty source checksum")
	}
	if source.CreatedBy != nil && *source.CreatedBy == uuid.Nil {
		return ErrInvalidCreatedBy
	}
	return nil
}

// Valid reports whether the kind belongs to the reviewed authored-content
// allowlist. Unknown database/user strings fail closed.
func (kind SourceKind) Valid() bool {
	return rendercontract.IsSourceKind(string(kind))
}

// NormalizedSource contains the exact owned bytes that must be registered and
// later supplied to Render. Bytes never alias the caller's input.
type NormalizedSource struct {
	PolicyVersion string
	Bytes         []byte
	Checksum      [32]byte
}

// RenderRequest binds exact canonical bytes to their registered source tuple.
type RenderRequest struct {
	SourceID           uuid.UUID
	SourceKind         SourceKind
	SourceChecksum     [32]byte
	NormalizedMarkdown []byte
}

// NormalizeMarkdown applies the current versioned policy: reject malformed
// UTF-8/NUL, remove one leading UTF-8 BOM, canonicalize CRLF/CR to LF, and NFC
// normalize Unicode without trimming author-significant whitespace.
func NormalizeMarkdown(input []byte) (NormalizedSource, error) {
	return normalizeMarkdown(input, DefaultMaxSourceBytes)
}

func normalizeMarkdown(input []byte, maximumBytes int) (NormalizedSource, error) {
	if maximumBytes <= 0 {
		return NormalizedSource{}, fmt.Errorf("content render: invalid source byte limit %d", maximumBytes)
	}
	if len(input) > maximumBytes {
		return NormalizedSource{}, ErrSourceTooLarge
	}
	if !utf8.Valid(input) {
		return NormalizedSource{}, ErrInvalidUTF8
	}
	if bytes.IndexByte(input, 0) >= 0 {
		return NormalizedSource{}, ErrAmbiguousEncoding
	}

	canonical := input
	if bytes.HasPrefix(canonical, []byte{0xef, 0xbb, 0xbf}) {
		canonical = canonical[3:]
	}
	canonical = bytes.ReplaceAll(canonical, []byte("\r\n"), []byte("\n"))
	canonical = bytes.ReplaceAll(canonical, []byte("\r"), []byte("\n"))
	canonical = norm.NFC.Bytes(canonical)
	if len(canonical) > maximumBytes {
		return NormalizedSource{}, ErrSourceTooLarge
	}
	canonical = append([]byte(nil), canonical...)

	return NormalizedSource{
		PolicyVersion: NormalizationPolicyVersion,
		Bytes:         canonical,
		Checksum:      sha256.Sum256(canonical),
	}, nil
}

// NewRenderRequest canonicalizes and owns Markdown before constructing the
// exact request that a registry row must match.
func NewRenderRequest(sourceID uuid.UUID, kind SourceKind, markdown []byte) (RenderRequest, error) {
	if sourceID == uuid.Nil {
		return RenderRequest{}, ErrInvalidSourceID
	}
	if !kind.Valid() {
		return RenderRequest{}, ErrInvalidSourceKind
	}
	normalized, err := NormalizeMarkdown(markdown)
	if err != nil {
		return RenderRequest{}, err
	}
	return RenderRequest{
		SourceID:           sourceID,
		SourceKind:         kind,
		SourceChecksum:     normalized.Checksum,
		NormalizedMarkdown: append([]byte(nil), normalized.Bytes...),
	}, nil
}

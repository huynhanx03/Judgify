// Package admincommand contains reusable application contracts for sensitive
// administrative workflows. It owns no domain repositories and grants no
// permission by itself; domain handlers must authorize before issuing or
// verifying a confirmation.
package admincommand

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	confirmationTokenVersion         = "v1"
	confirmationTokenPrefix          = confirmationTokenVersion + "."
	confirmationMACBytes             = sha256.Size
	maximumConfirmationTokenBytes    = 2048
	maximumConfirmationRequestBytes  = 64 << 10
	maximumConfirmationTargetBytes   = 256
	minimumConfirmationTTL           = 30 * time.Second
	maximumConfirmationTTL           = 15 * time.Minute
	confirmationFutureClockTolerance = 30 * time.Second
)

var (
	ErrInvalidConfirmation  = errors.New("admin command: invalid confirmation")
	ErrConfirmationExpired  = errors.New("admin command: confirmation expired")
	ErrConfirmationMismatch = errors.New("admin command: confirmation does not match request")

	confirmationNamePattern   = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,127}$`)
	confirmationTargetPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}$`)
)

// ConfirmationExpectation is the complete public identity of one reviewed
// command. RequestHash must include every mutable domain input, including its
// reason. IdempotencyKey is bound separately so callers cannot accidentally
// omit the durable command identity from their canonical request document.
type ConfirmationExpectation struct {
	ActorID         uuid.UUID
	SessionID       uuid.UUID
	IdempotencyKey  uuid.UUID
	Action          string
	Resource        string
	TargetID        string
	ExpectedVersion int64
	RequestHash     [sha256.Size]byte
}

// ConfirmationClaims are returned only after token authentication, expiry,
// and exact expectation checks have all succeeded.
type ConfirmationClaims struct {
	ConfirmationExpectation
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type confirmationDocument struct {
	Version         string    `json:"v"`
	ActorID         uuid.UUID `json:"actor_id"`
	SessionID       uuid.UUID `json:"session_id"`
	IdempotencyKey  uuid.UUID `json:"idempotency_key"`
	Action          string    `json:"action"`
	Resource        string    `json:"resource"`
	TargetID        string    `json:"target_id"`
	ExpectedVersion int64     `json:"expected_version"`
	RequestHash     string    `json:"request_hash"`
	IssuedAtUnixNS  int64     `json:"issued_at_unix_ns"`
	ExpiresAtUnixNS int64     `json:"expires_at_unix_ns"`
}

// ConfirmationProtector creates short-lived, stateless confirmations. The
// HMAC is domain-separated from every other JWT/CSRF/confirmation use of the
// supplied deployment secret.
type ConfirmationProtector struct {
	key [sha256.Size]byte
	ttl time.Duration
	now func() time.Time
}

func NewConfirmationProtector(
	secret []byte,
	ttl time.Duration,
) (*ConfirmationProtector, error) {
	if len(secret) < sha256.Size ||
		ttl < minimumConfirmationTTL || ttl > maximumConfirmationTTL {
		return nil, ErrInvalidConfirmation
	}
	keyMaterial := make(
		[]byte,
		0,
		len("judgify.admin-command-confirmation.v1\x00")+len(secret),
	)
	keyMaterial = append(
		keyMaterial,
		[]byte("judgify.admin-command-confirmation.v1\x00")...,
	)
	keyMaterial = append(keyMaterial, secret...)
	key := sha256.Sum256(keyMaterial)
	clear(keyMaterial)
	return &ConfirmationProtector{
		key: key,
		ttl: ttl,
		now: func() time.Time { return time.Now().UTC() },
	}, nil
}

func (protector *ConfirmationProtector) Issue(
	expectation ConfirmationExpectation,
) (string, error) {
	token, _, err := protector.IssueWithClaims(expectation)
	return token, err
}

// IssueWithClaims creates a confirmation and returns its authenticated time
// window alongside the opaque token. Callers can show ExpiresAt to a reviewer
// without decoding or otherwise depending on the token representation.
func (protector *ConfirmationProtector) IssueWithClaims(
	expectation ConfirmationExpectation,
) (string, ConfirmationClaims, error) {
	if !protector.valid() || !expectation.valid() {
		return "", ConfirmationClaims{}, ErrInvalidConfirmation
	}
	now := protector.now().UTC()
	if now.IsZero() {
		return "", ConfirmationClaims{}, ErrInvalidConfirmation
	}
	expiresAt := now.Add(protector.ttl)
	document := confirmationDocument{
		Version:         confirmationTokenVersion,
		ActorID:         expectation.ActorID,
		SessionID:       expectation.SessionID,
		IdempotencyKey:  expectation.IdempotencyKey,
		Action:          expectation.Action,
		Resource:        expectation.Resource,
		TargetID:        expectation.TargetID,
		ExpectedVersion: expectation.ExpectedVersion,
		RequestHash:     hex.EncodeToString(expectation.RequestHash[:]),
		IssuedAtUnixNS:  now.UnixNano(),
		ExpiresAtUnixNS: expiresAt.UnixNano(),
	}
	payload, err := json.Marshal(document)
	if err != nil || len(payload) == 0 || len(payload) > maximumConfirmationTokenBytes {
		return "", ConfirmationClaims{}, ErrInvalidConfirmation
	}
	mac := protector.sign(payload)
	token := confirmationTokenPrefix +
		base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(mac)
	if len(token) > maximumConfirmationTokenBytes {
		return "", ConfirmationClaims{}, ErrInvalidConfirmation
	}
	return token, ConfirmationClaims{
		ConfirmationExpectation: expectation,
		IssuedAt:                now,
		ExpiresAt:               expiresAt,
	}, nil
}

func (protector *ConfirmationProtector) Verify(
	token string,
	expectation ConfirmationExpectation,
) (ConfirmationClaims, error) {
	if !protector.valid() || !expectation.valid() || token == "" ||
		len(token) > maximumConfirmationTokenBytes ||
		!strings.HasPrefix(token, confirmationTokenPrefix) {
		return ConfirmationClaims{}, ErrInvalidConfirmation
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != confirmationTokenVersion ||
		parts[1] == "" || parts[2] == "" {
		return ConfirmationClaims{}, ErrInvalidConfirmation
	}
	payload, err := decodeCanonicalBase64(parts[1], maximumConfirmationTokenBytes)
	if err != nil || len(payload) == 0 {
		return ConfirmationClaims{}, ErrInvalidConfirmation
	}
	providedMAC, err := decodeCanonicalBase64(parts[2], confirmationMACBytes)
	if err != nil || len(providedMAC) != confirmationMACBytes ||
		!hmac.Equal(providedMAC, protector.sign(payload)) {
		return ConfirmationClaims{}, ErrInvalidConfirmation
	}
	document, err := decodeConfirmationDocument(payload)
	if err != nil {
		return ConfirmationClaims{}, ErrInvalidConfirmation
	}
	claims, err := document.claims()
	if err != nil {
		return ConfirmationClaims{}, err
	}
	now := protector.now().UTC()
	if !claims.IssuedAt.Before(claims.ExpiresAt) ||
		claims.ExpiresAt.Sub(claims.IssuedAt) != protector.ttl ||
		claims.IssuedAt.After(now.Add(confirmationFutureClockTolerance)) {
		return ConfirmationClaims{}, ErrInvalidConfirmation
	}
	if !now.Before(claims.ExpiresAt) {
		return ConfirmationClaims{}, ErrConfirmationExpired
	}
	if !claims.matches(expectation) {
		return ConfirmationClaims{}, ErrConfirmationMismatch
	}
	return claims, nil
}

func (protector *ConfirmationProtector) valid() bool {
	return protector != nil && protector.key != ([sha256.Size]byte{}) &&
		protector.ttl >= minimumConfirmationTTL &&
		protector.ttl <= maximumConfirmationTTL && protector.now != nil
}

func (protector *ConfirmationProtector) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, protector.key[:])
	_, _ = mac.Write([]byte("confirm\x00"))
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}

func (expectation ConfirmationExpectation) valid() bool {
	return expectation.ActorID != uuid.Nil &&
		expectation.SessionID != uuid.Nil &&
		expectation.IdempotencyKey != uuid.Nil &&
		confirmationNamePattern.MatchString(expectation.Action) &&
		confirmationNamePattern.MatchString(expectation.Resource) &&
		len(expectation.TargetID) <= maximumConfirmationTargetBytes &&
		utf8.ValidString(expectation.TargetID) &&
		confirmationTargetPattern.MatchString(expectation.TargetID) &&
		expectation.ExpectedVersion > 0 &&
		expectation.RequestHash != ([sha256.Size]byte{})
}

func (document confirmationDocument) claims() (ConfirmationClaims, error) {
	requestHash, err := hex.DecodeString(document.RequestHash)
	if err != nil || len(requestHash) != sha256.Size {
		return ConfirmationClaims{}, ErrInvalidConfirmation
	}
	expectation := ConfirmationExpectation{
		ActorID:         document.ActorID,
		SessionID:       document.SessionID,
		IdempotencyKey:  document.IdempotencyKey,
		Action:          document.Action,
		Resource:        document.Resource,
		TargetID:        document.TargetID,
		ExpectedVersion: document.ExpectedVersion,
	}
	copy(expectation.RequestHash[:], requestHash)
	if document.Version != confirmationTokenVersion || !expectation.valid() ||
		document.IssuedAtUnixNS <= 0 || document.ExpiresAtUnixNS <= 0 {
		return ConfirmationClaims{}, ErrInvalidConfirmation
	}
	return ConfirmationClaims{
		ConfirmationExpectation: expectation,
		IssuedAt:                time.Unix(0, document.IssuedAtUnixNS).UTC(),
		ExpiresAt:               time.Unix(0, document.ExpiresAtUnixNS).UTC(),
	}, nil
}

func (claims ConfirmationClaims) matches(
	expectation ConfirmationExpectation,
) bool {
	return claims.ActorID == expectation.ActorID &&
		claims.SessionID == expectation.SessionID &&
		claims.IdempotencyKey == expectation.IdempotencyKey &&
		claims.Action == expectation.Action &&
		claims.Resource == expectation.Resource &&
		claims.TargetID == expectation.TargetID &&
		claims.ExpectedVersion == expectation.ExpectedVersion &&
		subtle.ConstantTimeCompare(
			claims.RequestHash[:],
			expectation.RequestHash[:],
		) == 1
}

func decodeCanonicalBase64(value string, maximum int) ([]byte, error) {
	if value == "" || len(value) > maximum*2 {
		return nil, ErrInvalidConfirmation
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) > maximum ||
		base64.RawURLEncoding.EncodeToString(decoded) != value {
		return nil, ErrInvalidConfirmation
	}
	return decoded, nil
}

func decodeConfirmationDocument(
	payload []byte,
) (confirmationDocument, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var document confirmationDocument
	if err := decoder.Decode(&document); err != nil {
		return confirmationDocument{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return confirmationDocument{}, ErrInvalidConfirmation
	}
	return document, nil
}

// ConfirmationRequestHash computes a bounded, length-delimited hash over one
// caller-owned canonical request document. The domain must be versioned so a
// byte-identical payload cannot authorize another action or schema version.
func ConfirmationRequestHash(
	domain string,
	canonicalDocument []byte,
) ([sha256.Size]byte, error) {
	if !confirmationNamePattern.MatchString(domain) ||
		len(canonicalDocument) == 0 ||
		len(canonicalDocument) > maximumConfirmationRequestBytes {
		return [sha256.Size]byte{}, ErrInvalidConfirmation
	}
	hasher := sha256.New()
	writeConfirmationHashPart(hasher, []byte(
		"judgify.admin-command-request.v1",
	))
	writeConfirmationHashPart(hasher, []byte(domain))
	writeConfirmationHashPart(hasher, canonicalDocument)
	var result [sha256.Size]byte
	copy(result[:], hasher.Sum(nil))
	return result, nil
}

type confirmationHashWriter interface {
	Write([]byte) (int, error)
}

func writeConfirmationHashPart(
	writer confirmationHashWriter,
	value []byte,
) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write(value)
}

package usecase

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/judgify/internal/ports"
)

func TestSecurityEmailPayloadCodecRoundTripsWithoutPlaintextCiphertext(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	codec := newSecurityEmailPayloadCodecForTest(t, "active-v1", map[string][]byte{
		"active-v1": bytes.Repeat([]byte{0x31}, 32),
	}, now, 0x42)
	metadata := securityEmailPayloadMetadataForTest(now.Add(15 * time.Minute))
	want := ports.PasswordResetDeliveryMessage{
		Recipient: "security.user@example.com",
		Token:     "jpr1." + string(bytes.Repeat([]byte{'A'}, 43)),
		ExpiresAt: metadata.ExpiresAt, ProviderIdempotencyKey: metadata.ProviderIdempotencyKey,
	}

	ciphertext, keyID, err := codec.SealPasswordReset(metadata, want)
	require.NoError(t, err)
	require.Equal(t, "active-v1", keyID)
	require.NotContains(t, string(ciphertext), want.Recipient)
	require.NotContains(t, string(ciphertext), want.Token)

	got, err := codec.OpenPasswordReset(metadata, keyID, ciphertext, now)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestSecurityEmailPayloadCodecAuthenticatesMetadataAndCiphertext(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	codec := newSecurityEmailPayloadCodecForTest(t, "active-v1", map[string][]byte{
		"active-v1": bytes.Repeat([]byte{0x31}, 32),
	}, now, 0x43)
	metadata := securityEmailPayloadMetadataForTest(now.Add(15 * time.Minute))
	message := ports.PasswordResetDeliveryMessage{
		Recipient: "security.user@example.com",
		Token:     "jpr1." + string(bytes.Repeat([]byte{'A'}, 43)),
		ExpiresAt: metadata.ExpiresAt, ProviderIdempotencyKey: metadata.ProviderIdempotencyKey,
	}
	ciphertext, keyID, err := codec.SealPasswordReset(metadata, message)
	require.NoError(t, err)

	tampered := append([]byte(nil), ciphertext...)
	tampered[len(tampered)-1] ^= 0xff
	_, err = codec.OpenPasswordReset(metadata, keyID, tampered, now)
	require.ErrorIs(t, err, ErrSecurityEmailPayloadAuthentication)

	changedMetadata := metadata
	changedMetadata.ProviderIdempotencyKey = "security-email/another-delivery"
	_, err = codec.OpenPasswordReset(changedMetadata, keyID, ciphertext, now)
	require.ErrorIs(t, err, ErrSecurityEmailPayloadAuthentication)

	changedMetadata = metadata
	changedMetadata.CredentialID = uuid.New()
	_, err = codec.OpenPasswordReset(changedMetadata, keyID, ciphertext, now)
	require.ErrorIs(t, err, ErrSecurityEmailPayloadAuthentication)
}

func TestSecurityEmailPayloadCodecRoundTripsRecoveryVerification(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	codec := newSecurityEmailPayloadCodecForTest(t, "active-v1", map[string][]byte{
		"active-v1": bytes.Repeat([]byte{0x31}, 32),
	}, now, 0x47)
	metadata := SecurityEmailPayloadMetadata{
		DeliveryID: uuid.New(), CredentialID: uuid.New(),
		Kind:                   ports.SecurityEmailKindRecoveryContactVerification,
		ProviderIdempotencyKey: "security-email/" + uuid.NewString(),
		ExpiresAt:              now.Add(24 * time.Hour),
	}
	want := ports.RecoveryContactVerificationDeliveryMessage{
		Recipient: "verify.user@example.com",
		Token:     "jcv1." + string(bytes.Repeat([]byte{'A'}, 43)),
		ExpiresAt: metadata.ExpiresAt, ProviderIdempotencyKey: metadata.ProviderIdempotencyKey,
	}

	ciphertext, keyID, err := codec.SealRecoveryContactVerification(metadata, want)
	require.NoError(t, err)
	require.NotContains(t, string(ciphertext), want.Recipient)
	require.NotContains(t, string(ciphertext), want.Token)
	got, err := codec.OpenRecoveryContactVerification(metadata, keyID, ciphertext, now)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestSecurityEmailPayloadCodecSupportsKeyRotationAndRejectsUnknownKey(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	oldKey := bytes.Repeat([]byte{0x21}, 32)
	newKey := bytes.Repeat([]byte{0x31}, 32)
	metadata := securityEmailPayloadMetadataForTest(now.Add(15 * time.Minute))
	message := ports.PasswordResetDeliveryMessage{
		Recipient: "security.user@example.com",
		Token:     "jpr1." + string(bytes.Repeat([]byte{'A'}, 43)),
		ExpiresAt: metadata.ExpiresAt, ProviderIdempotencyKey: metadata.ProviderIdempotencyKey,
	}
	oldCodec := newSecurityEmailPayloadCodecForTest(t, "old-v1", map[string][]byte{
		"old-v1": oldKey,
	}, now, 0x44)
	ciphertext, oldKeyID, err := oldCodec.SealPasswordReset(metadata, message)
	require.NoError(t, err)

	rotatedCodec := newSecurityEmailPayloadCodecForTest(t, "new-v2", map[string][]byte{
		"old-v1": oldKey,
		"new-v2": newKey,
	}, now, 0x45)
	got, err := rotatedCodec.OpenPasswordReset(metadata, oldKeyID, ciphertext, now)
	require.NoError(t, err)
	require.Equal(t, message, got)

	_, err = rotatedCodec.OpenPasswordReset(metadata, "retired-key", ciphertext, now)
	require.ErrorIs(t, err, ErrSecurityEmailPayloadUnsupported)
}

func TestSecurityEmailPayloadCodecRejectsExpiredOrOversizedPayload(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	codec := newSecurityEmailPayloadCodecForTest(t, "active-v1", map[string][]byte{
		"active-v1": bytes.Repeat([]byte{0x31}, 32),
	}, now, 0x46)
	metadata := securityEmailPayloadMetadataForTest(now.Add(time.Minute))
	message := ports.PasswordResetDeliveryMessage{
		Recipient: "security.user@example.com",
		Token:     "jpr1." + string(bytes.Repeat([]byte{'A'}, 43)),
		ExpiresAt: metadata.ExpiresAt, ProviderIdempotencyKey: metadata.ProviderIdempotencyKey,
	}
	ciphertext, keyID, err := codec.SealPasswordReset(metadata, message)
	require.NoError(t, err)

	_, err = codec.OpenPasswordReset(metadata, keyID, ciphertext, metadata.ExpiresAt)
	require.ErrorIs(t, err, ErrSecurityEmailPayloadExpired)

	_, err = codec.OpenPasswordReset(metadata, keyID, bytes.Repeat([]byte{0x7f}, 4097), now)
	require.ErrorIs(t, err, ErrSecurityEmailPayloadInvalid)
	require.False(t, errors.Is(err, ErrSecurityEmailPayloadAuthentication))
}

func newSecurityEmailPayloadCodecForTest(
	t *testing.T,
	activeKeyID string,
	keys map[string][]byte,
	now time.Time,
	randomByte byte,
) *SecurityEmailPayloadCodec {
	t.Helper()
	codec, err := NewSecurityEmailPayloadCodec(SecurityEmailPayloadCodecConfig{
		ActiveKeyID:            activeKeyID,
		Keys:                   keys,
		MaximumCiphertextBytes: 4096,
		Clock:                  func() time.Time { return now },
		Random:                 bytes.NewReader(bytes.Repeat([]byte{randomByte}, 4096)),
	})
	require.NoError(t, err)
	return codec
}

func securityEmailPayloadMetadataForTest(expiresAt time.Time) SecurityEmailPayloadMetadata {
	return SecurityEmailPayloadMetadata{
		DeliveryID:             uuid.New(),
		CredentialID:           uuid.New(),
		Kind:                   ports.SecurityEmailKindPasswordReset,
		ProviderIdempotencyKey: "security-email/" + uuid.NewString(),
		ExpiresAt:              expiresAt.UTC(),
	}
}

package usecase

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAndMaskRecoveryEmail(t *testing.T) {
	t.Parallel()

	normalized, err := NormalizeRecoveryEmail("  Jerry.Example@Example.COM  ", 320)
	require.NoError(t, err)
	require.Equal(t, "jerry.example@example.com", normalized)
	masked, err := MaskRecoveryEmail(normalized)
	require.NoError(t, err)
	require.Equal(t, "j***@example.com", masked)

	invalid := []string{
		"", "display <user@example.com>", "user @example.com", "user@@example.com",
		".user@example.com", "user..name@example.com", "user@-example.com",
		"user@example..com", "user@example.com\r\nBcc:test@example.com", "ü@example.com",
	}
	for _, value := range invalid {
		_, err := NormalizeRecoveryEmail(value, 320)
		require.ErrorIs(t, err, ErrInvalidRecoveryEmail, value)
	}
}

func TestRecoveryContactProtectorBindsEveryIdentityDimension(t *testing.T) {
	t.Parallel()

	protector := newTestRecoveryContactProtector(t, "current", map[string][]byte{
		"current": bytes.Repeat([]byte{1}, 32),
	})
	metadata := RecoveryContactMetadata{
		UserID:    uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		ContactID: uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		Kind:      recoveryContactKindEmail, Version: 7,
	}
	ciphertext, keyID, err := protector.Protect(metadata, "User@Example.com")
	require.NoError(t, err)
	require.NotContains(t, string(ciphertext), "user@example.com")
	opened, err := protector.Open(metadata, keyID, ciphertext)
	require.NoError(t, err)
	require.Equal(t, "user@example.com", opened)

	mutations := []RecoveryContactMetadata{
		{UserID: uuid.New(), ContactID: metadata.ContactID, Kind: metadata.Kind, Version: metadata.Version},
		{UserID: metadata.UserID, ContactID: uuid.New(), Kind: metadata.Kind, Version: metadata.Version},
		{UserID: metadata.UserID, ContactID: metadata.ContactID, Kind: metadata.Kind, Version: metadata.Version + 1},
	}
	for _, mutated := range mutations {
		_, err := protector.Open(mutated, keyID, ciphertext)
		require.ErrorIs(t, err, ErrRecoveryContactAuthentication)
	}

	tampered := append([]byte(nil), ciphertext...)
	tampered[len(tampered)-1] ^= 1
	_, err = protector.Open(metadata, keyID, tampered)
	require.ErrorIs(t, err, ErrRecoveryContactAuthentication)
}

func TestRecoveryContactProtectorSupportsBoundedKeyRotation(t *testing.T) {
	t.Parallel()

	oldKey := bytes.Repeat([]byte{2}, 32)
	newKey := bytes.Repeat([]byte{3}, 32)
	metadata := RecoveryContactMetadata{
		UserID: uuid.New(), ContactID: uuid.New(), Kind: recoveryContactKindEmail, Version: 1,
	}
	oldProtector := newTestRecoveryContactProtector(t, "old", map[string][]byte{"old": oldKey})
	ciphertext, oldKeyID, err := oldProtector.Protect(metadata, "rotate@example.com")
	require.NoError(t, err)

	rotated := newTestRecoveryContactProtector(t, "new", map[string][]byte{
		"old": oldKey,
		"new": newKey,
	})
	opened, err := rotated.Open(metadata, oldKeyID, ciphertext)
	require.NoError(t, err)
	require.Equal(t, "rotate@example.com", opened)

	lookup, fingerprint, activeKeyID, err := rotated.ActiveHashes("rotate@example.com")
	require.NoError(t, err)
	require.Equal(t, "new", activeKeyID)
	require.Len(t, lookup, 32)
	require.Len(t, fingerprint, 32)
	require.False(t, bytes.Equal(lookup, fingerprint))
	oldLookup, err := rotated.LookupHash("old", "rotate@example.com")
	require.NoError(t, err)
	require.False(t, bytes.Equal(lookup, oldLookup))
}

func TestRecoveryContactProtectorKeepsLiveOneTimeTokensValidAcrossKeyRotation(t *testing.T) {
	t.Parallel()

	oldKey := bytes.Repeat([]byte{4}, 32)
	newKey := bytes.Repeat([]byte{5}, 32)
	oldProtector := newTestRecoveryContactProtector(t, "old", map[string][]byte{"old": oldKey})
	raw, stored, err := oldProtector.NewPasswordResetToken()
	require.NoError(t, err)
	require.Equal(t, "old", stored.KeyID)
	require.Len(t, stored.Hash, 32)
	require.NotContains(t, string(stored.Hash), raw)

	rotated := newTestRecoveryContactProtector(t, "new", map[string][]byte{
		"old": oldKey,
		"new": newKey,
	})
	candidates, err := rotated.PasswordResetTokenCandidates(raw, 256)
	require.NoError(t, err)
	require.Len(t, candidates, 2)
	require.Contains(t, candidates, stored)

	verificationCandidates, err := rotated.VerificationTokenCandidates(
		strings.Replace(raw, passwordResetTokenPrefix, recoveryVerificationTokenPrefix, 1),
		256,
	)
	require.NoError(t, err)
	require.Len(t, verificationCandidates, 2)
	for _, candidate := range verificationCandidates {
		if candidate.KeyID == stored.KeyID {
			require.False(t, bytes.Equal(candidate.Hash, stored.Hash))
		}
	}
}

func TestRecoveryContactProtectorRejectsMalformedOneTimeTokens(t *testing.T) {
	t.Parallel()

	protector := newTestRecoveryContactProtector(t, "active", map[string][]byte{
		"active": bytes.Repeat([]byte{6}, 32),
	})
	for _, raw := range []string{
		"", "jpr1.short", "jcv1.short",
		"jpr1." + strings.Repeat("+", 43),
		"jcv1." + strings.Repeat("A", 42) + " ",
	} {
		_, err := protector.PasswordResetTokenCandidates(raw, 256)
		require.Error(t, err, raw)
		_, err = protector.VerificationTokenCandidates(raw, 256)
		require.Error(t, err, raw)
	}
}

func TestRecoveryContactProtectorRejectsInvalidConfigurationWithoutSecretErrors(t *testing.T) {
	t.Parallel()

	secretText := strings.Repeat("sensitive", 4)
	_, err := NewRecoveryContactProtector(RecoveryContactProtectorConfig{
		ActiveKeyID: "missing",
		Keys:        map[string][]byte{"other": []byte(secretText)},
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidRecoveryContactProtector))
	require.NotContains(t, err.Error(), secretText)
}

func FuzzNormalizeRecoveryEmailNeverPanics(f *testing.F) {
	for _, seed := range []string{
		"user@example.com", "", "a@b.co", "ü@example.com", "a\x00b@example.com",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		normalized, err := NormalizeRecoveryEmail(value, 320)
		if err != nil {
			return
		}
		require.Equal(t, strings.ToLower(strings.TrimSpace(value)), normalized)
		require.NotContains(t, normalized, "\r")
		require.NotContains(t, normalized, "\n")
	})
}

func newTestRecoveryContactProtector(
	t *testing.T,
	activeKeyID string,
	keys map[string][]byte,
) *RecoveryContactProtector {
	t.Helper()
	protector, err := NewRecoveryContactProtector(RecoveryContactProtectorConfig{
		ActiveKeyID: activeKeyID,
		Keys:        keys,
		Random:      bytes.NewReader(bytes.Repeat([]byte{9}, 4096)),
	})
	require.NoError(t, err)
	return protector
}

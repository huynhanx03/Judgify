package onboarding

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateRegistrationRecoveryWindow(t *testing.T) {
	require.NoError(t, ValidateRegistrationRecoveryWindow(10*time.Minute, 12*time.Minute))

	for _, test := range []struct {
		name        string
		offerTTL    time.Duration
		recoveryTTL time.Duration
	}{
		{name: "equal", offerTTL: 10 * time.Minute, recoveryTTL: 10 * time.Minute},
		{name: "shorter", offerTTL: 10 * time.Minute, recoveryTTL: 5 * time.Minute},
		{name: "missing offer ttl", recoveryTTL: 12 * time.Minute},
		{name: "missing recovery ttl", offerTTL: 10 * time.Minute},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateRegistrationRecoveryWindow(test.offerTTL, test.recoveryTTL)
			require.ErrorIs(t, err, ErrInvalidRegistrationRecoveryWindow)
			require.Contains(t, err.Error(), "recovery receipt TTL must exceed trait offer TTL")
		})
	}
}

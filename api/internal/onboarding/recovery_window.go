package onboarding

import (
	"errors"
	"fmt"
	"time"
)

var ErrInvalidRegistrationRecoveryWindow = errors.New("onboarding: invalid registration recovery window")

// ValidateRegistrationRecoveryWindow guarantees that an exact idempotent
// retry can recover credentials after its durable trait offer expires.
func ValidateRegistrationRecoveryWindow(offerTTL, recoveryReceiptTTL time.Duration) error {
	if offerTTL <= 0 || recoveryReceiptTTL <= offerTTL {
		return fmt.Errorf(
			"onboarding: recovery receipt TTL must exceed trait offer TTL (recovery=%s, offer=%s): %w",
			recoveryReceiptTTL,
			offerTTL,
			ErrInvalidRegistrationRecoveryWindow,
		)
	}
	return nil
}

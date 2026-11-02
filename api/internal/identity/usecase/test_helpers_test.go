package usecase

import (
	"context"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

// testHashPassword produces a bcrypt hash suitable for use in test credentials.
// Uses cost 10, the minimum accepted by password.Hasher.Verify.
func testHashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		t.Fatalf("testHashPassword: %v", err)
	}
	return string(hash)
}

type securityMutationJournalCapture struct {
	mu        sync.Mutex
	mutations []entity.SecurityMutation
	err       error
}

func (capture *securityMutationJournalCapture) Append(
	_ context.Context,
	mutation entity.SecurityMutation,
) error {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if capture.err != nil {
		return capture.err
	}
	capture.mutations = append(capture.mutations, mutation)
	return nil
}

func (capture *securityMutationJournalCapture) snapshot() []entity.SecurityMutation {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return append([]entity.SecurityMutation(nil), capture.mutations...)
}

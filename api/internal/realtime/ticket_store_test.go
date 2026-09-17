package realtime

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestTicketIsOpaqueSingleUseAndBoundedByAuthentication(t *testing.T) {
	now := time.Date(2026, time.July, 19, 9, 0, 0, 0, time.UTC)
	principal := ticketPrincipal(now, 10*time.Second)
	store := testTicketStore(t, TicketConfig{TTL: 30 * time.Second}, func() time.Time { return now }, entropy(1, 1))

	issued, err := store.Issue(context.Background(), principal)
	require.NoError(t, err)
	require.Len(t, issued.Ticket, 43)
	require.Equal(t, principal.AuthenticatedUntil, issued.ExpiresAt)
	require.Len(t, store.tickets, 1)

	resolved, err := store.Consume(context.Background(), issued.Ticket)
	require.NoError(t, err)
	require.Equal(t, principal, resolved)
	require.Empty(t, store.tickets)
	require.Empty(t, store.byPrincipal)

	_, err = store.Consume(context.Background(), issued.Ticket)
	require.ErrorIs(t, err, ErrTicketUnavailable)
}

func TestTicketConcurrentConsumeHasExactlyOneWinner(t *testing.T) {
	now := time.Date(2026, time.July, 19, 9, 0, 0, 0, time.UTC)
	store := testTicketStore(t, TicketConfig{}, func() time.Time { return now }, entropy(1, 1))
	issued, err := store.Issue(context.Background(), ticketPrincipal(now, time.Minute))
	require.NoError(t, err)

	var successes atomic.Int32
	var wait sync.WaitGroup
	for range 128 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, consumeErr := store.Consume(context.Background(), issued.Ticket); consumeErr == nil {
				successes.Add(1)
			} else if !errors.Is(consumeErr, ErrTicketUnavailable) {
				t.Errorf("unexpected consume error: %v", consumeErr)
			}
		}()
	}
	wait.Wait()
	require.Equal(t, int32(1), successes.Load())
}

func TestTicketCapacityAndCleanupArePerPrincipalAndGlobal(t *testing.T) {
	now := time.Date(2026, time.July, 19, 9, 0, 0, 0, time.UTC)
	current := now
	store := testTicketStore(t, TicketConfig{
		TTL: 5 * time.Second, Capacity: 2, MaxTicketsPerPrincipal: 1,
	}, func() time.Time { return current }, entropy(4, 4))
	first := ticketPrincipal(now, time.Minute)
	second := ticketPrincipal(now, time.Minute)
	second.UserID = uuid.MustParse("00000000-0000-7000-8000-000000000002").String()
	second.Subject = second.UserID

	_, err := store.Issue(context.Background(), first)
	require.NoError(t, err)
	_, err = store.Issue(context.Background(), first)
	require.ErrorIs(t, err, ErrTicketCapacity)
	_, err = store.Issue(context.Background(), second)
	require.NoError(t, err)

	current = now.Add(5 * time.Second)
	require.Equal(t, 2, store.cleanupExpired(current))
	require.Empty(t, store.tickets)
	_, err = store.Issue(context.Background(), first)
	require.NoError(t, err)
}

func TestExpiredTicketIsRemovedBeforeReturn(t *testing.T) {
	now := time.Date(2026, time.July, 19, 9, 0, 0, 0, time.UTC)
	current := now
	store := testTicketStore(t, TicketConfig{TTL: time.Second}, func() time.Time { return current }, entropy(1, 1))
	issued, err := store.Issue(context.Background(), ticketPrincipal(now, time.Minute))
	require.NoError(t, err)
	current = now.Add(time.Second)

	_, err = store.Consume(context.Background(), issued.Ticket)
	require.ErrorIs(t, err, ErrTicketUnavailable)
	require.Empty(t, store.tickets)
}

func TestTicketRejectsAmbiguousPrincipalAndClosesCleanly(t *testing.T) {
	now := time.Date(2026, time.July, 19, 9, 0, 0, 0, time.UTC)
	store := testTicketStore(t, TicketConfig{}, func() time.Time { return now }, entropy(1, 1))
	principal := ticketPrincipal(now, time.Minute)
	principal.Subject = "user:" + principal.UserID
	_, err := store.Issue(context.Background(), principal)
	require.ErrorIs(t, err, ErrInvalidPrincipal)

	runContext, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- store.Run(runContext) }()
	require.Eventually(t, func() bool {
		store.life.mu.Lock()
		defer store.life.mu.Unlock()
		return store.life.state == componentRunning
	}, time.Second, time.Millisecond)
	cancel()
	require.NoError(t, <-runDone)
	require.NoError(t, store.Shutdown(context.Background()))
	_, err = store.Issue(context.Background(), ticketPrincipal(now, time.Minute))
	require.ErrorIs(t, err, ErrTicketStoreClosed)
}

func TestPrincipalResolverSupportsAnonymousAndConsumesOnlyOneTicket(t *testing.T) {
	now := time.Date(2026, time.July, 19, 9, 0, 0, 0, time.UTC)
	store := testTicketStore(t, TicketConfig{}, func() time.Time { return now }, entropy(1, 1))

	anonymousRequest := httptest.NewRequest("GET", "http://example.test/ws", nil)
	principal, err := store.PrincipalResolver(context.Background(), anonymousRequest)
	require.NoError(t, err)
	require.False(t, principal.Authenticated)

	expected := ticketPrincipal(now, time.Minute)
	issued, err := store.Issue(context.Background(), expected)
	require.NoError(t, err)
	request := httptest.NewRequest("GET", "http://example.test/ws?ticket="+issued.Ticket, nil)
	principal, err = store.PrincipalResolver(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, expected, principal)

	_, err = store.PrincipalResolver(context.Background(), request)
	require.ErrorIs(t, err, ErrTicketUnavailable)
	multiple := httptest.NewRequest("GET", "http://example.test/ws?ticket=a&ticket=b", nil)
	_, err = store.PrincipalResolver(context.Background(), multiple)
	require.ErrorIs(t, err, ErrTicketUnavailable)
}

func TestIssueFromContextRequiresCanonicalSessionPrincipal(t *testing.T) {
	now := time.Date(2026, time.July, 19, 9, 0, 0, 0, time.UTC)
	store := testTicketStore(t, TicketConfig{}, func() time.Time { return now }, entropy(1, 1))
	_, err := store.IssueFromContext(context.Background())
	require.ErrorIs(t, err, ErrInvalidPrincipal)

	principal := ticketPrincipal(now, time.Minute)
	ctx := authentication.WithPrincipal(context.Background(), principal)
	issued, err := store.IssueFromContext(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, issued.Ticket)
}

func testTicketStore(
	t *testing.T,
	config TicketConfig,
	now func() time.Time,
	random *bytes.Reader,
) *TicketStore {
	t.Helper()
	store, err := newTicketStore(config, zap.NewNop(), now, random)
	require.NoError(t, err)
	return store
}

func ticketPrincipal(now time.Time, lifetime time.Duration) authentication.Principal {
	userID := uuid.MustParse("00000000-0000-7000-8000-000000000001").String()
	return authentication.Principal{
		Authenticated:      true,
		Subject:            userID,
		UserID:             userID,
		SessionID:          uuid.MustParse("00000000-0000-7000-8000-000000000010").String(),
		TokenID:            uuid.MustParse("00000000-0000-7000-8000-000000000011").String(),
		AuthenticatedAt:    now.Add(-time.Minute),
		SecurityVersion:    1,
		AuthenticatedUntil: now.Add(lifetime),
	}
}

func entropy(tickets, seed int) *bytes.Reader {
	data := make([]byte, tickets*ticketEntropyBytes)
	for index := range data {
		data[index] = byte(seed + index)
	}
	return bytes.NewReader(data)
}

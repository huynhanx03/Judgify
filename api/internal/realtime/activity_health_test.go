package realtime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestActivityHealthRequiresRecentSuccessAndRecovers(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 22, 9, 0, 0, 0, time.UTC)
	health, err := newActivityHealth(func() time.Time { return now })
	require.NoError(t, err)

	require.False(t, health.available(10*time.Second))

	health.succeeded()
	require.True(t, health.available(10*time.Second))

	health.failed()
	now = now.Add(10*time.Second - time.Nanosecond)
	require.True(t, health.available(10*time.Second), "a bounded transient failure must not drop readiness")

	now = now.Add(time.Nanosecond)
	require.False(t, health.available(10*time.Second), "an unresolved stale success must drop readiness")

	health.succeeded()
	require.True(t, health.available(10*time.Second), "a successful operation must restore readiness")
}

package testutil

import (
	"fmt"

	"github.com/google/uuid"
)

// ID returns a deterministic UUID derived from n, so tests can use stable,
// readable identifiers (testutil.ID(1), testutil.ID(2), …) in place of the old
// integer IDs while keeping equality comparisons meaningful.
func ID(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012d", n))
}

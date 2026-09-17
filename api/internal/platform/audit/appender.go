package audit

import (
	"context"

	"github.com/google/uuid"
)

// Appender atomically appends a redacted audit record to the UnitOfWork bound
// to ctx. It never opens or commits a transaction itself.
type Appender interface {
	Append(ctx context.Context, entry Entry) (uuid.UUID, error)
}

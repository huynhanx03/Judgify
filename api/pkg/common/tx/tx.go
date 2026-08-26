package tx

import "context"

// Manager abstracts transaction management so services can be tested
// without a real database connection.
type Manager interface {
	DoInTx(ctx context.Context, fn func(ctx context.Context) error) error
}

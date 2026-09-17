package operation

import (
	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

// Components is the transport-neutral operation composition surface. Domain
// modules receive Starter and register executors; API infrastructure adapts
// Repository and Controller to HTTP without coupling workers to Gin.
type Components struct {
	Repository   *EntRepository
	Transactions platformtx.UnitOfWork
	Starter      *Starter
	Controller   *Controller
}

func NewComponents(client *dbent.EntClient) (*Components, error) {
	repository, err := NewEntRepository(client)
	if err != nil {
		return nil, err
	}
	transactions := platformtx.NewEntUnitOfWork(client.Client())
	auditRepository := audit.NewRepository()
	starter, err := NewStarter(repository, transactions, auditRepository)
	if err != nil {
		return nil, err
	}
	controller, err := NewController(repository, transactions, auditRepository)
	if err != nil {
		return nil, err
	}
	return &Components{
		Repository:   repository,
		Transactions: transactions,
		Starter:      starter,
		Controller:   controller,
	}, nil
}

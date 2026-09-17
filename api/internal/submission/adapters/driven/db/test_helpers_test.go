package db

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

func newIntakeMockClient(t *testing.T) (*generate.Client, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("open sql mock: %v", err)
	}
	client := generate.NewClient(
		generate.Driver(entsql.OpenDB(dialect.Postgres, database)),
	)
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client, mock
}

func wrapSubmissionClient(client *generate.Client) *internalent.EntClient {
	return internalent.WrapClient(client, zap.NewNop())
}

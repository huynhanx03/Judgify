package migrate

import (
	"testing"

	"entgo.io/ent/dialect/sql/schema"
)

func TestSessionCommandTargetForeignKeyRetainsReceiptTarget(t *testing.T) {
	t.Parallel()

	const symbol = "auth_session_command_receipts_auth_sessions_targeted_commands"
	for _, foreignKey := range AuthSessionCommandReceiptsTable.ForeignKeys {
		if foreignKey.Symbol != symbol {
			continue
		}
		if foreignKey.OnDelete != schema.NoAction {
			t.Fatalf("foreign key %q OnDelete = %v, want %v", symbol, foreignKey.OnDelete, schema.NoAction)
		}
		return
	}
	t.Fatalf("foreign key %q is missing from generated table", symbol)
}

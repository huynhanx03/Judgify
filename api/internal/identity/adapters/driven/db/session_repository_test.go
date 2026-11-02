package db

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/authrefreshtoken"
	"github.com/huynhanx03/judgify/internal/ent/generate/authsession"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

func TestAccountWideRevocationJoinsTransactionAndLocksUserSessionsThenTokens(t *testing.T) {
	client, mock := newReceiptCleanupMockClient(t)
	wrapper := dbent.WrapClient(client, zap.NewNop())
	manager := dbent.NewTxManager(wrapper)
	sessions := NewSessionRepository(wrapper)
	refreshTokens := NewRefreshTokenRepository(wrapper)
	userID := uuid.New()
	now := time.Date(2026, 7, 26, 11, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	// The account-wide fence must lock the canonical physical row even after a
	// concurrent transaction has soft-deleted it. The final UserRepository
	// delete remains the authority that maps that race to a user-facing 404.
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT "users"."id" FROM "users" WHERE "users"."id" = $1 LIMIT 2 FOR UPDATE`,
	)).
		WillReturnRows(sqlmock.NewRows([]string{user.FieldID}).AddRow(userID))
	mock.ExpectExec(`UPDATE "auth_sessions" SET .*"status".*"revoked_at".*"revoke_reason".* WHERE .*"user_id".*"status".*`).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`UPDATE "auth_refresh_tokens" SET .*"status".*"revoked_at".* WHERE .*`).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	err := manager.DoInTx(context.Background(), func(txCtx context.Context) error {
		return manager.DoInTx(txCtx, func(nestedCtx context.Context) error {
			if revokeErr := sessions.RevokeAll(nestedCtx, userID, "account_deleted", now); revokeErr != nil {
				return revokeErr
			}
			return refreshTokens.RevokeIssuedForUser(nestedCtx, userID, now)
		})
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionAndRefreshRepositoriesResolveTransactionAwareClient(t *testing.T) {
	tests := []struct {
		file    string
		methods []string
	}{
		{"session.go", []string{"Create", "GetActiveForAuthentication", "TouchLastSeen", "Revoke", "RevokeAll"}},
		{"refresh_token.go", []string{"Create", "Get", "GetByHash", "RevokeIssuedForSession", "RevokeIssuedForUser"}},
	}
	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Clean(test.file), nil, 0)
			require.NoError(t, err)
			found := make(map[string]bool)
			for _, declaration := range parsed.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok || function.Recv == nil || function.Body == nil {
					continue
				}
				for _, expected := range test.methods {
					if function.Name.Name != expected {
						continue
					}
					found[expected] = callsContextDatabase(function.Body)
				}
			}
			for _, method := range test.methods {
				require.Truef(t, found[method], "%s.%s must resolve repository.client.DB(ctx)", test.file, method)
			}
		})
	}
}

func TestSecurityRepositoryMappingsOwnSensitiveBytes(t *testing.T) {
	now := time.Now().UTC()
	hash := make([]byte, refreshTokenHashBytes)
	hash[0] = 1
	refreshRecord := &generate.AuthRefreshToken{
		ID:        uuid.New(),
		SessionID: uuid.New(),
		TokenHash: hash,
		Status:    authrefreshtoken.StatusIssued,
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
	}
	refreshEntity := refreshTokenEntity(refreshRecord)
	require.Equal(t, byte(1), refreshEntity.TokenHash[0])
	hash[0] = 2
	require.Equal(t, byte(1), refreshEntity.TokenHash[0])
	refreshEntity.TokenHash[0] = 3
	require.Equal(t, byte(2), refreshRecord.TokenHash[0])

	ipHash := make([]byte, sessionIPHashBytes)
	ipHash[0] = 4
	sessionRecord := &generate.AuthSession{
		ID:                    uuid.New(),
		UserID:                uuid.New(),
		FamilyID:              uuid.New(),
		Status:                authsession.StatusActive,
		UserSecurityVersion:   1,
		LastSeenAt:            now,
		LastReauthenticatedAt: now,
		IdleExpiresAt:         now.Add(time.Hour),
		AbsoluteExpiresAt:     now.Add(2 * time.Hour),
		IPHash:                &ipHash,
	}
	sessionEntity := &entity.Session{}
	mapSession(sessionRecord, sessionEntity)
	ipHash[0] = 5
	require.Equal(t, byte(4), sessionEntity.IPHash[0])
	sessionEntity.IPHash[0] = 6
	require.Equal(t, byte(5), (*sessionRecord.IPHash)[0])
}

func TestSecurityRepositoryCreateValidation(t *testing.T) {
	now := time.Now().UTC()
	validSession := &entity.Session{
		ID:                    uuid.New(),
		UserID:                uuid.New(),
		FamilyID:              uuid.New(),
		Status:                entity.SessionStatusActive,
		UserSecurityVersion:   1,
		LastSeenAt:            now,
		LastReauthenticatedAt: now,
		IdleExpiresAt:         now.Add(time.Hour),
		AbsoluteExpiresAt:     now.Add(2 * time.Hour),
		CreatedCID:            "test-cid",
	}
	require.NoError(t, validateSessionForCreate(validSession))
	invalidSession := *validSession
	invalidSession.Status = entity.SessionStatusRevoked
	require.Error(t, validateSessionForCreate(&invalidSession))
	invalidSession = *validSession
	unsafeUserAgent := "browser\nforged-log-line"
	invalidSession.UserAgent = &unsafeUserAgent
	require.Error(t, validateSessionForCreate(&invalidSession))
	unsafeUserAgent = "browser\u202eexe.txt"
	require.Error(t, validateSessionForCreate(&invalidSession))
	unsafeUserAgent = string([]byte{'b', 'a', 'd', 0xff})
	require.Error(t, validateSessionForCreate(&invalidSession))
	_, err := normalizeRevokeReason("logout\rforged-log-line")
	require.Error(t, err)
	_, err = normalizeRevokeReason("logout\u2066spoof")
	require.Error(t, err)

	validRefresh := &entity.RefreshToken{
		ID:        uuid.New(),
		SessionID: uuid.New(),
		TokenHash: make([]byte, refreshTokenHashBytes),
		Status:    entity.RefreshTokenStatusIssued,
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
	}
	require.NoError(t, validateRefreshTokenForCreate(validRefresh))
	invalidRefresh := *validRefresh
	invalidRefresh.TokenHash = []byte("plain-or-truncated")
	require.Error(t, validateRefreshTokenForCreate(&invalidRefresh))
}

func callsContextDatabase(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "DB" {
			return true
		}
		argument, ok := call.Args[0].(*ast.Ident)
		if !ok || argument.Name != "ctx" {
			return true
		}
		client, ok := selector.X.(*ast.SelectorExpr)
		if !ok || client.Sel.Name != "client" {
			return true
		}
		receiver, ok := client.X.(*ast.Ident)
		found = ok && receiver.Name == "repository"
		return !found
	})
	return found
}

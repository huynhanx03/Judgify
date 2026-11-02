//go:build integration

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	_ "github.com/huynhanx03/judgify/internal/ent/generate/runtime"
	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
	"github.com/huynhanx03/judgify/internal/testutil/dbtest"
)

func TestProblemRepositoryPostgresRejectsSequentialDuplicateSlug(t *testing.T) {
	ctx, database, repository, authorID, difficultyID := openProblemPostgresHarness(
		t,
		"sequential",
		70_001,
	)

	first := newPostgresProblem(t, "stable-public-slug", authorID, difficultyID)
	require.NoError(t, repository.Create(ctx, first))

	duplicate := newPostgresProblem(t, "stable-public-slug", authorID, difficultyID)
	err := repository.Create(ctx, duplicate)
	require.ErrorIs(t, err, ports.ErrProblemSlugConflict)
	requirePostgresProblemSlugCount(t, ctx, database, "stable-public-slug", 1)
}

func TestProblemRepositoryPostgresSerializesConcurrentCanonicalEquivalentSlug(t *testing.T) {
	ctx, database, repository, authorID, difficultyID := openProblemPostgresHarness(
		t,
		"concurrent",
		70_002,
	)

	candidates := []string{"  Graph__Traversal  ", "graph-traversal"}
	start := make(chan struct{})
	results := make(chan error, len(candidates))
	for _, candidate := range candidates {
		problem := newPostgresProblem(t, candidate, authorID, difficultyID)
		require.Equal(t, "graph-traversal", problem.Slug)
		go func() {
			<-start
			results <- repository.Create(ctx, problem)
		}()
	}

	close(start)
	var successes, conflicts int
	for range candidates {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ports.ErrProblemSlugConflict):
			conflicts++
		default:
			t.Fatalf("concurrent problem create error = %v", err)
		}
	}

	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	requirePostgresProblemSlugCount(t, ctx, database, "graph-traversal", 1)
}

func TestProblemRepositoryPostgresDoesNotReuseSlugAfterSoftDelete(t *testing.T) {
	ctx, database, repository, authorID, difficultyID := openProblemPostgresHarness(
		t,
		"soft-delete",
		70_003,
	)

	original := newPostgresProblem(t, "retired-public-slug", authorID, difficultyID)
	require.NoError(t, repository.Create(ctx, original))
	require.NoError(
		t,
		repository.client.DB(ctx).Problem.DeleteOneID(original.ID).Exec(ctx),
	)

	replacement := newPostgresProblem(t, "retired-public-slug", authorID, difficultyID)
	err := repository.Create(ctx, replacement)
	require.ErrorIs(t, err, ports.ErrProblemSlugConflict)

	var total, softDeleted int
	err = database.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE deleted_at IS NOT NULL)
		FROM public.problems
		WHERE slug = $1
	`, "retired-public-slug").Scan(&total, &softDeleted)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, 1, softDeleted)
}

func openProblemPostgresHarness(
	t *testing.T,
	scenario string,
	difficultyLevel int,
) (
	context.Context,
	*sql.DB,
	*ProblemRepository,
	uuid.UUID,
	uuid.UUID,
) {
	t.Helper()

	dsn := dbtest.RequireDatabaseURL(t)

	database, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	database.SetMaxOpenConns(4)
	database.SetMaxIdleConns(4)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	require.NoError(t, database.PingContext(ctx))

	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	repository, ok := NewProblemRepository(dbent.WrapClient(client, nil)).(*ProblemRepository)
	require.True(t, ok)

	authorID := uuid.New()
	difficultyID := uuid.New()
	createdAt := time.Now().UTC()
	_, err = database.ExecContext(ctx, `
		INSERT INTO public.users
			(id, created_at, updated_at, profile_visibility_updated_at, username, status, security_version)
		VALUES ($1, $2, $2, $2, $3, 'active', 1)
	`, authorID, createdAt, fmt.Sprintf("slug-%s-%s", scenario, authorID.String()[:8]))
	require.NoError(t, err)
	_, err = database.ExecContext(ctx, `
		INSERT INTO public.difficulties
			(id, created_at, updated_at, name, level, exp_reward)
		VALUES ($1, $2, $2, $3, $4, 100)
	`, difficultyID, createdAt, fmt.Sprintf("Slug %s %s", scenario, difficultyID.String()[:8]), difficultyLevel)
	require.NoError(t, err)

	return ctx, database, repository, authorID, difficultyID
}

func newPostgresProblem(
	t *testing.T,
	slugCandidate string,
	authorID uuid.UUID,
	difficultyID uuid.UUID,
) *entity.Problem {
	t.Helper()

	slug, err := entity.CanonicalProblemSlug(slugCandidate)
	require.NoError(t, err)
	return &entity.Problem{
		Slug:          slug,
		Title:         "PostgreSQL slug contract",
		Description:   "Verifies stable public slug ownership.",
		DifficultyID:  difficultyID,
		TimeLimitMs:   1_000,
		MemoryLimitKb: 256 << 10,
		AuthorID:      authorID,
	}
}

func requirePostgresProblemSlugCount(
	t *testing.T,
	ctx context.Context,
	database *sql.DB,
	slug string,
	want int,
) {
	t.Helper()

	var count int
	require.NoError(
		t,
		database.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM public.problems WHERE slug = $1`,
			slug,
		).Scan(&count),
	)
	require.Equal(t, want, count)
}

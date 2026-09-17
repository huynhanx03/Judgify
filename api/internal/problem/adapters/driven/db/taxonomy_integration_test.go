//go:build integration

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"

	cultivationdb "github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db"
	cultivationentity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	_ "github.com/huynhanx03/judgify/internal/ent/generate/runtime"
	"github.com/huynhanx03/judgify/internal/ports"
	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
	"github.com/huynhanx03/judgify/internal/testutil/dbtest"
)

func TestTaxonomyRepositoryPostgresWritesImmutableHistoryAndRejectsMalformedFacts(t *testing.T) {
	ctx, database, client := openTaxonomyPostgres(t)
	difficulties := NewDifficultyRepository(client)
	tags := NewTagRepository(client)
	elements := cultivationdb.NewElementRepository(client)

	element := &cultivationentity.Element{
		Name:        taxonomyUnique("element"),
		Code:        "e" + uuid.NewString()[:12],
		Description: "created by PostgreSQL taxonomy acceptance",
	}
	require.NoError(t, elements.Create(ctx, element))
	require.NotEqual(t, uuid.Nil, element.ID)
	require.EqualValues(t, 1, element.Version)

	difficulty := &problementity.Difficulty{
		Name:        taxonomyUnique("difficulty"),
		Level:       taxonomyUniqueLevel(),
		ExpReward:   123,
		Description: "created by PostgreSQL taxonomy acceptance",
	}
	require.NoError(t, difficulties.Create(ctx, difficulty))
	require.NotEqual(t, uuid.Nil, difficulty.ID)
	require.EqualValues(t, 1, difficulty.Version)

	tag := &problementity.Tag{
		Name: taxonomyUnique("tag"),
		Elements: []problementity.TagElement{{
			ID: element.ID, Name: element.Name, Code: element.Code,
		}},
	}
	require.NoError(t, tags.Create(ctx, tag))
	require.EqualValues(t, 1, tag.Version)

	requireTaxonomyHistoryCounts(t, ctx, database, difficulty.ID, element.ID, tag.ID, 1, 1, 1, 1)

	element.Name = taxonomyUnique("element-v2")
	element.Description = "updated taxonomy element"
	require.NoError(t, elements.Update(ctx, element))
	difficulty.Name = taxonomyUnique("difficulty-v2")
	difficulty.ExpReward = 456
	require.NoError(t, difficulties.Update(ctx, difficulty))
	tag.Name = taxonomyUnique("tag-v2")
	tag.Elements = []problementity.TagElement{{ID: element.ID, Name: element.Name, Code: element.Code}}
	require.NoError(t, tags.Update(ctx, tag))

	require.EqualValues(t, 2, element.Version)
	require.EqualValues(t, 2, difficulty.Version)
	require.EqualValues(t, 2, tag.Version)
	requireTaxonomyHistoryCounts(t, ctx, database, difficulty.ID, element.ID, tag.ID, 2, 2, 2, 2)

	// The composite FK is the durable defense against a forged revision fact.
	// Use a distinct element so the link's unique pair cannot mask the exact
	// revision FK with a 23505 first.
	forgedElement := &cultivationentity.Element{
		Name: taxonomyUnique("forged-element"), Code: "e" + uuid.NewString()[:12],
	}
	require.NoError(t, elements.Create(ctx, forgedElement))
	_, err := database.ExecContext(ctx, `
		INSERT INTO public.tag_revision_elements (id, tag_revision_id, element_id, element_revision)
		VALUES ($1, $2, $3, 999999)
	`, uuid.New(), latestTaxonomyTagRevisionID(t, ctx, database, tag.ID), forgedElement.ID)
	requirePostgresSQLState(t, err, "23503")

	// A repository write joined to an outer transaction must leave neither its
	// stable projection nor its immutable revision behind on a late failure.
	rollbackName := taxonomyUnique("difficulty-rollback")
	lateFailure := errors.New("taxonomy late failure")
	err = client.DoInTx(ctx, func(txCtx context.Context) error {
		if err := difficulties.Create(txCtx, &problementity.Difficulty{
			Name: rollbackName, Level: taxonomyUniqueLevel(), ExpReward: 1,
		}); err != nil {
			return err
		}
		return lateFailure
	})
	require.ErrorIs(t, err, lateFailure)
	var stableCount, revisionCount int
	require.NoError(t, database.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM public.difficulties WHERE name = $1),
			(SELECT COUNT(*) FROM public.difficulty_revisions WHERE name = $1)
	`, rollbackName).Scan(&stableCount, &revisionCount))
	require.Zero(t, stableCount)
	require.Zero(t, revisionCount)

	rollbackTagName := taxonomyUnique("tag-rollback")
	err = client.DoInTx(ctx, func(txCtx context.Context) error {
		if err := tags.Create(txCtx, &problementity.Tag{
			Name: rollbackTagName,
			Elements: []problementity.TagElement{{
				ID: element.ID, Name: element.Name, Code: element.Code,
			}},
		}); err != nil {
			return err
		}
		return lateFailure
	})
	require.ErrorIs(t, err, lateFailure)
	var tagCount, tagRevisionCount, tagLinkCount int
	require.NoError(t, database.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM public.tags WHERE name = $1),
			(SELECT COUNT(*) FROM public.tag_revisions WHERE name = $1),
			(SELECT COUNT(*) FROM public.tag_revision_elements AS link
			 JOIN public.tag_revisions AS revision ON revision.id = link.tag_revision_id
			 WHERE revision.name = $1)
	`, rollbackTagName).Scan(&tagCount, &tagRevisionCount, &tagLinkCount))
	require.Zero(t, tagCount)
	require.Zero(t, tagRevisionCount)
	require.Zero(t, tagLinkCount)
}

func TestTaxonomyCatalogPostgresKeepsHistoricalRewardSnapshotAfterUpdatesAndSoftDelete(t *testing.T) {
	fixture := newTaxonomyPostgresFixture(t)
	catalog, err := NewTaxonomyCatalog(fixture.client.Client())
	require.NoError(t, err)

	facts := fixture.resolveCurrentFacts(t, catalog)
	snapshot, err := problementity.BuildRewardSnapshot(problementity.RewardMappingIdentity{
		TagMappingID:     uuid.New(),
		ElementMappingID: uuid.New(),
	}, 1, facts)
	require.NoError(t, err)

	fixture.element.Name = taxonomyUnique("historical-element-v2")
	require.NoError(t, fixture.elements.Update(fixture.ctx, fixture.element))
	fixture.difficulty.Name = taxonomyUnique("historical-difficulty-v2")
	fixture.difficulty.ExpReward = 987
	require.NoError(t, fixture.difficulties.Update(fixture.ctx, fixture.difficulty))
	fixture.tag.Name = taxonomyUnique("historical-tag-v2")
	fixture.tag.Elements = []problementity.TagElement{{
		ID: fixture.element.ID, Name: fixture.element.Name, Code: fixture.element.Code,
	}}
	require.NoError(t, fixture.tags.Update(fixture.ctx, fixture.tag))

	// History is independent from the mutable projection lifecycle. Verify it
	// remains readable both before and after all stable rows are soft deleted.
	require.NoError(t, fixture.elements.Delete(fixture.ctx, fixture.element.ID))
	require.NoError(t, fixture.difficulties.Delete(fixture.ctx, fixture.difficulty.ID))
	require.NoError(t, fixture.tags.Delete(fixture.ctx, fixture.tag.ID))
	requireTaxonomyHistoryCounts(
		t, fixture.ctx, fixture.database, fixture.difficulty.ID, fixture.element.ID, fixture.tag.ID,
		2, 2, 2, 2,
	)
	require.NoError(t, fixture.client.DoInTx(fixture.ctx, func(txCtx context.Context) error {
		return catalog.VerifyRewardSnapshot(txCtx, snapshot)
	}))
}

func TestTaxonomyCatalogPostgresResolutionIsWholeBeforeOrAfterConcurrentUpdate(t *testing.T) {
	fixture := newTaxonomyPostgresFixture(t)
	catalog, err := NewTaxonomyCatalog(fixture.client.Client())
	require.NoError(t, err)

	// The resolver obtains the stable Difficulty and Tag row locks before it
	// reports facts. A distinct connection then starts the Tag update and must
	// wait until the resolver commits. Channels make this ordering explicit;
	// no timing sleep is involved.
	factsReady := make(chan error, 1)
	releaseResolver := make(chan struct{})
	resolverDone := make(chan error, 1)
	var before problementity.RewardFacts
	go func() {
		resolverDone <- fixture.client.DoInTx(fixture.ctx, func(txCtx context.Context) error {
			facts, err := catalog.ResolveRewardFacts(txCtx, ports.RewardIntent{
				DifficultyID: fixture.difficulty.ID,
				TagIDs:       []uuid.UUID{fixture.tag.ID},
			})
			if err != nil {
				factsReady <- err
				return err
			}
			before = facts
			factsReady <- nil
			<-releaseResolver
			return nil
		})
	}()
	require.NoError(t, <-factsReady)

	updateDSN := dbtest.RequireDatabaseURL(t)
	if strings.Contains(updateDSN, "?") {
		updateDSN += "&application_name=judgify-taxonomy-update"
	} else {
		updateDSN += "?application_name=judgify-taxonomy-update"
	}
	updateDatabase, err := sql.Open("postgres", updateDSN)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, updateDatabase.Close()) })
	updateDatabase.SetMaxOpenConns(1)
	updateClient := dbent.WrapClient(
		generate.NewClient(generate.Driver(entsql.OpenDB(dialect.Postgres, updateDatabase))), nil,
	)
	updateTags := NewTagRepository(updateClient)
	updateDone := make(chan error, 1)
	update := *fixture.tag
	update.Name = taxonomyUnique("concurrent-tag-v2")
	go func() {
		updateDone <- updateTags.Update(fixture.ctx, &update)
	}()
	requireTaxonomyUpdateWaitsOnRowLock(t, fixture.ctx, fixture.database, updateDone)

	close(releaseResolver)
	require.NoError(t, <-resolverDone)
	require.NoError(t, <-updateDone)
	require.EqualValues(t, 1, before.Difficulty.Revision)
	require.Len(t, before.Tags, 1)
	require.EqualValues(t, 1, before.Tags[0].Revision)
	require.Len(t, before.Elements, 1)
	require.EqualValues(t, 1, before.Elements[0].Revision)

	after := fixture.resolveCurrentFacts(t, catalog)
	require.EqualValues(t, 1, after.Difficulty.Revision)
	require.Len(t, after.Tags, 1)
	require.EqualValues(t, 2, after.Tags[0].Revision)
	require.Len(t, after.Elements, 1)
	require.EqualValues(t, 1, after.Elements[0].Revision)
}

func requireTaxonomyUpdateWaitsOnRowLock(
	t *testing.T,
	ctx context.Context,
	database *sql.DB,
	updateDone <-chan error,
) {
	t.Helper()
	for {
		select {
		case err := <-updateDone:
			t.Fatalf("taxonomy update completed before resolver released its row lock: %v", err)
		default:
		}

		var waiting bool
		err := database.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM pg_stat_activity
				WHERE application_name = 'judgify-taxonomy-update'
				  AND wait_event_type = 'Lock'
			)
		`).Scan(&waiting)
		require.NoError(t, err)
		if waiting {
			return
		}
		// Yield without a wall-clock sleep: PostgreSQL and the updater continue
		// independently, while the test waits for the observable lock state.
		runtime.Gosched()
	}
}

func openTaxonomyPostgres(t *testing.T) (context.Context, *sql.DB, *dbent.EntClient) {
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
	client := generate.NewClient(generate.Driver(entsql.OpenDB(dialect.Postgres, database)))
	return ctx, database, dbent.WrapClient(client, nil)
}

type taxonomyPostgresFixture struct {
	ctx          context.Context
	database     *sql.DB
	client       *dbent.EntClient
	difficulties ports.DifficultyRepository
	tags         ports.TagRepository
	elements     interface {
		Create(context.Context, *cultivationentity.Element) error
		Update(context.Context, *cultivationentity.Element) error
		Delete(context.Context, uuid.UUID) error
	}
	difficulty *problementity.Difficulty
	element    *cultivationentity.Element
	tag        *problementity.Tag
}

func newTaxonomyPostgresFixture(t *testing.T) *taxonomyPostgresFixture {
	t.Helper()
	ctx, database, client := openTaxonomyPostgres(t)
	fixture := &taxonomyPostgresFixture{
		ctx:          ctx,
		database:     database,
		client:       client,
		difficulties: NewDifficultyRepository(client),
		tags:         NewTagRepository(client),
		elements:     cultivationdb.NewElementRepository(client),
		element: &cultivationentity.Element{
			Name: taxonomyUnique("fixture-element"), Code: "e" + uuid.NewString()[:12],
		},
		difficulty: &problementity.Difficulty{
			Name: taxonomyUnique("fixture-difficulty"), Level: taxonomyUniqueLevel(), ExpReward: 321,
		},
	}
	require.NoError(t, fixture.elements.Create(ctx, fixture.element))
	require.NoError(t, fixture.difficulties.Create(ctx, fixture.difficulty))
	fixture.tag = &problementity.Tag{
		Name: taxonomyUnique("fixture-tag"),
		Elements: []problementity.TagElement{{
			ID: fixture.element.ID, Name: fixture.element.Name, Code: fixture.element.Code,
		}},
	}
	require.NoError(t, fixture.tags.Create(ctx, fixture.tag))
	return fixture
}

func (fixture *taxonomyPostgresFixture) resolveCurrentFacts(
	t *testing.T,
	catalog *TaxonomyCatalog,
) problementity.RewardFacts {
	t.Helper()
	var facts problementity.RewardFacts
	require.NoError(t, fixture.client.DoInTx(fixture.ctx, func(txCtx context.Context) error {
		var err error
		facts, err = catalog.ResolveRewardFacts(txCtx, ports.RewardIntent{
			DifficultyID: fixture.difficulty.ID,
			TagIDs:       []uuid.UUID{fixture.tag.ID},
		})
		return err
	}))
	return facts
}

func requireTaxonomyHistoryCounts(
	t *testing.T,
	ctx context.Context,
	database *sql.DB,
	difficultyID, elementID, tagID uuid.UUID,
	wantDifficulty, wantElement, wantTag, wantLinks int,
) {
	t.Helper()
	var difficultyCount, elementCount, tagCount, linkCount int
	require.NoError(t, database.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM public.difficulty_revisions WHERE difficulty_id = $1),
			(SELECT COUNT(*) FROM public.element_revisions WHERE element_id = $2),
			(SELECT COUNT(*) FROM public.tag_revisions WHERE tag_id = $3),
			(SELECT COUNT(*) FROM public.tag_revision_elements AS link
			 JOIN public.tag_revisions AS revision ON revision.id = link.tag_revision_id
			 WHERE revision.tag_id = $3)
	`, difficultyID, elementID, tagID).Scan(&difficultyCount, &elementCount, &tagCount, &linkCount))
	require.Equal(t, wantDifficulty, difficultyCount)
	require.Equal(t, wantElement, elementCount)
	require.Equal(t, wantTag, tagCount)
	require.Equal(t, wantLinks, linkCount)
}

func latestTaxonomyTagRevisionID(t *testing.T, ctx context.Context, database *sql.DB, tagID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, database.QueryRowContext(ctx, `
		SELECT id FROM public.tag_revisions WHERE tag_id = $1 ORDER BY revision DESC LIMIT 1
	`, tagID).Scan(&id))
	return id
}

func requirePostgresSQLState(t *testing.T, err error, want string) {
	t.Helper()
	require.Error(t, err)
	var postgresError *pq.Error
	require.Truef(t, errors.As(err, &postgresError), "error = %v", err)
	require.Equal(t, want, string(postgresError.Code))
}

func taxonomyUnique(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, uuid.NewString()[:8])
}

func taxonomyUniqueLevel() int {
	return int(time.Now().UnixNano()%800000) + 100000
}

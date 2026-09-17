//go:build integration

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	_ "github.com/huynhanx03/judgify/internal/ent/generate/runtime"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
	"github.com/huynhanx03/judgify/internal/testutil/dbtest"
)

func TestRevisionRepositoryPostgresPersistsAndDeduplicatesCompleteDraftGraph(t *testing.T) {
	dsn := dbtest.RequireDatabaseURL(t)

	database, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	database.SetMaxOpenConns(4)
	database.SetMaxIdleConns(4)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		t.Fatalf("database.PingContext() error = %v", err)
	}

	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	contentRepository, err := contentrender.NewEntRepository(client)
	if err != nil {
		t.Fatalf("contentrender.NewEntRepository() error = %v", err)
	}
	repository, err := NewRevisionRepository(client, contentRepository)
	if err != nil {
		t.Fatalf("NewRevisionRepository() error = %v", err)
	}

	userID := task2DBUUID(80_001)
	difficultyID := task2DBUUID(80_002)
	problemID := task2DBUUID(80_003)
	tagMappingID := task2DBUUID(80_004)
	elementMappingID := task2DBUUID(80_005)
	difficultyRevisionID := task2DBUUID(80_006)
	createdAt := time.Date(2026, time.July, 26, 16, 0, 0, 0, time.UTC)
	difficultyChecksum := difficultyRevisionChecksum(difficultyID, 1, "Task 2", 1, 100, "")
	if _, err := database.ExecContext(ctx, `
		INSERT INTO public.users
			(id, created_at, updated_at, profile_visibility_updated_at, username, status, security_version)
		VALUES ($1, $2, $2, $2, 'task2-author', 'active', 1)
	`, userID, createdAt); err != nil {
		t.Fatalf("seed stable problem author: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO public.difficulties
			(id, created_at, updated_at, name, level, exp_reward)
		VALUES ($1, $2, $2, 'Task 2', 1, 100)
	`, difficultyID, createdAt); err != nil {
		t.Fatalf("seed stable problem difficulty: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO public.difficulty_revisions
			(id, created_at, difficulty_id, revision, name, level, exp_reward, description, checksum)
		VALUES ($1, $2, $3, 1, 'Task 2', 1, 100, '', $4)
	`, difficultyRevisionID, createdAt, difficultyID, difficultyChecksum[:]); err != nil {
		t.Fatalf("seed immutable problem difficulty revision: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO public.problems
			(id, created_at, updated_at, slug, lifecycle, version, title,
			 description, difficulty_id, time_limit_ms, memory_limit_kb,
			 author_id, tag_mapping_id, element_mapping_id, is_published, submission_count, accepted_count)
		VALUES ($1, $2, $2, 'task2-postgres-draft', 'draft', 1,
			'Legacy projection', 'Legacy projection', $3, 1000, 262144,
			$4, $5, $6, false, 0, 0)
	`, problemID, createdAt, difficultyID, userID, tagMappingID, elementMappingID); err != nil {
		t.Fatalf("seed stable problem aggregate: %v", err)
	}

	testset := testsetEntityFromRecord(t, validTestsetRecord(t))
	testset.ProblemID = problemID
	testset.RevisionNumber = 1
	testset.CreatedBy = userID
	testset.CreatedAt = createdAt
	assetCandidates := make([]entity.TestAsset, 0, testset.TestCount*2)
	seenAssets := make(map[[32]byte]struct{}, testset.TestCount*2)
	for groupIndex := range testset.Groups {
		group := &testset.Groups[groupIndex]
		group.TestsetRevisionID = testset.ID
		group.CreatedAt = createdAt
		for caseIndex := range group.Cases {
			item := &group.Cases[caseIndex]
			item.TestsetRevisionID = testset.ID
			item.ProblemID = problemID
			item.CreatedAt = createdAt
			for _, asset := range []entity.TestAsset{item.InputAsset, item.ExpectedAsset} {
				asset.CreatedAt = createdAt
				if _, exists := seenAssets[asset.SHA256]; !exists {
					seenAssets[asset.SHA256] = struct{}{}
					assetCandidates = append(assetCandidates, asset)
				}
			}
		}
	}

	statement := []byte("# Sum\n\nCompute `a + b`.\n")
	renderRequest, err := contentrender.NewRenderRequest(
		task2DBUUID(80_010),
		contentrender.ProblemStatement,
		statement,
	)
	if err != nil {
		t.Fatalf("contentrender.NewRenderRequest() error = %v", err)
	}
	renderer, err := contentrender.NewRenderer()
	if err != nil {
		t.Fatalf("contentrender.NewRenderer() error = %v", err)
	}
	renderArtifact, err := renderer.Render(ctx, renderRequest)
	if err != nil {
		t.Fatalf("renderer.Render() error = %v", err)
	}
	renderArtifact.CreatedBy = uuidPointer(userID)
	renderArtifact.CreatedAt = createdAt
	source := contentrender.Source{
		ID:              renderRequest.SourceID,
		OwnerRevisionID: task2DBUUID(80_011),
		Kind:            contentrender.ProblemStatement,
		Checksum:        renderRequest.SourceChecksum,
		CreatedBy:       uuidPointer(userID),
		CreatedAt:       createdAt,
	}

	var firstAssetIDs map[[32]byte]uuid.UUID
	var draftSelection entity.RenderSelection
	var persistedReward entity.RewardSnapshot
	uow := platformtx.NewEntUnitOfWork(client)
	err = uow.Within(ctx, func(txCtx context.Context) error {
		aggregate, err := repository.LockAggregate(txCtx, problemID)
		if err != nil {
			return err
		}
		numbers, err := repository.NextRevisionNumbers(txCtx, problemID)
		if err != nil {
			return err
		}
		if numbers.ProblemRevision != 1 || numbers.TestsetRevision != 1 {
			t.Fatalf("initial revision numbers = %#v", numbers)
		}
		resolved, err := repository.ResolveAssets(txCtx, assetCandidates)
		if err != nil {
			return err
		}
		firstAssetIDs = make(map[[32]byte]uuid.UUID, len(resolved))
		resolvedByChecksum := make(map[[32]byte]entity.TestAsset, len(resolved))
		for _, asset := range resolved {
			firstAssetIDs[asset.SHA256] = asset.ID
			resolvedByChecksum[asset.SHA256] = asset
		}
		for groupIndex := range testset.Groups {
			for caseIndex := range testset.Groups[groupIndex].Cases {
				item := &testset.Groups[groupIndex].Cases[caseIndex]
				item.InputAsset = resolvedByChecksum[item.InputAsset.SHA256]
				item.ExpectedAsset = resolvedByChecksum[item.ExpectedAsset.SHA256]
			}
		}
		testset.Checksum, err = testset.ComputeChecksum()
		if err != nil {
			return err
		}
		if err := repository.StoreTestset(txCtx, testset); err != nil {
			return err
		}
		persistedSource, err := contentRepository.RegisterSource(txCtx, source)
		if err != nil {
			return err
		}
		reward := entity.RewardSnapshot{
			DifficultyID:       difficultyID,
			DifficultyRevision: 1,
			BaseExperience:     100,
			Tags: entity.TagMappingSnapshot{
				MappingID: tagMappingID,
				Revision:  1,
				Items:     []entity.TagSnapshotItem{},
			},
		}
		reward.Checksum, err = reward.ComputeChecksum()
		if err != nil {
			return err
		}
		persistedReward = reward
		checkerConfig, err := entity.CanonicalizeCheckerConfig(json.RawMessage(`{"trim":true}`))
		if err != nil {
			return err
		}
		revision := entity.Revision{
			ID:                   source.OwnerRevisionID,
			ProblemID:            problemID,
			RevisionNumber:       numbers.ProblemRevision,
			Title:                "Sum",
			StatementMarkdown:    append([]byte(nil), renderRequest.NormalizedMarkdown...),
			RenderSourceID:       persistedSource.ID,
			RenderSourceKind:     string(persistedSource.Kind),
			RenderSourceChecksum: persistedSource.Checksum,
			Reward:               reward,
			Limits: entity.Limits{
				CPUTimeMilliseconds:  1_000,
				WallTimeMilliseconds: 2_000,
				MemoryKilobytes:      256 << 10,
				OutputBytes:          64 << 10,
				ProcessCount:         1,
			},
			CheckerKey:         "exact",
			CheckerVersion:     "v1",
			CheckerConfig:      checkerConfig,
			TestsetRevisionID:  testset.ID,
			AllowedRuntimeKeys: []string{"go1.25"},
			CreatedBy:          userID,
			CreatedAt:          createdAt,
		}
		revision.ContentChecksum, err = revision.ComputeContentChecksum()
		if err != nil {
			return err
		}
		if err := repository.StoreRevision(txCtx, revision); err != nil {
			return err
		}
		persistedArtifact, err := contentRepository.StoreArtifact(
			txCtx,
			persistedSource,
			renderArtifact,
		)
		if err != nil {
			return err
		}
		draftSelection = entity.RenderSelection{
			RevisionID:     revision.ID,
			SourceID:       persistedSource.ID,
			SourceKind:     string(persistedSource.Kind),
			SourceChecksum: persistedSource.Checksum,
			ArtifactID:     persistedArtifact.ID,
		}
		updated, err := repository.AdvanceDraft(
			txCtx,
			problemID,
			aggregate.Version,
			draftSelection,
			createdAt,
		)
		if err != nil {
			return err
		}
		if updated.Draft == nil || *updated.Draft != draftSelection ||
			updated.Version != aggregate.Version+1 {
			t.Fatalf("updated aggregate = %#v", updated)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("persist complete draft graph: %v", err)
	}
	storedRevision, err := client.ProblemRevision.Get(ctx, draftSelection.RevisionID)
	if err != nil {
		t.Fatalf("load persisted problem revision reward: %v", err)
	}
	loadedRevision, err := revisionFromRecord(storedRevision)
	if err != nil {
		t.Fatalf("decode persisted problem revision reward: %v", err)
	}
	persistedReward = loadedRevision.Reward

	// The reward belongs to the immutable ProblemRevision. It must continue to
	// verify against its exact catalog revision even after the mutable
	// Difficulty projection has moved forward and then been soft deleted.
	updatedDifficultyChecksum := difficultyRevisionChecksum(difficultyID, 2, "Task 2 v2", 1, 101, "")
	if _, err := client.Difficulty.UpdateOneID(difficultyID).
		SetName("Task 2 v2").
		SetExpReward(101).
		SetVersion(2).
		Save(ctx); err != nil {
		t.Fatalf("advance stable difficulty projection: %v", err)
	}
	if err := client.DifficultyRevision.Create().
		SetDifficultyID(difficultyID).
		SetRevision(2).
		SetName("Task 2 v2").
		SetLevel(1).
		SetExpReward(101).
		SetDescription("").
		SetChecksum(updatedDifficultyChecksum[:]).
		Exec(ctx); err != nil {
		t.Fatalf("append newer difficulty history: %v", err)
	}
	if err := client.Difficulty.DeleteOneID(difficultyID).Exec(ctx); err != nil {
		t.Fatalf("soft delete advanced difficulty projection: %v", err)
	}
	catalog, err := NewTaxonomyCatalog(client)
	if err != nil {
		t.Fatalf("NewTaxonomyCatalog: %v", err)
	}
	if err := uow.Within(ctx, func(txCtx context.Context) error {
		return catalog.VerifyRewardSnapshot(txCtx, persistedReward)
	}); err != nil {
		t.Fatalf("verify persisted historical reward after catalog evolution: %v", err)
	}
	if err := uow.Within(ctx, func(txCtx context.Context) error {
		_, resolveErr := catalog.ResolveRewardFacts(txCtx, ports.RewardIntent{DifficultyID: difficultyID})
		return resolveErr
	}); !errors.Is(err, ErrInvalidTaxonomyCatalog) {
		t.Fatalf("resolve active facts after difficulty soft delete = %v, want invalid catalog", err)
	}

	err = uow.Within(ctx, func(txCtx context.Context) error {
		reusedCandidates := make([]entity.TestAsset, len(assetCandidates))
		for index, asset := range assetCandidates {
			reusedCandidates[index] = asset
			reusedCandidates[index].ID = task2DBUUID(81_000 + uint64(index))
			reusedCandidates[index].CreatedAt = createdAt.Add(time.Hour)
		}
		resolved, err := repository.ResolveAssets(txCtx, reusedCandidates)
		if err != nil {
			return err
		}
		for _, asset := range resolved {
			if asset.ID != firstAssetIDs[asset.SHA256] {
				t.Fatalf("deduplicated asset ID = %s, want %s", asset.ID, firstAssetIDs[asset.SHA256])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("deduplicate existing test assets: %v", err)
	}

	var version int64
	var draftRevisionID, draftSourceID, draftArtifactID uuid.UUID
	if err := database.QueryRowContext(ctx, `
		SELECT version, draft_revision_id, draft_render_source_id, draft_render_artifact_id
		FROM public.problems
		WHERE id = $1
	`, problemID).Scan(&version, &draftRevisionID, &draftSourceID, &draftArtifactID); err != nil {
		t.Fatalf("load committed draft selection: %v", err)
	}
	if version != 2 ||
		draftRevisionID != draftSelection.RevisionID ||
		draftSourceID != draftSelection.SourceID ||
		draftArtifactID != draftSelection.ArtifactID {
		t.Fatalf(
			"committed selection = version:%d revision:%s source:%s artifact:%s",
			version,
			draftRevisionID,
			draftSourceID,
			draftArtifactID,
		)
	}

	lateFailure := errors.New("late transaction evidence failure")
	err = platformtx.NewEntUnitOfWork(client).Within(ctx, func(txCtx context.Context) error {
		aggregate, err := repository.LockAggregate(txCtx, problemID)
		if err != nil {
			return err
		}
		if aggregate.Version != version {
			return fmt.Errorf(
				"rollback probe locked version %d, want %d",
				aggregate.Version,
				version,
			)
		}
		updated, err := repository.AdvanceDraft(
			txCtx,
			problemID,
			version,
			draftSelection,
			createdAt.Add(2*time.Hour),
		)
		if err != nil {
			return err
		}
		if updated.Version != version+1 {
			return fmt.Errorf(
				"rollback probe advanced version %d, want %d",
				updated.Version,
				version+1,
			)
		}
		return lateFailure
	})
	if !errors.Is(err, lateFailure) {
		t.Fatalf("late transaction failure = %v, want %v", err, lateFailure)
	}
	var versionAfterRollback int64
	if err := database.QueryRowContext(
		ctx,
		`SELECT version FROM public.problems WHERE id = $1`,
		problemID,
	).Scan(&versionAfterRollback); err != nil {
		t.Fatalf("load version after rollback probe: %v", err)
	}
	if versionAfterRollback != version {
		t.Fatalf(
			"version after rollback = %d, want unchanged %d",
			versionAfterRollback,
			version,
		)
	}

	const concurrentWriters = 2
	start := make(chan struct{})
	results := make(chan error, concurrentWriters)
	var writers sync.WaitGroup
	writers.Add(concurrentWriters)
	for writer := 0; writer < concurrentWriters; writer++ {
		writer := writer
		go func() {
			defer writers.Done()
			<-start
			results <- platformtx.NewEntUnitOfWork(client).Within(
				ctx,
				func(txCtx context.Context) error {
					aggregate, err := repository.LockAggregate(txCtx, problemID)
					if err != nil {
						return err
					}
					if aggregate.Version != version {
						return ports.ErrDraftConflict
					}
					_, err = repository.AdvanceDraft(
						txCtx,
						problemID,
						version,
						draftSelection,
						createdAt.Add(3*time.Hour+time.Duration(writer)*time.Nanosecond),
					)
					return err
				},
			)
		}()
	}
	close(start)
	writers.Wait()
	close(results)

	var committedWriters, conflictedWriters int
	for result := range results {
		switch {
		case result == nil:
			committedWriters++
		case errors.Is(result, ports.ErrDraftConflict):
			conflictedWriters++
		default:
			t.Fatalf("concurrent draft writer error = %v", result)
		}
	}
	if committedWriters != 1 || conflictedWriters != 1 {
		t.Fatalf(
			"concurrent writers committed=%d conflicted=%d, want 1/1",
			committedWriters,
			conflictedWriters,
		)
	}
	var versionAfterConcurrency int64
	if err := database.QueryRowContext(
		ctx,
		`SELECT version FROM public.problems WHERE id = $1`,
		problemID,
	).Scan(&versionAfterConcurrency); err != nil {
		t.Fatalf("load version after concurrent writers: %v", err)
	}
	if versionAfterConcurrency != version+1 {
		t.Fatalf(
			"version after concurrent writers = %d, want %d",
			versionAfterConcurrency,
			version+1,
		)
	}

	immutableRows := []struct {
		table string
		id    uuid.UUID
	}{
		{table: "content_render_sources", id: source.ID},
		{table: "content_render_artifacts", id: draftSelection.ArtifactID},
		{table: "problem_revisions", id: draftSelection.RevisionID},
		{table: "test_assets", id: firstAssetIDs[assetCandidates[0].SHA256]},
		{table: "testcase_groups", id: testset.Groups[0].ID},
		{table: "testcase_revisions", id: testset.Groups[0].Cases[0].ID},
		{table: "testset_revisions", id: testset.ID},
	}
	for _, row := range immutableRows {
		row := row
		t.Run("append-only/"+row.table, func(t *testing.T) {
			requirePostgresErrorCode(
				t,
				execPostgresMutation(
					ctx,
					database,
					fmt.Sprintf(
						`UPDATE public.%s SET created_at = created_at WHERE id = $1`,
						row.table,
					),
					row.id,
				),
				"55000",
			)
			requirePostgresErrorCode(
				t,
				execPostgresMutation(
					ctx,
					database,
					fmt.Sprintf(`DELETE FROM public.%s WHERE id = $1`, row.table),
					row.id,
				),
				"55000",
			)
		})
	}

	publishedAt := createdAt.Add(time.Hour)
	archivedAt := publishedAt.Add(time.Hour)
	publicationStateMutation := `
		UPDATE public.problems
		SET lifecycle = $2,
			published_revision_id = draft_revision_id,
			published_render_source_id = draft_render_source_id,
			published_render_source_kind = draft_render_source_kind,
			published_render_source_checksum = draft_render_source_checksum,
			published_render_artifact_id = draft_render_artifact_id,
			published_at = $3,
			archived_at = $4,
			is_published = $5
		WHERE id = $1
	`
	visibilityCases := []struct {
		name      string
		statement string
		arguments []any
		errorCode string
	}{
		{
			name:      "draft cannot expose legacy public flag",
			statement: `UPDATE public.problems SET is_published = true WHERE id = $1`,
			arguments: []any{problemID},
			errorCode: "23514",
		},
		{
			name:      "published requires public flag",
			statement: publicationStateMutation,
			arguments: []any{problemID, "published", publishedAt, nil, false},
			errorCode: "23514",
		},
		{
			name:      "published accepts public flag",
			statement: publicationStateMutation,
			arguments: []any{problemID, "published", publishedAt, nil, true},
		},
		{
			name:      "archived rejects public flag",
			statement: publicationStateMutation,
			arguments: []any{problemID, "archived", publishedAt, archivedAt, true},
			errorCode: "23514",
		},
		{
			name:      "archived accepts hidden flag",
			statement: publicationStateMutation,
			arguments: []any{problemID, "archived", publishedAt, archivedAt, false},
		},
	}
	for _, test := range visibilityCases {
		test := test
		t.Run("visibility/"+test.name, func(t *testing.T) {
			err := execPostgresMutationInRollback(
				ctx,
				database,
				test.statement,
				test.arguments...,
			)
			if test.errorCode == "" {
				if err != nil {
					t.Fatalf("valid lifecycle visibility mutation failed: %v", err)
				}
				return
			}
			requirePostgresErrorCode(t, err, test.errorCode)
		})
	}

	_, err = database.ExecContext(ctx, `
		UPDATE public.problems
		SET draft_revision_id = $1,
			version = version + 1
		WHERE id = $2
	`, task2DBUUID(89_999), problemID)
	requirePostgresErrorCode(t, err, "23503")
}

func execPostgresMutation(
	ctx context.Context,
	database *sql.DB,
	statement string,
	arguments ...any,
) error {
	_, err := database.ExecContext(ctx, statement, arguments...)
	return err
}

func execPostgresMutationInRollback(
	ctx context.Context,
	database *sql.DB,
	statement string,
	arguments ...any,
) error {
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback() }()
	_, err = transaction.ExecContext(ctx, statement, arguments...)
	return err
}

func requirePostgresErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("PostgreSQL mutation succeeded, want SQLSTATE %s", code)
	}
	var postgresError *pq.Error
	if !errors.As(err, &postgresError) {
		t.Fatalf("PostgreSQL mutation error = %T %v, want *pq.Error", err, err)
	}
	if string(postgresError.Code) != code {
		t.Fatalf(
			"PostgreSQL mutation SQLSTATE = %s (%v), want %s",
			postgresError.Code,
			err,
			code,
		)
	}
}

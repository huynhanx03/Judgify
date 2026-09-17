package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

func TestAuthoringQueryAuthorizesBeforeLoadingHiddenDraft(t *testing.T) {
	t.Parallel()

	authorized := false
	repository := &authoringQueryRepositoryStub{
		aggregate: validAuthoringAggregate(),
		draft:     validAuthoringDraft(),
		onLoadDraft: func() {
			if !authorized {
				t.Fatal("hidden draft was loaded before object authorization")
			}
		},
	}
	authorizer := authoringQueryAuthorizerStub{
		authorize: func(context.Context, entity.ProblemAggregate) error {
			authorized = true
			return nil
		},
	}
	query, err := NewAuthoringQuery(AuthoringQueryDependencies{
		Repository: repository,
		Authorizer: authorizer,
	})
	if err != nil {
		t.Fatalf("NewAuthoringQuery() error = %v", err)
	}

	result, err := query.GetDraft(context.Background(), repository.aggregate.ID)
	if err != nil {
		t.Fatalf("GetDraft() error = %v", err)
	}
	if !authorized || repository.loadDraftCalls != 1 {
		t.Fatalf("authorization/load calls = %t/%d, want true/1", authorized, repository.loadDraftCalls)
	}
	if result.ProblemID != repository.aggregate.ID ||
		result.RevisionID != repository.aggregate.Draft.RevisionID ||
		result.Version != repository.aggregate.Version {
		t.Fatalf("GetDraft() identity = %#v, want aggregate draft identity", result)
	}

	// The query boundary owns every mutable collection returned by a driven
	// adapter so callers cannot mutate cached/shared repository state.
	result.TagIDs[0] = uuid.New()
	result.AllowedRuntimeKeys[0] = "mutated"
	result.Groups[0].Cases[0].Input = "mutated"
	if repository.draft.TagIDs[0] == result.TagIDs[0] ||
		repository.draft.AllowedRuntimeKeys[0] == result.AllowedRuntimeKeys[0] ||
		repository.draft.Groups[0].Cases[0].Input == result.Groups[0].Cases[0].Input {
		t.Fatal("GetDraft() returned repository-owned mutable state")
	}
}

func TestAuthoringQueryDenialNeverLoadsHiddenDraft(t *testing.T) {
	t.Parallel()

	denied := errors.New("denied")
	repository := &authoringQueryRepositoryStub{
		aggregate: validAuthoringAggregate(),
		draft:     validAuthoringDraft(),
	}
	query, err := NewAuthoringQuery(AuthoringQueryDependencies{
		Repository: repository,
		Authorizer: authoringQueryAuthorizerStub{
			authorize: func(context.Context, entity.ProblemAggregate) error {
				return denied
			},
		},
	})
	if err != nil {
		t.Fatalf("NewAuthoringQuery() error = %v", err)
	}

	_, err = query.GetDraft(context.Background(), repository.aggregate.ID)
	if !errors.Is(err, denied) {
		t.Fatalf("GetDraft() error = %v, want %v", err, denied)
	}
	if repository.loadDraftCalls != 0 {
		t.Fatalf("hidden draft load calls = %d, want 0", repository.loadDraftCalls)
	}
}

func TestAuthoringQueryRejectsMissingOrCorruptDraftProjection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*ports.AuthoringDraft)
	}{
		{
			name: "wrong aggregate",
			mutate: func(value *ports.AuthoringDraft) {
				value.ProblemID = uuid.New()
			},
		},
		{
			name: "wrong version",
			mutate: func(value *ports.AuthoringDraft) {
				value.Version++
			},
		},
		{
			name: "missing hidden testcase",
			mutate: func(value *ports.AuthoringDraft) {
				value.Groups[0].Cases[0].Kind = "sample"
			},
		},
		{
			name: "unsafe testcase content",
			mutate: func(value *ports.AuthoringDraft) {
				value.Groups[0].Cases[0].Input = "unsafe\x00content"
			},
		},
		{
			name: "invalid artifact status",
			mutate: func(value *ports.AuthoringDraft) {
				value.ArtifactStatus = "unknown"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			aggregate := validAuthoringAggregate()
			draft := validAuthoringDraft()
			test.mutate(&draft)
			query, err := NewAuthoringQuery(AuthoringQueryDependencies{
				Repository: &authoringQueryRepositoryStub{
					aggregate: aggregate,
					draft:     draft,
				},
				Authorizer: authoringQueryAuthorizerStub{},
			})
			if err != nil {
				t.Fatalf("NewAuthoringQuery() error = %v", err)
			}
			if _, err := query.GetDraft(context.Background(), aggregate.ID); !errors.Is(err, ports.ErrInvalidAuthoringQuery) {
				t.Fatalf("GetDraft() error = %v, want ErrInvalidAuthoringQuery", err)
			}
		})
	}
}

func TestAuthoringQueryRequiresCompleteDependenciesAndInput(t *testing.T) {
	t.Parallel()

	validRepository := &authoringQueryRepositoryStub{
		aggregate: validAuthoringAggregate(),
		draft:     validAuthoringDraft(),
	}
	if _, err := NewAuthoringQuery(AuthoringQueryDependencies{}); !errors.Is(err, ports.ErrInvalidAuthoringQuery) {
		t.Fatalf("NewAuthoringQuery(empty) error = %v", err)
	}
	query, err := NewAuthoringQuery(AuthoringQueryDependencies{
		Repository: validRepository,
		Authorizer: authoringQueryAuthorizerStub{},
	})
	if err != nil {
		t.Fatalf("NewAuthoringQuery() error = %v", err)
	}
	if _, err := query.GetDraft(context.TODO(), uuid.Nil); !errors.Is(err, ports.ErrInvalidAuthoringQuery) {
		t.Fatalf("GetDraft(nil UUID) error = %v", err)
	}
	if _, err := query.GetDraft(context.Background(), uuid.Nil); !errors.Is(err, ports.ErrInvalidAuthoringQuery) {
		t.Fatalf("GetDraft(nil UUID) error = %v", err)
	}
}

type authoringQueryRepositoryStub struct {
	aggregate      entity.ProblemAggregate
	draft          ports.AuthoringDraft
	loadDraftCalls int
	onLoadDraft    func()
}

func (stub *authoringQueryRepositoryStub) LoadAuthoringAggregate(
	context.Context,
	uuid.UUID,
) (entity.ProblemAggregate, error) {
	return stub.aggregate, nil
}

func (stub *authoringQueryRepositoryStub) LoadAuthoringDraft(
	context.Context,
	entity.ProblemAggregate,
) (ports.AuthoringDraft, error) {
	stub.loadDraftCalls++
	if stub.onLoadDraft != nil {
		stub.onLoadDraft()
	}
	return stub.draft, nil
}

type authoringQueryAuthorizerStub struct {
	authorize func(context.Context, entity.ProblemAggregate) error
}

func (stub authoringQueryAuthorizerStub) AuthorizeReadDraft(
	ctx context.Context,
	aggregate entity.ProblemAggregate,
) error {
	if stub.authorize == nil {
		return nil
	}
	return stub.authorize(ctx, aggregate)
}

func validAuthoringAggregate() entity.ProblemAggregate {
	now := time.Date(2026, time.July, 29, 8, 0, 0, 0, time.UTC)
	return entity.ProblemAggregate{
		ID:               uuid.MustParse("10000000-0000-4000-8000-000000000001"),
		Slug:             "two-sum",
		AuthorID:         uuid.MustParse("10000000-0000-4000-8000-000000000002"),
		TagMappingID:     uuid.MustParse("10000000-0000-4000-8000-000000000003"),
		ElementMappingID: uuid.MustParse("10000000-0000-4000-8000-000000000004"),
		Lifecycle:        entity.LifecycleDraft,
		Version:          3,
		Draft: &entity.RenderSelection{
			RevisionID:     uuid.MustParse("10000000-0000-4000-8000-000000000005"),
			SourceID:       uuid.MustParse("10000000-0000-4000-8000-000000000006"),
			SourceKind:     string(contentrender.ProblemStatement),
			SourceChecksum: [32]byte{1},
			ArtifactID:     uuid.MustParse("10000000-0000-4000-8000-000000000007"),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func validAuthoringDraft() ports.AuthoringDraft {
	aggregate := validAuthoringAggregate()
	return ports.AuthoringDraft{
		ProblemID:         aggregate.ID,
		Slug:              aggregate.Slug,
		Lifecycle:         aggregate.Lifecycle,
		Version:           aggregate.Version,
		RevisionID:        aggregate.Draft.RevisionID,
		TestsetRevisionID: uuid.MustParse("10000000-0000-4000-8000-000000000008"),
		Title:             "Two Sum",
		StatementMarkdown: "# Two Sum",
		DifficultyID:      uuid.MustParse("10000000-0000-4000-8000-000000000009"),
		TagIDs: []uuid.UUID{
			uuid.MustParse("10000000-0000-4000-8000-000000000010"),
		},
		Limits: entity.Limits{
			CPUTimeMilliseconds:  1000,
			WallTimeMilliseconds: 2000,
			MemoryKilobytes:      262144,
			OutputBytes:          1 << 20,
			ProcessCount:         32,
		},
		CheckerKey:          "exact",
		CheckerVersion:      "v1",
		CheckerConfig:       []byte(`{}`),
		AllowedRuntimeKeys:  []string{"go"},
		ArtifactStatus:      contentrender.StatusReady,
		ArtifactDiagnostics: []string{},
		SavedAt:             time.Date(2026, time.July, 29, 8, 1, 0, 0, time.UTC),
		Groups: []ports.AuthoringTestcaseGroup{
			{
				StopOnFailure: false,
				Cases: []ports.AuthoringTestcase{
					{
						TestCaseID:     uuid.MustParse("10000000-0000-4000-8000-000000000011"),
						Kind:           "hidden",
						Input:          "1 2\n",
						ExpectedOutput: "3\n",
					},
				},
			},
		},
	}
}

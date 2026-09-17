package db

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

func TestContestProblemProjectionReturnsExactBrowserMembershipsInOrder(t *testing.T) {
	database, mock, err := sqlmock.New(
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp),
	)
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	repository := NewContestRepository(
		dbent.WrapClient(client, nil),
	).(*ContestRepository)
	contestID := uuid.New()
	firstMembershipID, firstProblemID := uuid.New(), uuid.New()
	secondMembershipID, secondProblemID := uuid.New(), uuid.New()
	points := 100
	mock.ExpectQuery(`SELECT selected.id, selected.problem_id, owner.slug, revision.title,.*selected.display_order`).
		WithArgs(contestID, maximumContestProblemProjections+1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"problem_id",
			"problem_slug",
			"problem_title",
			"display_order",
			"alias",
			"points",
			"visible_before_start",
		}).
			AddRow(
				firstMembershipID,
				firstProblemID,
				"two-sum",
				"Two Sum",
				1,
				"A",
				nil,
				true,
			).
			AddRow(
				secondMembershipID,
				secondProblemID,
				"sort",
				"Sort",
				2,
				"B",
				points,
				false,
			))

	result, err := repository.GetContestProblemProjections(
		context.Background(),
		contestID,
	)
	if err != nil {
		t.Fatalf("GetContestProblemProjections() error = %v", err)
	}
	if len(result) != 2 ||
		result[0].ContestProblemID != firstMembershipID ||
		result[0].ProblemID != firstProblemID ||
		result[1].ContestProblemID != secondMembershipID ||
		result[1].Points == nil ||
		*result[1].Points != points {
		t.Fatalf("contest problem projections = %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("database expectations: %v", err)
	}
}

func TestContestProblemProjectionRejectsDuplicateProblemIdentity(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	repository := NewContestRepository(
		dbent.WrapClient(client, nil),
	).(*ContestRepository)
	contestID, problemID := uuid.New(), uuid.New()
	mock.ExpectQuery(`SELECT selected.id, selected.problem_id, owner.slug, revision.title`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"problem_id",
			"problem_slug",
			"problem_title",
			"display_order",
			"alias",
			"points",
			"visible_before_start",
		}).
			AddRow(uuid.New(), problemID, "two-sum", "Two Sum", 1, "A", nil, true).
			AddRow(uuid.New(), problemID, "two-sum", "Two Sum", 2, "B", nil, true))

	if _, err := repository.GetContestProblemProjections(
		context.Background(),
		contestID,
	); err == nil {
		t.Fatal("duplicate contest problem identity unexpectedly projected")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("database expectations: %v", err)
	}
}

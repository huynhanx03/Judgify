package db

import (
	"strings"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
	commondto "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/go-common/pkg/utils"

	"github.com/huynhanx03/judgify/internal/ent/generate/difficulty"
	"github.com/huynhanx03/judgify/internal/ent/generate/element"
	"github.com/huynhanx03/judgify/internal/ent/generate/predicate"
	"github.com/huynhanx03/judgify/internal/ent/generate/problem"
	"github.com/huynhanx03/judgify/internal/ent/generate/tag"
)

const problemAcceptanceRateSortKey = "acceptance_rate"

var (
	problemQueryColumns = []string{
		problem.FieldID,
		problem.FieldCreatedAt,
		problem.FieldUpdatedAt,
		problem.FieldTitle,
		problem.FieldDifficultyID,
		problem.FieldTimeLimitMs,
		problem.FieldMemoryLimitKB,
		problem.FieldAuthorID,
		problem.FieldIsPublished,
		problem.FieldSubmissionCount,
		problem.FieldAcceptedCount,
	}
	tagQueryColumns = []string{
		tag.FieldID,
		tag.FieldCreatedAt,
		tag.FieldUpdatedAt,
		tag.FieldName,
	}
	difficultyQueryColumns = []string{
		difficulty.FieldID,
		difficulty.FieldCreatedAt,
		difficulty.FieldUpdatedAt,
		difficulty.FieldName,
		difficulty.FieldLevel,
		difficulty.FieldExpReward,
	}
)

func applyProblemQueryWindow(
	options *commondto.QueryOptions,
	selector *entsql.Selector,
) {
	if options == nil || len(options.Sort) != 1 ||
		utils.ToSnakeCase(options.Sort[0].Key) !=
			problemAcceptanceRateSortKey {
		commonent.ApplyQueryWindow(
			options,
			selector,
			problemQueryColumns...,
		)
		return
	}

	accepted := selector.C(problem.FieldAcceptedCount)
	submitted := selector.C(problem.FieldSubmissionCount)
	ratio := entsql.ExprFunc(func(builder *entsql.Builder) {
		builder.
			WriteString("CASE WHEN ").
			Ident(submitted).
			WriteString(" = 0 THEN 0::double precision ELSE ").
			Ident(accepted).
			WriteString("::double precision / NULLIF(").
			Ident(submitted).
			WriteString(", 0) END")
	})
	if options.Sort[0].Order == -1 {
		selector.
			OrderExpr(entsql.DescExpr(ratio)).
			OrderBy(
				entsql.Desc(problem.FieldAcceptedCount),
				entsql.Desc(problem.FieldSubmissionCount),
				entsql.Desc(problem.FieldID),
			)
	} else {
		selector.
			OrderExpr(ratio).
			OrderBy(
				entsql.Asc(problem.FieldAcceptedCount),
				entsql.Asc(problem.FieldSubmissionCount),
				entsql.Asc(problem.FieldID),
			)
	}
	commonent.ApplyPagination(options.Pagination, selector)
}

func problemRelationPredicates(opts *commondto.QueryOptions) []predicate.Problem {
	if opts == nil {
		return nil
	}
	result := make([]predicate.Problem, 0, 1)
	for _, filter := range opts.Filters {
		if utils.ToSnakeCase(filter.Key) != "tag_ids" {
			continue
		}
		ids := queryUUIDs(filter.Value)
		if len(ids) > 0 {
			result = append(result, problem.HasTagsWith(tag.IDIn(ids...)))
		}
	}
	return result
}

func tagRelationPredicates(opts *commondto.QueryOptions) []predicate.Tag {
	if opts == nil {
		return nil
	}
	result := make([]predicate.Tag, 0, 1)
	for _, filter := range opts.Filters {
		if utils.ToSnakeCase(filter.Key) != "element_id" {
			continue
		}
		ids := queryUUIDs(filter.Value)
		if len(ids) > 0 {
			result = append(result, tag.HasElementsWith(element.IDIn(ids...)))
		}
	}
	return result
}

func queryUUIDs(value any) []uuid.UUID {
	switch typed := value.(type) {
	case uuid.UUID:
		if typed == uuid.Nil {
			return nil
		}
		return []uuid.UUID{typed}
	case string:
		parsed, err := uuid.Parse(strings.TrimSpace(typed))
		if err != nil || parsed == uuid.Nil {
			return nil
		}
		return []uuid.UUID{parsed}
	case []uuid.UUID:
		result := make([]uuid.UUID, 0, len(typed))
		for _, id := range typed {
			if id != uuid.Nil {
				result = append(result, id)
			}
		}
		return result
	case []string:
		values := make([]any, len(typed))
		for index := range typed {
			values[index] = typed[index]
		}
		return queryUUIDs(values)
	case []any:
		result := make([]uuid.UUID, 0, len(typed))
		for _, item := range typed {
			result = append(result, queryUUIDs(item)...)
		}
		return result
	default:
		return nil
	}
}

package db

import (
	"strings"

	commondto "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/go-common/pkg/utils"

	"github.com/huynhanx03/judgify/internal/ent/generate/contest"
	"github.com/huynhanx03/judgify/internal/ent/generate/predicate"
)

func contestSearchPredicates(opts *commondto.QueryOptions) []predicate.Contest {
	if opts == nil {
		return nil
	}
	result := make([]predicate.Contest, 0, 1)
	for _, filter := range opts.Filters {
		if utils.ToSnakeCase(filter.Key) != "name" {
			continue
		}
		term, ok := filter.Value.(string)
		if !ok {
			continue
		}
		term = strings.TrimSpace(term)
		if term != "" {
			result = append(result, contest.TitleContainsFold(term))
		}
	}
	return result
}

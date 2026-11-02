package db

import (
	"strings"

	commondto "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/go-common/pkg/utils"

	"github.com/huynhanx03/judgify/internal/ent/generate/material"
	"github.com/huynhanx03/judgify/internal/ent/generate/materialcategory"
	"github.com/huynhanx03/judgify/internal/ent/generate/predicate"
)

var (
	materialQueryColumns = []string{
		material.FieldID,
		material.FieldCreatedAt,
		material.FieldUpdatedAt,
		material.FieldTitle,
		material.FieldDifficultyID,
		material.FieldAuthorID,
		material.FieldCategoryID,
		material.FieldStatus,
		material.FieldVisibility,
		material.FieldGroupID,
		material.FieldViewCount,
		material.FieldEstimatedReadTime,
	}
	materialCategoryQueryColumns = []string{
		materialcategory.FieldID,
		materialcategory.FieldCreatedAt,
		materialcategory.FieldUpdatedAt,
		materialcategory.FieldName,
	}
)

func materialSearchPredicates(opts *commondto.QueryOptions) []predicate.Material {
	if opts == nil {
		return nil
	}
	result := make([]predicate.Material, 0, 1)
	for _, filter := range opts.Filters {
		key := utils.ToSnakeCase(filter.Key)
		if key != "search" && key != "name" {
			continue
		}
		term, ok := filter.Value.(string)
		if !ok {
			continue
		}
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		result = append(result, material.Or(
			material.TitleContainsFold(term),
			material.DescriptionContainsFold(term),
		))
	}
	return result
}

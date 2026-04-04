package service

import (
	"context"
	"math/rand"
	"sort"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/cultivation/core/mapper"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
)

const (
	TraitTypeRootBone = "root_bone"
	TraitTypeTalent   = "talent"

	DefaultRootBoneCount = 1
	DefaultTalentCount   = 6
)

// weightedPool holds precomputed prefix sums for O(log n) weighted random sampling.
type weightedPool struct {
	traits    []*entity.TraitWithWeight
	prefixSum []int
	totalW    int
}

// sample picks n unique random traits using prefix-sum binary search.
func (p *weightedPool) sample(n int) []*entity.TraitWithWeight {
	if n <= 0 || len(p.traits) == 0 {
		return nil
	}
	if n > len(p.traits) {
		n = len(p.traits)
	}

	picked := make(map[int]struct{}, n)
	result := make([]*entity.TraitWithWeight, 0, n)

	for len(result) < n {
		r := rand.Intn(p.totalW)
		idx := sort.SearchInts(p.prefixSum, r+1)
		if idx >= len(p.traits) {
			idx = len(p.traits) - 1
		}
		if _, exists := picked[idx]; exists {
			continue
		}
		picked[idx] = struct{}{}
		result = append(result, p.traits[idx])
	}
	return result
}

// buildPool builds a weighted pool from traits with precomputed prefix sums.
func buildPool(traits []*entity.TraitWithWeight) *weightedPool {
	prefix := make([]int, len(traits))
	total := 0
	for i, t := range traits {
		total += t.Weight
		prefix[i] = total
	}
	return &weightedPool{traits: traits, prefixSum: prefix, totalW: total}
}

type gachaService struct {
	traitRepo    ports.TraitRepository
	rootBonePool *weightedPool
	talentPool   *weightedPool
}

// NewGachaService creates a new GachaService.
func NewGachaService(traitRepo ports.TraitRepository) ports.GachaService {
	return &gachaService{traitRepo: traitRepo}
}

// LoadPool fetches all traits from DB once, splits by type, and builds weighted pools.
func (s *gachaService) LoadPool(ctx context.Context) error {
	traits, err := s.traitRepo.FindAllWithWeight(ctx)
	if err != nil {
		return err
	}

	byType := make(map[string][]*entity.TraitWithWeight)
	for _, t := range traits {
		byType[t.Type] = append(byType[t.Type], t)
	}

	s.rootBonePool = buildPool(byType[TraitTypeRootBone])
	s.talentPool = buildPool(byType[TraitTypeTalent])
	return nil
}

// InvalidatePool reloads pools from DB.
func (s *gachaService) InvalidatePool(ctx context.Context) error {
	return s.LoadPool(ctx)
}

// Roll performs a weighted gacha roll with default counts.
func (s *gachaService) Roll(ctx context.Context) (*dto.GachaRollResponse, error) {
	if s.rootBonePool == nil || s.talentPool == nil {
		return nil, apperr.New(response.CodeInternalError, "gacha pool not loaded", nil)
	}
	if s.rootBonePool.totalW == 0 || s.talentPool.totalW == 0 {
		return nil, apperr.New(response.CodeInternalError, "trait pool is empty", nil)
	}

	rootBones := s.rootBonePool.sample(DefaultRootBoneCount)
	talents := s.talentPool.sample(DefaultTalentCount)

	resp := &dto.GachaRollResponse{
		RootBones: make([]*dto.TraitResponse, len(rootBones)),
		Talents:   make([]*dto.TraitResponse, len(talents)),
	}
	for i, t := range rootBones {
		resp.RootBones[i] = mapper.ToTraitResponse(t.Trait)
	}
	for i, t := range talents {
		resp.Talents[i] = mapper.ToTraitResponse(t.Trait)
	}
	return resp, nil
}

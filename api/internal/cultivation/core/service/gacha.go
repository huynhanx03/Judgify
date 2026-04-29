package service

import (
	"context"
	"math/rand"
	"sync"

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

// aliasPool stores Alias Method tables for O(1) weighted sampling.
type aliasPool struct {
	traits []*entity.TraitWithWeight
	prob   []float64
	alias  []int
}

func (p *aliasPool) size() int {
	if p == nil {
		return 0
	}
	return len(p.traits)
}

// sampleIndex picks one item index in O(1) using alias tables.
func (p *aliasPool) sampleIndex() int {
	col := rand.Intn(len(p.traits))
	if rand.Float64() < p.prob[col] {
		return col
	}
	return p.alias[col]
}

// sample picks n unique random traits.
func (p *aliasPool) sample(n int) []*entity.TraitWithWeight {
	size := p.size()
	if n <= 0 || size == 0 {
		return nil
	}
	if n >= size {
		result := make([]*entity.TraitWithWeight, size)
		copy(result, p.traits)
		return result
	}

	picked := make(map[int]struct{}, n)
	result := make([]*entity.TraitWithWeight, 0, n)

	for len(result) < n {
		idx := p.sampleIndex()
		if _, exists := picked[idx]; exists {
			continue
		}
		picked[idx] = struct{}{}
		result = append(result, p.traits[idx])
	}
	return result
}

// buildPool builds Alias Method tables in O(n).
func buildPool(traits []*entity.TraitWithWeight) *aliasPool {
	filtered := make([]*entity.TraitWithWeight, 0, len(traits))
	total := 0
	for _, t := range traits {
		if t.Weight <= 0 {
			continue
		}
		filtered = append(filtered, t)
		total += t.Weight
	}

	if len(filtered) == 0 || total <= 0 {
		return &aliasPool{}
	}

	n := len(filtered)
	prob := make([]float64, n)
	alias := make([]int, n)
	scaled := make([]float64, n)
	small := make([]int, 0, n)
	large := make([]int, 0, n)

	for i, t := range filtered {
		scaled[i] = float64(t.Weight) * float64(n) / float64(total)
		if scaled[i] < 1.0 {
			small = append(small, i)
		} else {
			large = append(large, i)
		}
	}

	for len(small) > 0 && len(large) > 0 {
		si := len(small) - 1
		s := small[si]
		small = small[:si]

		li := len(large) - 1
		l := large[li]
		large = large[:li]

		prob[s] = scaled[s]
		alias[s] = l

		scaled[l] = scaled[l] + scaled[s] - 1.0
		if scaled[l] < 1.0 {
			small = append(small, l)
			continue
		}
		large = append(large, l)
	}

	for _, i := range large {
		prob[i] = 1.0
		alias[i] = i
	}
	for _, i := range small {
		prob[i] = 1.0
		alias[i] = i
	}

	return &aliasPool{
		traits: filtered,
		prob:   prob,
		alias:  alias,
	}
}

type gachaService struct {
	traitRepo ports.TraitRepository

	mu           sync.RWMutex
	rootBonePool *aliasPool
	talentPool   *aliasPool
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

	rootBonePool := buildPool(byType[TraitTypeRootBone])
	talentPool := buildPool(byType[TraitTypeTalent])

	s.mu.Lock()
	s.rootBonePool = rootBonePool
	s.talentPool = talentPool
	s.mu.Unlock()
	return nil
}

// InvalidatePool reloads pools from DB.
func (s *gachaService) InvalidatePool(ctx context.Context) error {
	return s.LoadPool(ctx)
}

// Roll performs a weighted gacha roll with default counts.
func (s *gachaService) Roll(ctx context.Context) (*dto.GachaRollResponse, error) {
	s.mu.RLock()
	rootBonePool := s.rootBonePool
	talentPool := s.talentPool
	s.mu.RUnlock()

	if rootBonePool == nil || talentPool == nil {
		return nil, apperr.New(response.CodeInternalError, "gacha pool not loaded", nil)
	}
	if rootBonePool.size() == 0 || talentPool.size() == 0 {
		return nil, apperr.New(response.CodeInternalError, "trait pool is empty", nil)
	}

	rootBones := rootBonePool.sample(DefaultRootBoneCount)
	talents := talentPool.sample(DefaultTalentCount)

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

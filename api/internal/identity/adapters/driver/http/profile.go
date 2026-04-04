package http

import (
	"math"
	"sort"

	"github.com/gin-gonic/gin"

	cultivationEntity "github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	cultivationPorts "github.com/huynhanx03/judgify/internal/cultivation/ports"
	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/ports"
	"github.com/huynhanx03/judgify/pkg/common/http/middlewares"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/constraints"
)

// Store keys for profile enrichers.
const (
	profileKeyBasic        = "profile.basic"
	profileKeyStats        = "profile.stats"
	profileKeyTraits       = "profile.traits"
	profileKeyElements     = "profile.elements"
	profileKeyLevels       = "profile.levels"
	profileKeyRanks        = "profile.ranks"
	profileKeyProblemStats = "profile.problem_stats"
)

// ProfileHandler runs parallel enrichers then assembles ProfileResponse.
type ProfileHandler interface {
	WithBasicProfile(c *gin.Context)
	WithCultivationStats(c *gin.Context)
	WithUserTraits(c *gin.Context)
	WithElementExps(c *gin.Context)
	WithLevelsRanks(c *gin.Context)
	WithProblemStats(c *gin.Context)
	GetProfile(c *gin.Context)
}

type profileHandler struct {
	userService  ports.UserService
	statsRepo    cultivationPorts.UserStatsRepository
	traitRepo    cultivationPorts.UserTraitRepository
	elemExpRepo  cultivationPorts.UserElementExpRepository
	levelRepo    cultivationPorts.LevelRepository
	rankRepo     cultivationPorts.RankRepository
	diffStatsRepo cultivationPorts.UserDifficultyStatsRepository
	tagStatsRepo  cultivationPorts.UserTagStatsRepository
}

// NewProfileHandler creates a ProfileHandler with all required dependencies.
func NewProfileHandler(
	userService ports.UserService,
	statsRepo cultivationPorts.UserStatsRepository,
	traitRepo cultivationPorts.UserTraitRepository,
	elemExpRepo cultivationPorts.UserElementExpRepository,
	levelRepo cultivationPorts.LevelRepository,
	rankRepo cultivationPorts.RankRepository,
	diffStatsRepo cultivationPorts.UserDifficultyStatsRepository,
	tagStatsRepo cultivationPorts.UserTagStatsRepository,
) ProfileHandler {
	return &profileHandler{
		userService:   userService,
		statsRepo:     statsRepo,
		traitRepo:     traitRepo,
		elemExpRepo:   elemExpRepo,
		levelRepo:     levelRepo,
		rankRepo:      rankRepo,
		diffStatsRepo: diffStatsRepo,
		tagStatsRepo:  tagStatsRepo,
	}
}

// ---- Enrichers (run concurrently via middlewares.Parallel) ----

func (h *profileHandler) WithBasicProfile(c *gin.Context) {
	userID := c.Request.Context().Value(constraints.ContextKeyUserID).(int)
	profile, err := h.userService.GetProfile(c.Request.Context(), userID)
	if err != nil {
		middlewares.Abort(c, err)
		return
	}
	middlewares.Set(c, profileKeyBasic, profile)
}

func (h *profileHandler) WithCultivationStats(c *gin.Context) {
	userID := c.Request.Context().Value(constraints.ContextKeyUserID).(int)
	stats, err := h.statsRepo.GetByUserID(c.Request.Context(), userID)
	if err != nil {
		return // not critical — new users may not have stats yet
	}
	middlewares.Set(c, profileKeyStats, stats)
}

func (h *profileHandler) WithUserTraits(c *gin.Context) {
	userID := c.Request.Context().Value(constraints.ContextKeyUserID).(int)
	traits, err := h.traitRepo.GetByUserID(c.Request.Context(), userID)
	if err != nil {
		return
	}
	middlewares.Set(c, profileKeyTraits, traits)
}

func (h *profileHandler) WithElementExps(c *gin.Context) {
	userID := c.Request.Context().Value(constraints.ContextKeyUserID).(int)
	exps, err := h.elemExpRepo.GetByUserID(c.Request.Context(), userID)
	if err != nil {
		return
	}
	middlewares.Set(c, profileKeyElements, exps)
}

func (h *profileHandler) WithProblemStats(c *gin.Context) {
	userID := c.Request.Context().Value(constraints.ContextKeyUserID).(int)

	diffStats, err := h.diffStatsRepo.GetByUserID(c.Request.Context(), userID)
	if err != nil {
		diffStats = nil
	}
	tagStats, err := h.tagStatsRepo.GetByUserID(c.Request.Context(), userID)
	if err != nil {
		tagStats = nil
	}
	middlewares.Set(c, profileKeyProblemStats, [2]interface{}{diffStats, tagStats})
}

func (h *profileHandler) WithLevelsRanks(c *gin.Context) {
	levels, err := h.levelRepo.FindAll(c.Request.Context())
	if err != nil {
		return
	}
	ranks, err := h.rankRepo.FindAll(c.Request.Context())
	if err != nil {
		return
	}
	middlewares.Set(c, profileKeyLevels, levels)
	middlewares.Set(c, profileKeyRanks, ranks)
}

// ---- Final handler ----

func (h *profileHandler) GetProfile(c *gin.Context) {
	basic, ok := middlewares.Get[*dto.ProfileAttrs](c, profileKeyBasic)
	if !ok || basic == nil {
		response.ErrorResponse(c, response.CodeInternalServer, nil)
		return
	}

	stats, _ := middlewares.Get[*cultivationEntity.UserStats](c, profileKeyStats)
	traits, _ := middlewares.Get[[]*cultivationEntity.Trait](c, profileKeyTraits)
	exps, _ := middlewares.Get[[]cultivationEntity.ElementExpDetail](c, profileKeyElements)
	levels, _ := middlewares.Get[[]*cultivationEntity.Level](c, profileKeyLevels)
	ranks, _ := middlewares.Get[[]*cultivationEntity.Rank](c, profileKeyRanks)
	problemStatsRaw, _ := middlewares.Get[[2]interface{}](c, profileKeyProblemStats)

	var diffStats []*cultivationEntity.UserDifficultyStats
	var tagStats []*cultivationEntity.UserTagStats
	if problemStatsRaw[0] != nil {
		diffStats, _ = problemStatsRaw[0].([]*cultivationEntity.UserDifficultyStats)
	}
	if problemStatsRaw[1] != nil {
		tagStats, _ = problemStatsRaw[1].([]*cultivationEntity.UserTagStats)
	}

	full := buildFullProfile(basic, stats, traits, exps, levels, ranks, diffStats, tagStats)
	response.SuccessResponse(c, response.CodeSuccess, full)
}

// ---- Builder helpers ----

func buildFullProfile(
	basic *dto.ProfileAttrs,
	stats *cultivationEntity.UserStats,
	traits []*cultivationEntity.Trait,
	exps []cultivationEntity.ElementExpDetail,
	levels []*cultivationEntity.Level,
	ranks []*cultivationEntity.Rank,
	diffStats []*cultivationEntity.UserDifficultyStats,
	tagStats []*cultivationEntity.UserTagStats,
) *dto.ProfileResponse {
	cultivation := dto.CultivationInfo{
		Talents:  []dto.TraitInfo{},
		Elements: []dto.ElementExp{},
	}

	problemStats := dto.ProblemStats{
		ByDifficulty: []dto.DiffStat{},
		ByTag:        []dto.TagStat{},
	}

	if stats != nil {
		cultivation.TotalExp = stats.TotalExp
		cultivation.Rating = stats.Rating
		cultivation.Level = resolveLevel(levels, stats.TotalExp)
		cultivation.Rank = resolveRank(ranks, stats.Rating)
		problemStats.TotalSubmissions = stats.TotalSubmissions
		problemStats.AcceptedCount = stats.AcceptedCount
	}

	for _, t := range traits {
		info := dto.TraitInfo{
			Name:        t.Name,
			Description: t.Description,
			Metadata:    t.Metadata,
		}
		if t.Rarity != nil {
			info.RarityCode = t.Rarity.Code
			info.RarityName = t.Rarity.Name
		}
		if t.Type == "root_bone" {
			copy := info
			cultivation.RootBone = &copy
		} else {
			cultivation.Talents = append(cultivation.Talents, info)
		}
	}

	for _, exp := range exps {
		cultivation.Elements = append(cultivation.Elements, dto.ElementExp{
			Code: exp.Code,
			Name: exp.Name,
			Exp:  exp.Exp,
		})
	}

	for _, d := range diffStats {
		problemStats.ByDifficulty = append(problemStats.ByDifficulty, dto.DiffStat{
			Name:        d.DifficultyName,
			Level:       d.DifficultyLevel,
			SolvedCount: d.SolvedCount,
		})
	}

	for _, t := range tagStats {
		elems := make([]dto.ElementBrief, len(t.Elements))
		for i, e := range t.Elements {
			elems[i] = dto.ElementBrief{Code: e.Code, Name: e.Name}
		}
		problemStats.ByTag = append(problemStats.ByTag, dto.TagStat{
			Name:        t.TagName,
			SolvedCount: t.SolvedCount,
			Elements:    elems,
		})
	}

	return &dto.ProfileResponse{
		Username:     basic.Username,
		FirstName:    basic.FirstName,
		LastName:     basic.LastName,
		Gender:       basic.Gender,
		Birthday:     basic.Birthday,
		JoinedAt:     basic.JoinedAt,
		Cultivation:  cultivation,
		ProblemStats: problemStats,
	}
}

func resolveLevel(levels []*cultivationEntity.Level, totalExp int64) dto.LevelInfo {
	if len(levels) == 0 {
		return dto.LevelInfo{Name: "—"}
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i].MinExp < levels[j].MinExp })

	idx := 0
	for i, l := range levels {
		if l.MinExp <= totalExp {
			idx = i
		} else {
			break
		}
	}

	current := levels[idx]
	info := dto.LevelInfo{Name: current.Name, TierIndex: idx}

	if idx+1 < len(levels) {
		next := levels[idx+1]
		info.NextName = next.Name
		info.NextTierIndex = idx + 1
		diff := next.MinExp - current.MinExp
		if diff > 0 {
			raw := float64(totalExp-current.MinExp) / float64(diff) * 100
			info.Progress = math.Min(raw, 100)
		}
		info.ExpToNext = next.MinExp - totalExp
		if info.ExpToNext < 0 {
			info.ExpToNext = 0
		}
	} else {
		info.Progress = 100
	}
	return info
}

func resolveRank(ranks []*cultivationEntity.Rank, rating int) dto.RankInfo {
	if len(ranks) == 0 {
		return dto.RankInfo{Name: "—"}
	}
	sort.Slice(ranks, func(i, j int) bool { return ranks[i].MinRating < ranks[j].MinRating })

	idx := 0
	for i, r := range ranks {
		if r.MinRating <= rating {
			idx = i
		} else {
			break
		}
	}

	current := ranks[idx]
	info := dto.RankInfo{Name: current.Name, TierIndex: idx}

	if idx+1 < len(ranks) {
		next := ranks[idx+1]
		info.NextName = next.Name
		info.NextTierIndex = idx + 1
		diff := next.MinRating - current.MinRating
		if diff > 0 {
			raw := float64(rating-current.MinRating) / float64(diff) * 100
			info.Progress = math.Min(raw, 100)
		}
		info.RatingToNext = next.MinRating - rating
		if info.RatingToNext < 0 {
			info.RatingToNext = 0
		}
	} else {
		info.Progress = 100
	}
	return info
}

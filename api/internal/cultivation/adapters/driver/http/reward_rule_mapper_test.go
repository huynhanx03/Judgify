package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
)

func TestToRewardRuleResponseSerializesRuleID(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	response := toRewardRuleResponse(entity.RewardRuleRevision{
		ID: id, RuleKey: "solve-reward", Revision: 2,
		Configuration: entity.RewardRuleConfiguration{MultiplierBasisPoints: 10000},
		EffectiveFrom: time.Now().UTC(),
	}, true)

	require.Equal(t, id.String(), response.ID)
	require.True(t, response.Replayed)
}

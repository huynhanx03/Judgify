package http

import (
	"go/parser"
	"go/token"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLegacyPublicRegistrationRollRouteIsAbsent(t *testing.T) {
	source := parsedCultivationRoutes(t)
	require.NotContains(t, string(source), `traits.POST("/roll"`)
	require.NotContains(t, string(source), `traits.GET("/roll"`)
}

func TestUserTraitProjectionHasNoMutationRoutes(t *testing.T) {
	source := parsedCultivationRoutes(t)
	require.Contains(t, string(source), `userTraits.POST("/find"`)
	require.Contains(t, string(source), `userTraits.GET("/:id"`)
	require.NotContains(t, string(source), `userTraits.POST(""`)
	require.NotContains(t, string(source), `userTraits.PUT(`)
	require.NotContains(t, string(source), `userTraits.DELETE(`)
}

func TestRewardProfileRoutesExposeSelfReadAndGuardedAdministration(t *testing.T) {
	source := parsedCultivationRoutes(t)
	require.Contains(t, string(source), `rewardProfile.GET(""`)
	require.Contains(t, string(source), `"/reward-profiles/:user_id"`)
	require.Contains(t, string(source), `"/reward-profiles/:user_id/preview"`)
	require.Contains(t, string(source), `permissions.PermissionScopePublish`)
}

func parsedCultivationRoutes(t *testing.T) []byte {
	t.Helper()
	source, err := os.ReadFile("routes.go")
	require.NoError(t, err)
	_, err = parser.ParseFile(token.NewFileSet(), "routes.go", source, 0)
	require.NoError(t, err)
	return source
}

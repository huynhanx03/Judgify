package http

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestParseSessionListQueryRejectsUnknownAndAmbiguousParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []string{
		"/auth/sessions?unexpected=value",
		"/auth/sessions?limit=20&limit=21",
		"/auth/sessions?cursor=a&cursor=b",
		"/auth/sessions?limit=+20",
	}
	for _, target := range tests {
		t.Run(target, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest("GET", target, nil)
			_, err := parseSessionListQuery(context)
			require.Error(t, err)
		})
	}
}

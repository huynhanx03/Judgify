package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestParseMySubmissionsRequestAcceptsOnlyCanonicalCursorQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	problemID := uuid.MustParse("00000000-0000-7000-8000-000000000021")
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000031")
	context.Params = gin.Params{{Key: "id", Value: problemID.String()}}
	context.Request = httptest.NewRequest(
		http.MethodGet,
		"/problems/"+problemID.String()+"/my-submissions?limit=25&cursor=opaque&contest_id="+contestID.String(),
		nil,
	)

	request, err := parseMySubmissionsRequest(context)
	if err != nil {
		t.Fatalf("parseMySubmissionsRequest() error = %v", err)
	}
	if request.ProblemID != problemID || request.ContestID == nil ||
		*request.ContestID != contestID || request.Limit != 25 ||
		request.Cursor != "opaque" {
		t.Fatalf("request = %+v", request)
	}
}

func TestParseMySubmissionsRequestRejectsAmbiguousOrUnknownQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	problemID := uuid.MustParse("00000000-0000-7000-8000-000000000021")
	for _, rawQuery := range []string{
		"limit=10&limit=20",
		"cursor=",
		"cursor=one&cursor=two",
		"contest_id=",
		"contest_id=00000000-0000-7000-8000-000000000031&contest_id=00000000-0000-7000-8000-000000000032",
		"contest_id=not-a-uuid",
		"cursor=opaque;limit=20",
		"limit=01",
		"limit=0",
		"limit=51",
		"unknown=value",
	} {
		t.Run(rawQuery, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Params = gin.Params{{Key: "id", Value: problemID.String()}}
			context.Request = httptest.NewRequest(
				http.MethodGet,
				"/problems/"+problemID.String()+"/my-submissions?"+rawQuery,
				nil,
			)
			if _, err := parseMySubmissionsRequest(context); err == nil {
				t.Fatalf("query %q was accepted", rawQuery)
			}
		})
	}
}

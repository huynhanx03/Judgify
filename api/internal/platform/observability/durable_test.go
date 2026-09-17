package observability

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestOperationCollectorFoldsKindsAndStatesInsideBoundedQuery(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = database.Close() }()
	metrics := newTestMetrics(t)
	if err := metrics.registerOperationCollector(database, []string{"zeta.v1", "alpha.v1"}); err != nil {
		t.Fatalf("registerOperationCollector: %v", err)
	}
	mock.ExpectQuery(
		`(?s)SELECT\s+CASE WHEN kind IN \(\$1, \$2\) THEN kind ELSE 'other' END AS metric_kind,.*`+
			`WHERE status IN \('pending', 'running', 'paused', 'completed', 'cancelled', 'failed'\).*GROUP BY 1, 2`,
	).
		WithArgs("alpha.v1", "zeta.v1").
		WillReturnRows(sqlmock.NewRows([]string{"metric_kind", "metric_state", "count"}).
			AddRow("alpha.v1", "running", int64(2)).
			AddRow("other", "scheduled", int64(5)))

	if err := metrics.collectors[0].collector.Collect(context.Background()); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
	// The direct collection above populated gauges; avoid a second database
	// snapshot while inspecting their exposition.
	metrics.collectors = nil
	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := response.Body.String()
	for _, sample := range []string{
		`judgify_operations{kind="alpha.v1",state="running"} 2`,
		`judgify_operations{kind="other",state="scheduled"} 5`,
	} {
		if !strings.Contains(body, sample) {
			t.Fatalf("missing %q in metrics body", sample)
		}
	}
}

func TestOperationCollectorRejectsNonCatalogKind(t *testing.T) {
	database, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = database.Close() }()
	if err := newTestMetrics(t).registerOperationCollector(database, []string{"Bad Kind"}); !errors.Is(err, ErrInvalidMetrics) {
		t.Fatalf("registerOperationCollector error = %v, want ErrInvalidMetrics", err)
	}
}

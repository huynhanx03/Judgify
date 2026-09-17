package schema

import (
	"fmt"
	"testing"

	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"

	"github.com/huynhanx03/judgify/pkg/judging"
)

func TestJudgeContentSchemasUseCanonicalByteChecks(t *testing.T) {
	t.Parallel()

	submissionChecks := sqlChecks(t, Submission{}.Annotations())
	if got, want := submissionChecks["submission_source_code_bytes"], fmt.Sprintf(
		"octet_length(source_code) BETWEEN 1 AND %d", judging.MaximumSourceCodeBytes,
	); got != want {
		t.Fatalf("submission source check = %q, want %q", got, want)
	}

	assetChecks := sqlChecks(t, TestAsset{}.Annotations())
	if got, want := assetChecks["test_asset_content_bound"], fmt.Sprintf(
		"octet_length(content) BETWEEN 0 AND %d", judging.MaximumTestInputBytes,
	); got != want {
		t.Errorf("test asset content check = %q, want %q", got, want)
	}
}

func sqlChecks(t *testing.T, annotations []entschema.Annotation) map[string]string {
	t.Helper()
	for _, annotation := range annotations {
		if sqlAnnotation, ok := annotation.(*entsql.Annotation); ok {
			return sqlAnnotation.Checks
		}
	}
	t.Fatal("Ent SQL checks annotation is missing")
	return nil
}

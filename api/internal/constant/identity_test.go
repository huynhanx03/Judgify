package constant

import "testing"

func TestIdentityRequiredSystemAttributeCatalogIsDefensiveCopy(t *testing.T) {
	attributes := RequiredSystemAttributeCatalog()
	if len(attributes) != 4 {
		t.Fatalf("catalog length = %d, want 4", len(attributes))
	}
	if attributes[0].Key != AttributeKeyLastName {
		t.Fatalf("first attribute key = %q, want %q", attributes[0].Key, AttributeKeyLastName)
	}

	attributes[0].Key = "mutated"
	attribute, ok := RequiredSystemAttribute(AttributeKeyLastName)
	if !ok || attribute.Key != AttributeKeyLastName {
		t.Fatalf("catalog mutation leaked into required attribute: %#v, found=%t", attribute, ok)
	}
}

func TestProblemConstantsRemainDomainScoped(t *testing.T) {
	if ProblemObjProblem != "problem" || ProblemObjDifficulty != "difficulty" {
		t.Fatalf("problem object constants changed: %q / %q", ProblemObjProblem, ProblemObjDifficulty)
	}
	if ProblemCacheTTL <= 0 {
		t.Fatalf("problem cache TTL = %s, want positive", ProblemCacheTTL)
	}
}

func TestMaterialConstantsRemainDomainScoped(t *testing.T) {
	if MaterialStatusPublished != "published" || MaterialVisibilityPublic != "public" {
		t.Fatalf("material constants changed: %q / %q", MaterialStatusPublished, MaterialVisibilityPublic)
	}
	if MaterialCacheTTL <= 0 {
		t.Fatalf("material cache TTL = %s, want positive", MaterialCacheTTL)
	}
}

func TestSubmissionConstantsRemainDomainScoped(t *testing.T) {
	if SubmissionObjSubmission != "submission" || SubmissionObjProblem != "problem" {
		t.Fatalf("submission object constants changed: %q / %q", SubmissionObjSubmission, SubmissionObjProblem)
	}
}

func TestContestConstantsRemainDomainScoped(t *testing.T) {
	if ContestStatusRunning != "running" || ContestObjContest != "contest" {
		t.Fatalf("contest constants changed: %q / %q", ContestStatusRunning, ContestObjContest)
	}
	if ContestOrchestratorTickInterval <= 0 {
		t.Fatalf("contest orchestrator interval = %s, want positive", ContestOrchestratorTickInterval)
	}
}

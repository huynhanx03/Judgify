package judge

import (
	"errors"
	"testing"

	"github.com/huynhanx03/judgify/internal/judge/checker"
	"github.com/huynhanx03/judgify/internal/judge/supervisor"
	submissionentity "github.com/huynhanx03/judgify/internal/submission/domain"
)

func TestClassifyUsesTrustedPrecedence(t *testing.T) {
	t.Parallel()

	base := supervisor.ExecuteResult{
		ContainerID:        "container",
		CgroupID:           "cgroup",
		ExitCode:           0,
		AccountingComplete: true,
		CleanupComplete:    true,
	}
	for _, test := range []struct {
		name    string
		mutate  func(*supervisor.ExecuteResult)
		checked checker.Result
		want    submissionentity.Verdict
	}{
		{
			name: "memory wins over time and exit",
			mutate: func(result *supervisor.ExecuteResult) {
				result.OOMKilled = true
				result.CPUTimeLimitExceeded = true
				result.ExitCode = 137
			},
			checked: checker.Result{Accepted: true, Code: "accepted"},
			want:    submissionentity.VerdictMemoryLimitExceeded,
		},
		{
			name: "process wins over output and time",
			mutate: func(result *supervisor.ExecuteResult) {
				result.ProcessLimitExceeded = true
				result.OutputLimitExceeded = true
				result.WallTimeLimitExceeded = true
			},
			checked: checker.Result{Accepted: true, Code: "accepted"},
			want:    submissionentity.VerdictProcessLimitExceeded,
		},
		{
			name: "output wins over time and exit",
			mutate: func(result *supervisor.ExecuteResult) {
				result.OutputLimitExceeded = true
				result.WallTimeLimitExceeded = true
				result.ExitCode = 1
			},
			checked: checker.Result{Accepted: true, Code: "accepted"},
			want:    submissionentity.VerdictOutputLimitExceeded,
		},
		{
			name: "CPU limit precedes runtime exit",
			mutate: func(result *supervisor.ExecuteResult) {
				result.CPUTimeLimitExceeded = true
				result.ExitCode = 1
			},
			checked: checker.Result{Accepted: true, Code: "accepted"},
			want:    submissionentity.VerdictTimeLimitExceeded,
		},
		{
			name: "wall limit precedes runtime exit",
			mutate: func(result *supervisor.ExecuteResult) {
				result.WallTimeLimitExceeded = true
				result.ExitCode = 1
			},
			checked: checker.Result{Accepted: true, Code: "accepted"},
			want:    submissionentity.VerdictTimeLimitExceeded,
		},
		{
			name: "nonzero exit is runtime error",
			mutate: func(result *supervisor.ExecuteResult) {
				result.ExitCode = 2
			},
			checked: checker.Result{Accepted: true, Code: "accepted"},
			want:    submissionentity.VerdictRuntimeError,
		},
		{
			name: "signal is runtime error",
			mutate: func(result *supervisor.ExecuteResult) {
				result.Signal = "SIGSEGV"
			},
			checked: checker.Result{Accepted: true, Code: "accepted"},
			want:    submissionentity.VerdictRuntimeError,
		},
		{
			name:    "checker accepts only after clean zero exit",
			checked: checker.Result{Accepted: true, Code: "accepted"},
			want:    submissionentity.VerdictAccepted,
		},
		{
			name:    "checker rejects only after clean zero exit",
			checked: checker.Result{Accepted: false, Code: "token_mismatch"},
			want:    submissionentity.VerdictWrongAnswer,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := base
			if test.mutate != nil {
				test.mutate(&result)
			}
			got, err := Classify(result, test.checked)
			if err != nil {
				t.Fatalf("Classify() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("Classify() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestClassifyFailsClosedOnUntrustedOrContradictoryEvidence(t *testing.T) {
	t.Parallel()

	valid := supervisor.ExecuteResult{
		ContainerID:        "container",
		CgroupID:           "cgroup",
		ExitCode:           0,
		AccountingComplete: true,
		CleanupComplete:    true,
	}
	for _, mutate := range []func(*supervisor.ExecuteResult){
		func(result *supervisor.ExecuteResult) { result.ContainerID = "" },
		func(result *supervisor.ExecuteResult) { result.CgroupID = "" },
		func(result *supervisor.ExecuteResult) { result.AccountingComplete = false },
		func(result *supervisor.ExecuteResult) { result.CleanupComplete = false },
		func(result *supervisor.ExecuteResult) { result.ExitCode = -1 },
		func(result *supervisor.ExecuteResult) {
			result.StdoutTruncated = true
			result.OutputLimitExceeded = false
		},
	} {
		result := valid
		mutate(&result)
		verdict, err := Classify(result, checker.Result{Accepted: true, Code: "accepted"})
		if verdict != submissionentity.VerdictSystemError ||
			!errors.Is(err, ErrUntrustedExecution) {
			t.Fatalf("Classify(%+v) = (%q, %v), want system_error/ErrUntrustedExecution", result, verdict, err)
		}
	}
}

func TestClassifyRejectsMalformedCheckerResult(t *testing.T) {
	t.Parallel()

	execution := supervisor.ExecuteResult{
		ContainerID:        "container",
		CgroupID:           "cgroup",
		ExitCode:           0,
		AccountingComplete: true,
		CleanupComplete:    true,
	}
	for _, checked := range []checker.Result{
		{},
		{Accepted: true, Code: "token_mismatch"},
		{Accepted: false, Code: "accepted"},
		{Accepted: false, Code: "contains whitespace"},
	} {
		verdict, err := Classify(execution, checked)
		if verdict != submissionentity.VerdictSystemError ||
			!errors.Is(err, ErrUntrustedCheckerResult) {
			t.Fatalf("Classify(%+v) = (%q, %v), want system_error/ErrUntrustedCheckerResult", checked, verdict, err)
		}
	}
}

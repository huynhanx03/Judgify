package judge

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/huynhanx03/judgify/internal/judge/checker"
	"github.com/huynhanx03/judgify/internal/judge/supervisor"
	submissionentity "github.com/huynhanx03/judgify/internal/submission/domain"
)

var (
	ErrUntrustedExecution     = errors.New("judge: untrusted execution evidence")
	ErrUntrustedCheckerResult = errors.New("judge: untrusted checker result")
)

// Classify applies the immutable verdict precedence to trusted host evidence.
// Infrastructure ambiguity returns system_error plus an error so the caller
// can apply the durable retry budget instead of publishing a contestant result.
func Classify(
	execution supervisor.ExecuteResult,
	checked checker.Result,
) (submissionentity.Verdict, error) {
	if !validTrustedExecution(execution) {
		return submissionentity.VerdictSystemError, ErrUntrustedExecution
	}
	if execution.OOMKilled || execution.MemoryLimitExceeded {
		return submissionentity.VerdictMemoryLimitExceeded, nil
	}
	if execution.ProcessLimitExceeded {
		return submissionentity.VerdictProcessLimitExceeded, nil
	}
	if execution.OutputLimitExceeded {
		return submissionentity.VerdictOutputLimitExceeded, nil
	}
	if execution.CPUTimeLimitExceeded || execution.WallTimeLimitExceeded {
		return submissionentity.VerdictTimeLimitExceeded, nil
	}
	if execution.ExitCode != 0 || execution.Signal != "" {
		return submissionentity.VerdictRuntimeError, nil
	}
	if !validCheckerResult(checked) {
		return submissionentity.VerdictSystemError, ErrUntrustedCheckerResult
	}
	if checked.Accepted {
		return submissionentity.VerdictAccepted, nil
	}
	return submissionentity.VerdictWrongAnswer, nil
}

func validTrustedExecution(result supervisor.ExecuteResult) bool {
	if result.ContainerID == "" || result.CgroupID == "" ||
		result.ExitCode < 0 || !result.AccountingComplete || !result.CleanupComplete ||
		result.WallTime < 0 || result.CPUTime < 0 || result.PeakMemoryBytes < 0 {
		return false
	}
	if (result.StdoutTruncated || result.StderrTruncated) && !result.OutputLimitExceeded {
		return false
	}
	return true
}

func validCheckerResult(result checker.Result) bool {
	if result.Code == "" || len(result.Code) > 64 || !utf8.ValidString(result.Code) ||
		result.Diagnostic != "" {
		return false
	}
	for _, character := range result.Code {
		if unicode.IsControl(character) || unicode.IsSpace(character) {
			return false
		}
	}
	if result.Accepted {
		return result.Code == "accepted"
	}
	return result.Code != "accepted" && strings.HasSuffix(result.Code, "_mismatch")
}

// Package contract contains the closed, dependency-free authored-content
// values shared by the renderer and its database schema.
package contract

const (
	SourceKindProblemStatement         = "problem_statement"
	SourceKindContestDescription       = "contest_description"
	SourceKindContestAnnouncement      = "contest_announcement"
	SourceKindContestClarification     = "contest_clarification"
	SourceKindMaterialBody             = "material_body"
	SourceKindNotificationCampaignBody = "notification_campaign_body"

	StatusReady    = "ready"
	StatusRejected = "rejected"

	DiagnosticInvalidUTF8       = "invalid_utf8"
	DiagnosticAmbiguousEncoding = "ambiguous_encoding"
	DiagnosticSourceTooLarge    = "source_too_large"
	DiagnosticOutputTooLarge    = "output_too_large"
	DiagnosticNestingTooDeep    = "nesting_too_deep"
	DiagnosticRenderRejected    = "render_rejected"

	MaximumPipelineVersionBytes = 64
	MaximumRenderedHTMLBytes    = 2 << 20
	MaximumDiagnostics          = 8
	MaximumDiagnosticsJSONBytes = 2048
)

var sourceKinds = [...]string{
	SourceKindProblemStatement,
	SourceKindContestDescription,
	SourceKindContestAnnouncement,
	SourceKindContestClarification,
	SourceKindMaterialBody,
	SourceKindNotificationCampaignBody,
}

var diagnosticCodes = [...]string{
	DiagnosticInvalidUTF8,
	DiagnosticAmbiguousEncoding,
	DiagnosticSourceTooLarge,
	DiagnosticOutputTooLarge,
	DiagnosticNestingTooDeep,
	DiagnosticRenderRejected,
}

var statuses = [...]string{
	StatusReady,
	StatusRejected,
}

// SourceKinds returns a copy so callers cannot widen the closed contract.
func SourceKinds() []string {
	return append([]string(nil), sourceKinds[:]...)
}

// DiagnosticCodes returns a copy in the stable storage order.
func DiagnosticCodes() []string {
	return append([]string(nil), diagnosticCodes[:]...)
}

// Statuses returns a copy so persistence adapters share the same closed set.
func Statuses() []string {
	return append([]string(nil), statuses[:]...)
}

func IsSourceKind(value string) bool {
	return contains(sourceKinds[:], value)
}

func IsDiagnosticCode(value string) bool {
	return contains(diagnosticCodes[:], value)
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

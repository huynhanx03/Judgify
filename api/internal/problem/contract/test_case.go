package contract

// SampleTestCaseResult is the public application projection for a visible example.
// It intentionally omits internal identifiers and the hidden marker.
type SampleTestCaseResult struct {
	Input          string
	ExpectedOutput string
	OrderIndex     int
}

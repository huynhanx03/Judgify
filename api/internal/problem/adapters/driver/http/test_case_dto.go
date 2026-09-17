package http

// publicSamplesRequest is the HTTP path input for public problem samples.
type publicSamplesRequest struct {
	ProblemID string `uri:"id" validate:"required,uuid"`
}

// SampleTestCaseResponse is the HTTP representation of a visible testcase.
type SampleTestCaseResponse struct {
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	OrderIndex     int    `json:"order_index"`
}

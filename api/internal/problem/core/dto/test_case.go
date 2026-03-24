package dto

// CreateTestCaseRequest represents request to create a test case.
type CreateTestCaseRequest struct {
	ProblemID      int    `json:"-" uri:"id"`
	Input          string `json:"input" validate:"required"`
	ExpectedOutput string `json:"expected_output" validate:"required"`
	IsHidden       *bool  `json:"is_hidden"`
	OrderIndex     int    `json:"order_index" validate:"min=0"`
}

// UpdateTestCaseRequest represents request to update a test case.
type UpdateTestCaseRequest struct {
	ID             int     `json:"-" uri:"id"`
	Input          *string `json:"input" validate:"omitempty"`
	ExpectedOutput *string `json:"expected_output" validate:"omitempty"`
	IsHidden       *bool   `json:"is_hidden" validate:"omitempty"`
	OrderIndex     *int    `json:"order_index" validate:"omitempty,min=0"`
}

// GetTestCaseRequest represents request to get test cases by problem ID.
type GetTestCaseRequest struct {
	ProblemID int `uri:"id" validate:"required"`
}

// DeleteTestCaseRequest represents request to delete a test case.
type DeleteTestCaseRequest struct {
	ID int `uri:"id" validate:"required"`
}

// TestCaseResponse represents test case data in API response.
type TestCaseResponse struct {
	ID             int    `json:"id"`
	ProblemID      int    `json:"problem_id"`
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	IsHidden       bool   `json:"is_hidden"`
	OrderIndex     int    `json:"order_index"`
}

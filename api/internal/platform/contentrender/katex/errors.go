package katex

import "errors"

// Public errors are deliberately context-free. In particular, errors returned
// by the JavaScript runtime and KaTeX can contain the submitted expression, so
// they must never cross this package boundary.
var (
	ErrInvalidConfiguration = errors.New("katex: invalid configuration")
	ErrInvalidEngine        = errors.New("katex: invalid engine")
	ErrInvalidContext       = errors.New("katex: invalid context")
	ErrAssetIntegrity       = errors.New("katex: asset integrity check failed")
	ErrVersionMismatch      = errors.New("katex: runtime version mismatch")
	ErrInputTooLarge        = errors.New("katex: input limit exceeded")
	ErrOutputTooLarge       = errors.New("katex: output limit exceeded")
	ErrForbiddenCommand     = errors.New("katex: forbidden command")
	ErrInvalidExpression    = errors.New("katex: invalid expression")
	ErrUnsafeOutput         = errors.New("katex: unsafe rendered output")
	ErrInternal             = errors.New("katex: internal rendering failure")
)

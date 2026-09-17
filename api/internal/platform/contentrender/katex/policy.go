package katex

import "unicode/utf8"

const (
	// MaximumInputBytes is the hard per-expression work boundary.
	MaximumInputBytes = 4 << 10

	// MaximumOutputBytes prevents an expansion from becoming a large retained
	// Go allocation even after KaTeX's own expansion limits have accepted it.
	MaximumOutputBytes = 1 << 20
)

func validateExpression(expression []byte) error {
	if len(expression) == 0 {
		return ErrInvalidExpression
	}
	// Reject on length before doing any content-dependent scan.
	if len(expression) > MaximumInputBytes {
		return ErrInputTooLarge
	}
	if !utf8.Valid(expression) {
		return ErrInvalidExpression
	}
	if containsForbiddenCommand(expression) {
		return ErrForbiddenCommand
	}
	return nil
}

// containsForbiddenCommand tokenizes TeX control words instead of performing a
// substring search. For example, \\href is the line-break control symbol
// followed by plain text and \hrefExtra is a distinct control word.
func containsForbiddenCommand(expression []byte) bool {
	for index := 0; index < len(expression); {
		if expression[index] != '\\' {
			index++
			continue
		}

		index++
		if index >= len(expression) {
			return false
		}
		if !isKaTeXControlWordCharacter(expression[index]) {
			// A backslash followed by a non-letter is one TeX control symbol.
			index++
			continue
		}

		start := index
		for index < len(expression) && isKaTeXControlWordCharacter(expression[index]) {
			index++
		}
		if isForbiddenControlWord(expression[start:index]) {
			return true
		}
	}
	return false
}

func isForbiddenControlWord(value []byte) bool {
	// KaTeX treats @ as a control-word character for private implementation
	// macros. Authored input never needs to invoke those internals directly.
	for _, character := range value {
		if character == '@' {
			return true
		}
	}

	switch string(value) {
	case "href",
		"url",
		"includegraphics",
		"htmlClass",
		"htmlId",
		"htmlStyle",
		"htmlData",
		// Macro-definition and alias primitives can turn a bounded source into
		// a multi-megabyte transient JavaScript string before the Go-side
		// output limit can observe it. User-defined macros are not part of the
		// authored-content dialect.
		"def",
		"gdef",
		"edef",
		"xdef",
		"newcommand",
		"renewcommand",
		"providecommand",
		"let",
		"futurelet",
		"global",
		"long",
		// Raw box shifts preserve arbitrary TeX units in MathML instead of
		// passing through KaTeX's maxSize conversion. Internal pinned KaTeX
		// macros remain usable because this scan covers authored input only.
		"raisebox",
		// Nested sizing controls multiply MathML mathsize values. A short
		// expression could otherwise create exponential visual amplification
		// even when every individual attribute is numerically bounded.
		"tiny",
		"sixptsize",
		"scriptsize",
		"footnotesize",
		"small",
		"normalsize",
		"large",
		"Large",
		"LARGE",
		"huge",
		"Huge",
		"pmb",
		"fcolorbox":
		return true
	default:
		return false
	}
}

func isASCIIAlpha(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func isKaTeXControlWordCharacter(value byte) bool {
	return isASCIIAlpha(value) || value == '@'
}

package contract

// MaxJSONSafeInteger is the largest integer that round-trips exactly through
// JSON.parse and JavaScript's number representation (2^53 - 1).
const MaxJSONSafeInteger int64 = 9_007_199_254_740_991

// IsJSONSafeNonNegativeInteger reports whether an API integer can cross the
// Go/JSON/JavaScript boundary without sign or precision loss.
func IsJSONSafeNonNegativeInteger(value int64) bool {
	return value >= 0 && value <= MaxJSONSafeInteger
}

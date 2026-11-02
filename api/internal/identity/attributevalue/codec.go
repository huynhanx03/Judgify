// Package attributevalue owns the closed, typed JSON representation used by
// revisioned profile values. It deliberately never coerces an invalid value:
// callers can retain its source row and record only an opaque quarantine
// reference instead.
package attributevalue

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

const (
	MaxCanonicalBytes = 8 << 10
	// Profile numbers cross Go's encoding/json and ECMAScript JSON.parse. Keep
	// the persisted domain to values for which both runtimes produce the same
	// exponent-free token; integral values additionally use JavaScript's exact
	// safe-integer range.
	minimumInteroperableNumberMagnitude = 1e-6
	maximumInteroperableNumberMagnitude = 1e21
	maximumInteroperableInteger         = 9_007_199_254_740_991
)

var (
	ErrInvalidValue      = errors.New("identity attribute value: invalid value")
	ErrInvalidValidation = errors.New("identity attribute value: invalid validation")
	numberPattern        = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
)

// Canonicalize validates raw JSON against revision and returns its unique
// compact representation. Number exponents are intentionally rejected: the
// source representation would otherwise have several textual encodings for
// the same value and make source-version drift difficult to audit.
func Canonicalize(raw json.RawMessage, revision entity.AttributeDefinitionRevision) (json.RawMessage, error) {
	if !revision.DataType.Valid() || len(raw) == 0 || len(raw) > MaxCanonicalBytes {
		return nil, ErrInvalidValue
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, errors.Join(ErrInvalidValue, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, ErrInvalidValue
	}

	validation, err := parseValidation(revision.Validation)
	if err == nil {
		err = validation.validateFor(revision.DataType)
	}
	if err != nil {
		return nil, err
	}
	canonical, err := canonicalByType(value, revision.DataType)
	if err != nil {
		return nil, err
	}
	if len(canonical) == 0 || len(canonical) > MaxCanonicalBytes {
		return nil, ErrInvalidValue
	}
	if err := validation.validate(value, canonical, revision.DataType); err != nil {
		return nil, err
	}
	return canonical, nil
}

// ValidateRevision verifies that a revision's validation document is both
// structurally valid and meaningful for its declared value type. Keeping this
// check in the codec prevents lifecycle code from activating a schema that no
// value could ever satisfy merely because the definition currently has no
// rows to migrate.
func ValidateRevision(revision entity.AttributeDefinitionRevision) error {
	if !revision.DataType.Valid() {
		return ErrInvalidValidation
	}
	spec, err := parseValidation(revision.Validation)
	if err != nil {
		return err
	}
	return spec.validateFor(revision.DataType)
}

// CanonicalizeLegacy is an explicit, narrowly-scoped conversion path for a
// legacy text projection. It is used only when a revisioned canonical source
// value is absent. The conversion is deterministic; failed parsing is never
// replaced with a default or an approximated value.
func CanonicalizeLegacy(value string, revision entity.AttributeDefinitionRevision) (json.RawMessage, error) {
	var raw json.RawMessage
	switch revision.DataType {
	case entity.AttributeValueTypeString, entity.AttributeValueTypeDate:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, errors.Join(ErrInvalidValue, err)
		}
		raw = encoded
	case entity.AttributeValueTypeNumber, entity.AttributeValueTypeBoolean:
		raw = json.RawMessage(value)
	default:
		return nil, ErrInvalidValue
	}
	return Canonicalize(raw, revision)
}

func canonicalByType(value any, dataType entity.AttributeValueType) (json.RawMessage, error) {
	switch dataType {
	case entity.AttributeValueTypeString:
		text, ok := value.(string)
		if !ok || !utf8.ValidString(text) || len(text) > MaxCanonicalBytes {
			return nil, ErrInvalidValue
		}
		return json.Marshal(text)
	case entity.AttributeValueTypeBoolean:
		boolean, ok := value.(bool)
		if !ok {
			return nil, ErrInvalidValue
		}
		return json.Marshal(boolean)
	case entity.AttributeValueTypeDate:
		date, ok := value.(string)
		if !ok || len(date) != len(time.DateOnly) {
			return nil, ErrInvalidValue
		}
		parsed, err := time.Parse(time.DateOnly, date)
		if err != nil || parsed.Format(time.DateOnly) != date {
			return nil, ErrInvalidValue
		}
		return json.Marshal(date)
	case entity.AttributeValueTypeNumber:
		number, ok := value.(json.Number)
		if !ok {
			return nil, ErrInvalidValue
		}
		return canonicalNumber(number.String())
	default:
		return nil, ErrInvalidValue
	}
}

func canonicalNumber(value string) (json.RawMessage, error) {
	if len(value) == 0 || len(value) > MaxCanonicalBytes || !numberPattern.MatchString(value) {
		return nil, ErrInvalidValue
	}
	negative := strings.HasPrefix(value, "-")
	unsigned := strings.TrimPrefix(value, "-")
	integer, fraction, hasFraction := strings.Cut(unsigned, ".")
	if hasFraction {
		fraction = strings.TrimRight(fraction, "0")
	}
	integer = strings.TrimLeft(integer, "0")
	if integer == "" {
		integer = "0"
	}
	if fraction == "" {
		hasFraction = false
	}
	if integer == "0" && !hasFraction {
		negative = false
	}
	canonical := integer
	if hasFraction {
		canonical += "." + fraction
	}
	if negative {
		canonical = "-" + canonical
	}
	parsed, err := strconv.ParseFloat(canonical, 64)
	magnitude := math.Abs(parsed)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) ||
		(parsed != 0 && (magnitude < minimumInteroperableNumberMagnitude ||
			magnitude >= maximumInteroperableNumberMagnitude)) ||
		(math.Trunc(parsed) == parsed && magnitude > maximumInteroperableInteger) ||
		strconv.FormatFloat(parsed, 'f', -1, 64) != canonical {
		return nil, ErrInvalidValue
	}
	return json.RawMessage(canonical), nil
}

type validationSpec struct {
	Enum      []json.RawMessage
	MinLength *int
	MaxLength *int
	Pattern   *regexp.Regexp
	Minimum   *float64
	Maximum   *float64
}

func (spec validationSpec) validFor(dataType entity.AttributeValueType) bool {
	switch dataType {
	case entity.AttributeValueTypeString:
		return spec.Minimum == nil && spec.Maximum == nil
	case entity.AttributeValueTypeNumber:
		return spec.MinLength == nil && spec.MaxLength == nil && spec.Pattern == nil
	case entity.AttributeValueTypeBoolean, entity.AttributeValueTypeDate:
		return spec.MinLength == nil && spec.MaxLength == nil && spec.Pattern == nil &&
			spec.Minimum == nil && spec.Maximum == nil
	default:
		return false
	}
}

func (spec validationSpec) validateFor(dataType entity.AttributeValueType) error {
	if !spec.validFor(dataType) {
		return ErrInvalidValidation
	}
	seen := make(map[string]struct{}, len(spec.Enum))
	for _, raw := range spec.Enum {
		value := decodeEnumValue(raw)
		canonical, err := canonicalByType(value, dataType)
		if err != nil || !bytes.Equal(bytes.TrimSpace(raw), canonical) {
			return ErrInvalidValidation
		}
		key := string(canonical)
		if _, duplicate := seen[key]; duplicate {
			return ErrInvalidValidation
		}
		seen[key] = struct{}{}
	}
	return nil
}

func parseValidation(raw json.RawMessage) (validationSpec, error) {
	if len(raw) == 0 || len(raw) > MaxCanonicalBytes {
		return validationSpec{}, ErrInvalidValidation
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(raw, &document); err != nil || document == nil {
		return validationSpec{}, ErrInvalidValidation
	}
	result := validationSpec{}
	for key, value := range document {
		switch key {
		case "enum":
			if len(value) == 0 || value[0] != '[' ||
				json.Unmarshal(value, &result.Enum) != nil || len(result.Enum) > 256 {
				return validationSpec{}, ErrInvalidValidation
			}
		case "min_length", "max_length":
			var amount int
			if err := json.Unmarshal(value, &amount); err != nil || amount < 0 || amount > MaxCanonicalBytes {
				return validationSpec{}, ErrInvalidValidation
			}
			if key == "min_length" {
				result.MinLength = &amount
			} else {
				result.MaxLength = &amount
			}
		case "pattern":
			var pattern string
			if err := json.Unmarshal(value, &pattern); err != nil || len(pattern) == 0 || len(pattern) > 1024 {
				return validationSpec{}, ErrInvalidValidation
			}
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				return validationSpec{}, ErrInvalidValidation
			}
			result.Pattern = compiled
		case "minimum", "maximum":
			canonical, err := canonicalNumber(string(bytes.TrimSpace(value)))
			if err != nil || !bytes.Equal(canonical, bytes.TrimSpace(value)) {
				return validationSpec{}, ErrInvalidValidation
			}
			number, err := strconv.ParseFloat(string(canonical), 64)
			if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
				return validationSpec{}, ErrInvalidValidation
			}
			if key == "minimum" {
				result.Minimum = &number
			} else {
				result.Maximum = &number
			}
		default:
			return validationSpec{}, ErrInvalidValidation
		}
	}
	if result.MinLength != nil && result.MaxLength != nil && *result.MinLength > *result.MaxLength {
		return validationSpec{}, ErrInvalidValidation
	}
	if result.Minimum != nil && result.Maximum != nil && *result.Minimum > *result.Maximum {
		return validationSpec{}, ErrInvalidValidation
	}
	return result, nil
}

func (spec validationSpec) validate(value any, canonical json.RawMessage, dataType entity.AttributeValueType) error {
	if len(spec.Enum) > 0 {
		found := false
		for _, allowed := range spec.Enum {
			candidate, err := canonicalByType(decodeEnumValue(allowed), dataType)
			if err == nil && bytes.Equal(candidate, canonical) {
				found = true
				break
			}
		}
		if !found {
			return ErrInvalidValue
		}
	}
	if text, ok := value.(string); ok {
		length := utf8.RuneCountInString(text)
		if spec.MinLength != nil && length < *spec.MinLength ||
			spec.MaxLength != nil && length > *spec.MaxLength ||
			spec.Pattern != nil && !spec.Pattern.MatchString(text) {
			return ErrInvalidValue
		}
	}
	if number, ok := value.(json.Number); ok && (spec.Minimum != nil || spec.Maximum != nil) {
		parsed, err := strconv.ParseFloat(number.String(), 64)
		if err != nil || (spec.Minimum != nil && parsed < *spec.Minimum) ||
			(spec.Maximum != nil && parsed > *spec.Maximum) {
			return ErrInvalidValue
		}
	}
	return nil
}

func decodeEnumValue(raw json.RawMessage) any {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil
	}
	return value
}

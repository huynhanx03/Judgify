package attributevalue

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

// LegacyProjection converts a canonical typed value into the temporary text
// projection consumed by fixed profile fields. It verifies the value is in
// canonical form first; callers must not use it as a permissive coercion path.
func LegacyProjection(
	canonical json.RawMessage,
	dataType entity.AttributeValueType,
) (string, error) {
	if len(canonical) == 0 || len(canonical) > MaxCanonicalBytes || !dataType.Valid() {
		return "", ErrInvalidValue
	}
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", errors.Join(ErrInvalidValue, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return "", ErrInvalidValue
	}
	verified, err := canonicalByType(value, dataType)
	if err != nil || !bytes.Equal(verified, canonical) {
		return "", ErrInvalidValue
	}
	switch dataType {
	case entity.AttributeValueTypeString, entity.AttributeValueTypeDate:
		text, ok := value.(string)
		if !ok {
			return "", ErrInvalidValue
		}
		return text, nil
	case entity.AttributeValueTypeNumber, entity.AttributeValueTypeBoolean:
		return string(canonical), nil
	default:
		return "", ErrInvalidValue
	}
}

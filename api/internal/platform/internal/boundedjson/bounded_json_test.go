package boundedjson

import (
	"errors"
	"fmt"
	"testing"
)

func TestMarshalMapRejectsOversizedContainerBeforeTraversal(t *testing.T) {
	value := make(map[string]any, 128)
	for index := range 128 {
		value[fmt.Sprintf("field_%03d", index)] = index
	}
	redactorCalls := 0
	_, err := MarshalMap(value, Limits{
		MaxBytes: 1 << 20,
		MaxDepth: 8,
		MaxNodes: 8,
	}, func(string) bool {
		redactorCalls++
		return false
	})
	if !errors.Is(err, ErrTooComplex) {
		t.Fatalf("MarshalMap error = %v, want ErrTooComplex", err)
	}
	if redactorCalls != 0 {
		t.Fatalf("redactor calls = %d, want preflight rejection", redactorCalls)
	}
}

package driver_test

import (
	"testing"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
)

type conversionVertex struct {
	Rank     string   `gremlin:"rank"`
	Tag      string   `gremlin:"tag"`
	Weights  []string `gremlin:"weights"`
	Score    *string  `gremlin:"score"`
	Quantity int      `gremlin:"quantity"`
	Ratio    float64  `gremlin:"ratio"`
}

// Gremlin backends return integers as int32/int64. Go's reflect package treats
// integer -> string as a convertible pair, but the conversion is a rune
// mapping: int32(42) becomes "*" and int64(2026) becomes "ߪ", not "42"/"2026".
// Loading an integer property into a string field must skip the field like
// any other mismatch instead of silently writing corrupted text.
func TestIntegerPropertyDoesNotConvertToStringField(t *testing.T) {
	t.Parallel()
	t.Run(
		"Int32LeavesStringFieldUntouched", func(t *testing.T) {
			t.Parallel()
			var v conversionVertex
			err := driver.UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{Data: map[any]any{"rank": int32(42)}},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if v.Rank != "" {
				t.Errorf("Rank should be left as the zero value, got %q", v.Rank)
			}
		},
	)
	t.Run(
		"Int64LeavesStringFieldUntouched", func(t *testing.T) {
			t.Parallel()
			var v conversionVertex
			err := driver.UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{Data: map[any]any{"rank": int64(2026)}},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if v.Rank != "" {
				t.Errorf("Rank should be left as the zero value, got %q", v.Rank)
			}
		},
	)
	t.Run(
		"UnsignedIntegersLeaveStringFieldUntouched", func(t *testing.T) {
			t.Parallel()
			var v conversionVertex
			err := driver.UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{Data: map[any]any{"rank": uint64(65)}},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if v.Rank != "" {
				t.Errorf("Rank should be left as the zero value, got %q", v.Rank)
			}
			var w conversionVertex
			err = driver.UnloadGremlinResultIntoStruct(
				&w, &gremlingo.Result{Data: map[any]any{"rank": uintptr(66)}},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if w.Rank != "" {
				t.Errorf("Rank should be left as the zero value, got %q", w.Rank)
			}
		},
	)
	t.Run(
		"IntegerSliceDoesNotNarrowIntoStringField", func(t *testing.T) {
			t.Parallel()
			v := conversionVertex{Rank: "unchanged"}
			err := driver.UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{Data: map[any]any{"rank": []any{int32(1), int32(2)}}},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if v.Rank != "unchanged" {
				t.Errorf("Rank should keep its existing value, got %q", v.Rank)
			}
		},
	)
	t.Run(
		"IntegerDoesNotWrapIntoStringSlice", func(t *testing.T) {
			t.Parallel()
			var v conversionVertex
			err := driver.UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{Data: map[any]any{"weights": int64(7)}},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if v.Weights != nil {
				t.Errorf("Weights should be left nil, got %v", v.Weights)
			}
		},
	)
	t.Run(
		"IntegerElementsDoNotEnterStringSlice", func(t *testing.T) {
			t.Parallel()
			var v conversionVertex
			err := driver.UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{Data: map[any]any{"weights": []any{int64(7), "eight"}}},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			for _, w := range v.Weights {
				if w == "*" || w == "\a" {
					t.Errorf("Weights should skip integer elements, got %v", v.Weights)
				}
			}
		},
	)
	t.Run(
		"IntegerLeavesPointerStringFieldNil", func(t *testing.T) {
			t.Parallel()
			var v conversionVertex
			err := driver.UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{Data: map[any]any{"score": int64(5)}},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if v.Score != nil {
				t.Errorf("Score should stay nil, got %q", *v.Score)
			}
		},
	)
	t.Run(
		"NumericAndByteConversionsStillWork", func(t *testing.T) {
			t.Parallel()
			var v conversionVertex
			err := driver.UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{Data: map[any]any{
					"quantity": int32(7),
					"ratio":    int64(3),
					"tag":      []byte("hello"),
				}},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if v.Quantity != 7 {
				t.Errorf("Quantity should be 7, got %d", v.Quantity)
			}
			if v.Ratio != 3 {
				t.Errorf("Ratio should be 3, got %v", v.Ratio)
			}
			if v.Tag != "hello" {
				t.Errorf("Tag should be hello, got %q", v.Tag)
			}
		},
	)
}

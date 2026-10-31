package driver_test

import (
	"reflect"
	"testing"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
)

type conversionVertex struct {
	Rank       string    `gremlin:"rank"`
	Tag        string    `gremlin:"tag"`
	Weights    []string  `gremlin:"weights"`
	PtrWeights []*string `gremlin:"ptrWeights"`
	Score      *string   `gremlin:"score"`
	Quantity   int       `gremlin:"quantity"`
	Ratio      float64   `gremlin:"ratio"`
	Ports      []int32   `gremlin:"ports"`
	AnyValue   any       `gremlin:"anyValue"`
	Fixed      [4]byte   `gremlin:"fixed"`
}

// Gremlin backends return integers as int32/int64. Go's reflect package treats
// integer -> string as a convertible pair, but the conversion is a rune
// mapping: int32(42) becomes "*" and int64(2026) becomes "ߪ", not "42"/"2026".
// Loading must render the decimal digits so a load -> save round trip keeps
// the number, and the reverse direction must reject string -> []int32, which
// reflect also allows via []rune and would load "80" as the character codes
// {56, 48}.
func TestIntegerStringConversions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		data  map[any]any
		start *conversionVertex
		check func(*testing.T, *conversionVertex)
	}{
		{
			name: "Int32LoadsAsDecimalText",
			data: map[any]any{"rank": int32(42)},
			check: func(t *testing.T, v *conversionVertex) {
				if v.Rank != "42" {
					t.Errorf("Rank = %q, want %q (decimal text, not rune '*')", v.Rank, "42")
				}
			},
		},
		{
			name: "Int64LoadsAsDecimalText",
			data: map[any]any{"rank": int64(2026)},
			check: func(t *testing.T, v *conversionVertex) {
				if v.Rank != "2026" {
					t.Errorf("Rank = %q, want %q (decimal text, not rune mojibake)", v.Rank, "2026")
				}
			},
		},
		{
			name: "UnsignedIntLoadsAsDecimalText",
			data: map[any]any{"rank": uint64(65)},
			check: func(t *testing.T, v *conversionVertex) {
				if v.Rank != "65" {
					t.Errorf("Rank = %q, want %q", v.Rank, "65")
				}
			},
		},
		{
			name: "UintptrLoadsAsDecimalText",
			data: map[any]any{"rank": uintptr(7)},
			check: func(t *testing.T, v *conversionVertex) {
				if v.Rank != "7" {
					t.Errorf("Rank = %q, want %q (decimal text, not rune '\\a')", v.Rank, "7")
				}
			},
		},
		{
			name:  "IntegerNarrowsToFirstAsDecimalText",
			data:  map[any]any{"rank": []any{int32(1), int32(2)}},
			start: &conversionVertex{Rank: "unchanged"},
			check: func(t *testing.T, v *conversionVertex) {
				if v.Rank != "1" {
					t.Errorf("Rank = %q, want %q", v.Rank, "1")
				}
			},
		},
		{
			name: "IntegerLoadsIntoPointerStringField",
			data: map[any]any{"score": int64(5)},
			check: func(t *testing.T, v *conversionVertex) {
				if v.Score == nil {
					t.Fatalf(`Score = nil, want pointer to "5"`)
				}
				if *v.Score != "5" {
					t.Errorf("*Score = %q, want %q", *v.Score, "5")
				}
			},
		},
		{
			name: "IntegerElementsLoadIntoStringSlice",
			data: map[any]any{"weights": []any{int64(7), "eight"}},
			check: func(t *testing.T, v *conversionVertex) {
				if want := []string{"7", "eight"}; !reflect.DeepEqual(v.Weights, want) {
					t.Errorf("Weights = %#v, want %#v", v.Weights, want)
				}
			},
		},
		{
			name: "IntegerElementsLoadIntoPointerStringSlice",
			data: map[any]any{"ptrWeights": []any{int64(7), "eight"}},
			check: func(t *testing.T, v *conversionVertex) {
				if len(v.PtrWeights) != 2 || v.PtrWeights[0] == nil || v.PtrWeights[1] == nil {
					t.Fatalf(`PtrWeights = %#v, want two non-nil pointers`, v.PtrWeights)
				}
				if *v.PtrWeights[0] != "7" || *v.PtrWeights[1] != "eight" {
					t.Errorf(`PtrWeights = [%q %q], want ["7" "eight"]`,
						*v.PtrWeights[0], *v.PtrWeights[1])
				}
			},
		},
		{
			name: "IntegerWrapsIntoStringSlice",
			data: map[any]any{"weights": int64(7)},
			check: func(t *testing.T, v *conversionVertex) {
				if want := []string{"7"}; !reflect.DeepEqual(v.Weights, want) {
					t.Errorf("Weights = %#v, want %#v", v.Weights, want)
				}
			},
		},
		{
			name: "IntegerWrapsIntoPointerStringSlice",
			data: map[any]any{"ptrWeights": int64(7)},
			check: func(t *testing.T, v *conversionVertex) {
				if len(v.PtrWeights) != 1 || v.PtrWeights[0] == nil {
					t.Fatalf(`PtrWeights = %#v, want [ptr("7")]`, v.PtrWeights)
				}
				if *v.PtrWeights[0] != "7" {
					t.Errorf(`*PtrWeights[0] = %q, want %q`, *v.PtrWeights[0], "7")
				}
			},
		},
		{
			name: "StringDoesNotDecodeIntoIntegerFields",
			data: map[any]any{"quantity": "42", "ratio": "3.5"},
			check: func(t *testing.T, v *conversionVertex) {
				if v.Quantity != 0 {
					t.Errorf("Quantity = %d, want 0 (no silent string parsing)", v.Quantity)
				}
				if v.Ratio != 0 {
					t.Errorf("Ratio = %v, want 0", v.Ratio)
				}
			},
		},
		{
			name: "StringDoesNotDecodeIntoIntSliceAsRunes",
			data: map[any]any{"ports": "80"},
			check: func(t *testing.T, v *conversionVertex) {
				if v.Ports != nil {
					t.Errorf("Ports = %#v, want nil (must not load as rune codes {56 48})", v.Ports)
				}
			},
		},
		{
			name: "StringLoadsIntoAnyField",
			data: map[any]any{"anyValue": "abc"},
			check: func(t *testing.T, v *conversionVertex) {
				if v.AnyValue != "abc" {
					t.Errorf(
						"AnyValue = %#v, want \"abc\" (strings must survive into any fields)",
						v.AnyValue,
					)
				}
			},
		},
		{
			name: "ByteSliceLoadsIntoByteArrayField",
			data: map[any]any{"fixed": []byte("hell")},
			check: func(t *testing.T, v *conversionVertex) {
				if want := [4]byte{'h', 'e', 'l', 'l'}; v.Fixed != want {
					t.Errorf("Fixed = %#v, want %#v", v.Fixed, want)
				}
			},
		},
		{
			name: "ByteSliceStillConvertsToString",
			data: map[any]any{"tag": []byte("hello")},
			check: func(t *testing.T, v *conversionVertex) {
				if v.Tag != "hello" {
					t.Errorf("Tag = %q, want %q", v.Tag, "hello")
				}
			},
		},
		{
			name: "NumericAndStringPathsStillWork",
			data: map[any]any{
				"quantity": int32(7),
				"ratio":    int64(3),
				"weights":  []any{"7", "eight"},
			},
			check: func(t *testing.T, v *conversionVertex) {
				if v.Quantity != 7 {
					t.Errorf("Quantity = %d, want 7", v.Quantity)
				}
				if v.Ratio != 3 {
					t.Errorf("Ratio = %v, want 3", v.Ratio)
				}
				if want := []string{"7", "eight"}; !reflect.DeepEqual(v.Weights, want) {
					t.Errorf("Weights = %#v, want %#v", v.Weights, want)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v := conversionVertex{}
			if tc.start != nil {
				v = *tc.start
			}
			err := driver.UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{Data: tc.data},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			tc.check(t, &v)
		})
	}
}

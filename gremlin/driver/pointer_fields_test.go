package driver_test

import (
	"slices"
	"testing"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

// testVertexWithOptionalFields models optional gremlin properties. Pointer
// fields are loaded through the value they point at, and stay nil when the
// stored value cannot be held by that value's type.
type testVertexWithOptionalFields struct {
	gsmtypes.Vertex
	Name    *string   `json:"name"    gremlin:"name"`
	Count   *int      `json:"count"   gremlin:"count"`
	Ratio   *float64  `json:"ratio"   gremlin:"ratio"`
	Tags    *[]string `json:"tags"    gremlin:"tags"`
	Missing *string   `json:"missing" gremlin:"missing"`
	Wrong   *int      `json:"wrong"   gremlin:"name"`
}

// testVertexWithScalarField receives gremlin values that do not fit it.
type testVertexWithScalarField struct {
	gsmtypes.Vertex
	Name string `json:"name" gremlin:"name"`
}

// testVertexWithNestedPointerField stores its value behind a chain of
// pointers, which structToMap must follow to the value it points at.
type testVertexWithNestedPointerField struct {
	gsmtypes.Vertex
	Value **string `json:"value" gremlin:"value"`
}

func optionalFieldsResult() *gremlingo.Result {
	return &gremlingo.Result{
		Data: map[any]any{
			"name":  "bob",
			"count": int64(42),
			"ratio": 1.5,
			"tags":  []any{"a", "b"},
		},
	}
}

func TestUnloadPointerFields(t *testing.T) {
	t.Parallel()
	t.Run(
		"TestOptionalPointerFieldsAreLoaded", func(t *testing.T) {
			t.Parallel()
			var vertex testVertexWithOptionalFields
			if err := driver.UnloadGremlinResultIntoStruct(
				&vertex,
				optionalFieldsResult(),
			); err != nil {
				t.Fatalf("Error unloading struct: %v", err)
			}
			if vertex.Name == nil || *vertex.Name != "bob" {
				t.Errorf("Name should be bob, got %v", vertex.Name)
			}
			if vertex.Count == nil || *vertex.Count != 42 {
				t.Errorf("Count should be 42, got %v", vertex.Count)
			}
			if vertex.Ratio == nil || *vertex.Ratio != 1.5 {
				t.Errorf("Ratio should be 1.5, got %v", vertex.Ratio)
			}
			if vertex.Tags == nil || !slices.Equal(*vertex.Tags, []string{"a", "b"}) {
				t.Errorf("Tags should be [a b], got %v", vertex.Tags)
			}
		},
	)
	t.Run(
		"TestOptionalPointerFieldsStayNil", func(t *testing.T) {
			t.Parallel()
			var vertex testVertexWithOptionalFields
			if err := driver.UnloadGremlinResultIntoStruct(
				&vertex,
				optionalFieldsResult(),
			); err != nil {
				t.Fatalf("Error unloading struct: %v", err)
			}
			if vertex.Missing != nil {
				t.Errorf("Absent property should leave Missing nil, got %v", *vertex.Missing)
			}
			// "name" holds a string, which an *int field cannot store.
			if vertex.Wrong != nil {
				t.Errorf("Mismatched property should leave Wrong nil, got %v", *vertex.Wrong)
			}
		},
	)
	t.Run(
		"TestNullPropertyLeavesPointerNil", func(t *testing.T) {
			t.Parallel()
			var vertex testVertexWithOptionalFields
			result := &gremlingo.Result{Data: map[any]any{"name": nil}}
			if err := driver.UnloadGremlinResultIntoStruct(&vertex, result); err != nil {
				t.Fatalf("Error unloading struct: %v", err)
			}
			if vertex.Name != nil {
				t.Errorf("Null property should leave Name nil, got %v", *vertex.Name)
			}
		},
	)
	t.Run(
		"TestExistingPointerIsUpdatedInPlace", func(t *testing.T) {
			t.Parallel()
			name := "old"
			vertex := testVertexWithOptionalFields{Name: &name}
			if err := driver.UnloadGremlinResultIntoStruct(
				&vertex,
				optionalFieldsResult(),
			); err != nil {
				t.Fatalf("Error unloading struct: %v", err)
			}
			if vertex.Name != &name {
				t.Errorf("Name should reuse the existing allocation")
			}
			if name != "bob" {
				t.Errorf("Pointed-to value should be updated to bob, got %s", name)
			}
		},
	)
	t.Run(
		"TestPointerToPointerTargetIsLoaded", func(t *testing.T) {
			t.Parallel()
			var vertex *testVertexWithOptionalFields
			if err := driver.UnloadGremlinResultIntoStruct(
				&vertex,
				optionalFieldsResult(),
			); err != nil {
				t.Fatalf("Error unloading struct: %v", err)
			}
			if vertex == nil || vertex.Name == nil || *vertex.Name != "bob" {
				t.Errorf("Nested pointer target should be allocated and loaded, got %+v", vertex)
			}
		},
	)
	t.Run(
		"TestListValueIntoScalarFieldIsIgnored", func(t *testing.T) {
			t.Parallel()
			var vertex testVertexWithScalarField
			result := &gremlingo.Result{Data: map[any]any{"name": []any{"a", "b"}}}
			if err := driver.UnloadGremlinResultIntoStruct(&vertex, result); err != nil {
				t.Fatalf("Error unloading struct: %v", err)
			}
			if vertex.Name != "" {
				t.Errorf("Incompatible list value should leave Name empty, got %s", vertex.Name)
			}
		},
	)
	t.Run(
		"TestStructToMapSkipsUnsetPointerFields", func(t *testing.T) {
			t.Parallel()
			nested := &testVertexWithOptionalFields{}
			mapValue, err := driver.StructToMapForTest(nested)
			if err != nil {
				t.Fatalf("Error converting struct to map: %v", err)
			}
			for _, key := range []string{"name", "count", "ratio", "tags", "missing"} {
				if _, ok := mapValue[key]; ok {
					t.Errorf("unset pointer field %q should be skipped, got %v", key, mapValue[key])
				}
			}
		},
	)
	t.Run(
		"TestStructToMapFollowsPointerChains", func(t *testing.T) {
			t.Parallel()
			storedValue := "stored"
			pointerToValue := &storedValue
			nested := &testVertexWithNestedPointerField{Value: &pointerToValue}
			mapValue, err := driver.StructToMapForTest(nested)
			if err != nil {
				t.Fatalf("Error converting struct to map: %v", err)
			}
			got, ok := mapValue["value"].(string)
			if !ok || got != "stored" {
				t.Errorf(
					"value should be the pointed-to string, got %T %v",
					mapValue["value"],
					mapValue["value"],
				)
			}
		},
	)
}

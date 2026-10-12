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

// testVertexWithNestedPointerField declares a stacked pointer property.
// Optional properties support a single pointer level, so the field is not a
// gremlin property: it is neither written nor loaded.
type testVertexWithNestedPointerField struct {
	gsmtypes.Vertex
	Value **string `json:"value" gremlin:"value"`
}

// testVertexWithOmitEmptyPointers pairs pointer fields with omitempty. A nil
// pointer is unset and omitted; a pointer to a zero value is an explicit
// value and must still be written, which is why the omitempty check happens
// before the pointer is dereferenced.
type testVertexWithOmitEmptyPointers struct {
	gsmtypes.Vertex
	Name   *string   `json:"name"   gremlin:"name,omitempty"`
	Active *bool     `json:"active" gremlin:"active,omitempty"`
	Count  *int      `json:"count"  gremlin:"count,omitempty"`
	Ratio  *float64  `json:"ratio"  gremlin:"ratio,omitempty"`
	Tags   *[]string `json:"tags"   gremlin:"tags,omitempty"`
	Nick   *string   `json:"nick"   gremlin:"nick"`
	Plain  string    `json:"plain"  gremlin:"plain,omitempty"`
}

// testVertexWithOmitEmptySerializer marks a custom serializer field with
// omitempty. testAttributes comes from serializer_test.go and serializes to
// JSON, so the tests can tell "skipped" from "serialized".
type testVertexWithOmitEmptySerializer struct {
	gsmtypes.Vertex
	Attributes *testAttributes `gremlin:"attributes,omitempty"`
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
			// The destination shape NewQuery[*Model] unloads through: a nil
			// model pointer the driver allocates. This is not field-level
			// support for stacked pointers.
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
		"TestStructToMapIgnoresNestedPointerFields", func(t *testing.T) {
			t.Parallel()
			storedValue := "stored"
			pointerToValue := &storedValue
			nested := &testVertexWithNestedPointerField{Value: &pointerToValue}
			mapValue, err := driver.StructToMapForTest(nested)
			if err != nil {
				t.Fatalf("Error converting struct to map: %v", err)
			}
			if _, ok := mapValue["value"]; ok {
				t.Errorf(
					"stacked pointer field should not be written, got %T %v",
					mapValue["value"],
					mapValue["value"],
				)
			}
		},
	)
}

func TestNestedPointerFieldIsNotLoaded(t *testing.T) {
	t.Parallel()
	var vertex testVertexWithNestedPointerField
	result := &gremlingo.Result{Data: map[any]any{"value": "stored"}}
	if err := driver.UnloadGremlinResultIntoStruct(&vertex, result); err != nil {
		t.Fatalf("Error unloading struct: %v", err)
	}
	if vertex.Value != nil {
		t.Errorf("stacked pointer field should stay untouched, got %v", *vertex.Value)
	}
}

func TestStructToMapOmitEmptyPointerFields(t *testing.T) {
	t.Parallel()
	t.Run(
		"TestNilPointersWithOmitEmptyAreSkipped", func(t *testing.T) {
			t.Parallel()
			mapValue, err := driver.StructToMapForTest(&testVertexWithOmitEmptyPointers{})
			if err != nil {
				t.Fatalf("Error converting struct to map: %v", err)
			}
			for _, key := range []string{"name", "active", "count", "ratio", "tags"} {
				if _, ok := mapValue[key]; ok {
					t.Errorf("nil pointer field %q should be skipped, got %v", key, mapValue[key])
				}
			}
		},
	)
	t.Run(
		"TestPointersToZeroValuesWithOmitEmptyAreKept", func(t *testing.T) {
			t.Parallel()
			name := ""
			active := false
			count := 0
			ratio := 0.0
			tags := []string{}
			vertex := &testVertexWithOmitEmptyPointers{
				Name:   &name,
				Active: &active,
				Count:  &count,
				Ratio:  &ratio,
				Tags:   &tags,
			}
			mapValue, err := driver.StructToMapForTest(vertex)
			if err != nil {
				t.Fatalf("Error converting struct to map: %v", err)
			}
			// A set pointer is a value the graph must receive, even when the
			// value it points at is the zero value for its type.
			if got, ok := mapValue["active"].(bool); !ok || got != false {
				t.Errorf("active should be stored as false, got %T %v",
					mapValue["active"], mapValue["active"])
			}
			for _, key := range []string{"name", "count", "ratio", "tags"} {
				if _, ok := mapValue[key]; !ok {
					t.Errorf("pointer to zero value %q should be stored, got missing", key)
				}
			}
		},
	)
	t.Run(
		"TestZeroValueWithoutOmitEmptyIsKept", func(t *testing.T) {
			t.Parallel()
			nick := ""
			vertex := &testVertexWithOmitEmptyPointers{Nick: &nick}
			mapValue, err := driver.StructToMapForTest(vertex)
			if err != nil {
				t.Fatalf("Error converting struct to map: %v", err)
			}
			if got, ok := mapValue["nick"].(string); !ok || got != "" {
				t.Errorf("nick without omitempty should be stored as empty, got %T %v",
					mapValue["nick"], mapValue["nick"])
			}
		},
	)
	t.Run(
		"TestZeroValueNonPointerWithOmitEmptyIsSkipped", func(t *testing.T) {
			t.Parallel()
			mapValue, err := driver.StructToMapForTest(&testVertexWithOmitEmptyPointers{})
			if err != nil {
				t.Fatalf("Error converting struct to map: %v", err)
			}
			// Non-pointer fields keep the usual zero-value meaning of omitempty.
			if _, ok := mapValue["plain"]; ok {
				t.Errorf("empty non-pointer field should be skipped, got %v", mapValue["plain"])
			}
		},
	)
	t.Run(
		"TestNilSerializerPointerWithOmitEmptyIsSkipped", func(t *testing.T) {
			t.Parallel()
			mapValue, err := driver.StructToMapForTest(&testVertexWithOmitEmptySerializer{})
			if err != nil {
				t.Fatalf("Error converting struct to map: %v", err)
			}
			if _, ok := mapValue["attributes"]; ok {
				t.Errorf(
					"nil serializer pointer should be skipped, got %v",
					mapValue["attributes"],
				)
			}
		},
	)
	t.Run(
		"TestPointerToZeroSerializerWithOmitEmptyIsSerialized", func(t *testing.T) {
			t.Parallel()
			empty := testAttributes{}
			mapValue, err := driver.StructToMapForTest(
				&testVertexWithOmitEmptySerializer{Attributes: &empty},
			)
			if err != nil {
				t.Fatalf("Error converting struct to map: %v", err)
			}
			// A set pointer is serialized even when the value behind it is empty.
			if got, ok := mapValue["attributes"].(string); !ok || got != "{}" {
				t.Errorf(
					"pointer to a zero serializer should serialize, got %T %v",
					mapValue["attributes"], mapValue["attributes"],
				)
			}
		},
	)
}

// testVertexWithMismatchedProperties tags fields with properties stored as a
// type the field cannot hold, which must leave every field untouched.
type testVertexWithMismatchedProperties struct {
	gsmtypes.Vertex
	TitleFromNumber *string `gremlin:"title_from_number"`
	TextToNumber    *int    `gremlin:"text_to_number"`
	Plain           string  `gremlin:"plain"`
}

func TestMismatchedPropertyTypesAreNotAssigned(t *testing.T) {
	t.Parallel()
	var vertex testVertexWithMismatchedProperties
	result := &gremlingo.Result{Data: map[any]any{
		"title_from_number": int64(65),
		"text_to_number":    "abc",
		"plain":             int64(65),
	}}
	if err := driver.UnloadGremlinResultIntoStruct(&vertex, result); err != nil {
		t.Fatalf("Error unloading struct: %v", err)
	}
	// Go converts a number to text as a UTF-8 code point, which would store
	// "A" for the int64 property 65 instead of leaving the field alone.
	if vertex.TitleFromNumber != nil {
		t.Errorf("numeric property should leave *string nil, got %q", *vertex.TitleFromNumber)
	}
	if vertex.TextToNumber != nil {
		t.Errorf("text property should leave *int nil, got %v", *vertex.TextToNumber)
	}
	if vertex.Plain != "" {
		t.Errorf("numeric property should leave string field empty, got %q", vertex.Plain)
	}
}

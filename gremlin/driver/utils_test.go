package driver

import (
	"slices"
	"testing"
	"time"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

type testVertexForUtils struct {
	gsmtypes.Vertex
	Name              string            `json:"name"          gremlin:"name"`
	Ignore            string            `json:"-"             gremlin:"-"`
	ListTest          []string          `json:"listTest"      gremlin:"listTest"`
	MapTest           map[string]string `json:"mapTest"       gremlin:"mapTest"`
	Unmapped          int               `json:"unmapped"      gremlin:"unmapped"`
	Sort              int               `json:"sort"          gremlin:"sort"`
	SubTraversalTest  string            `json:"testConstant"                                    gremlinSubTraversal:"subTraversalTest"`
	SubTraversalTest2 int               `json:"testConstant2"                                   gremlinSubTraversal:"subTraversalTest2"`
	OmitEmptyTest     string            `json:"omitEmptyTest" gremlin:"omitEmptyTest,omitempty"`
}

type testVertexWithNumSlice struct {
	gsmtypes.Vertex
	ListInts []int `json:"listInts" gremlin:"listInts"`
}

type testVertexWithCustomLabel struct {
	gsmtypes.Vertex
	Name string `json:"name" gremlin:"name"`
}

type testVertexWithExtras struct {
	gsmtypes.Vertex
	Name   string         `json:"name"   gremlin:"name"`
	Extras map[string]any `json:"extras" gremlin:"-,unmapped"`
}

type testVertexWithMultipleExtras struct {
	gsmtypes.Vertex
	Name     string         `json:"name"     gremlin:"name"`
	Extras   map[string]any `json:"extras"   gremlin:"-,unmapped"`
	ExtrasV2 map[string]any `json:"extrasV2" gremlin:"-,unmapped"`
}

type testVertexWithInvalidExtras struct {
	gsmtypes.Vertex
	Name   string            `json:"name"   gremlin:"name"`
	Extras map[string]string `json:"extras" gremlin:"-,unmapped"`
}

type testVertexWithSubTraversalPreference struct {
	gsmtypes.Vertex
	Value string `json:"value" gremlin:"value" gremlinSubTraversal:"value_sub"`
}

// Label implements a custom label function
func (v *testVertexWithCustomLabel) Label() string {
	return "customVertexLabel"
}

func TestUtils(t *testing.T) {
	t.Parallel()
	t.Run(
		"GetStructName", func(t *testing.T) {
			t.Parallel()
			name, err := getStructName[testVertexForUtils]()
			if err != nil {
				t.Errorf("Error getting struct name: %v", err)
			}
			if name != "testVertexForUtils" {
				t.Errorf("Struct name should be testVertexForUtils, got %s", name)
			}
		},
	)
	t.Run(
		"GetStructNameErr", func(t *testing.T) {
			t.Parallel()
			_, err := getStructName[int]()
			if err == nil {
				t.Errorf("No error getting struct name: %v", err)
			}
		},
	)
	t.Run(
		"TestUnloadGremlinResultIntoStruct", func(t *testing.T) {
			t.Parallel()
			var v testVertexForUtils
			now := time.Now().UTC()
			err := UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{
					Data: map[any]any{
						"id":            "1",
						"last_modified": now,
						"created_at":    now,
						"name":          "test",
						"listTest":      []string{"test1", "test2"},
					},
				},
			)
			if err != nil {
				t.Errorf("Error unloading gremlin result into struct: %v", err)
			}
			if v.ID != "1" {
				t.Errorf("Vertex ID should be 1, got %s", v.ID)
			}
			if !v.LastModified.Equal(now) {
				t.Errorf("Vertex LastModified should be %v, got %v", now, v.LastModified)
			}
			if !v.CreatedAt.Equal(now) {
				t.Errorf("Vertex CreatedAt should be %v, got %v", now, v.CreatedAt)
			}
			if v.Name != "test" {
				t.Errorf("Vertex Name should be test, got %s", v.Name)
			}
		},
	)
	t.Run(
		"TestUnloadGremlinResultIntoStructExtras", func(t *testing.T) {
			t.Parallel()
			var v testVertexWithExtras
			now := time.Now().UTC()
			err := UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{
					Data: map[any]any{
						"id":            "1",
						"last_modified": now,
						"created_at":    now,
						"name":          "test",
						"unknown":       "extra",
						"flag":          true,
						"count":         42,
						"tags":          []any{"a", "b"},
					},
				},
			)
			if err != nil {
				t.Errorf("Error unloading gremlin result into struct: %v", err)
			}
			if v.Name != "test" {
				t.Errorf("Vertex Name should be test, got %s", v.Name)
			}
			if v.Extras == nil {
				t.Errorf("Extras should not be nil")
			}
			if v.Extras["unknown"] != "extra" {
				t.Errorf("Extras should contain unknown field")
			}
			if v.Extras["flag"] != true {
				t.Errorf("Extras should contain boolean field")
			}
			if v.Extras["count"] != 42 {
				t.Errorf("Extras should contain int field")
			}
			tags, ok := v.Extras["tags"].([]any)
			if !ok || len(tags) != 2 {
				t.Errorf("Extras should contain slice field")
			}
			if _, ok := v.Extras["name"]; ok {
				t.Errorf("Extras should not contain mapped fields")
			}
		},
	)
	t.Run(
		"TestUnloadGremlinResultIntoStructMultipleExtras", func(t *testing.T) {
			t.Parallel()
			var v testVertexWithMultipleExtras
			err := UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{
					Data: map[any]any{
						"id":      "1",
						"name":    "test",
						"unknown": "extra",
					},
				},
			)
			if err != nil {
				t.Errorf("Error unloading gremlin result into struct: %v", err)
			}
			if v.Extras == nil || v.ExtrasV2 == nil {
				t.Errorf("Extras should not be nil")
			}
			if v.Extras["unknown"] != "extra" || v.ExtrasV2["unknown"] != "extra" {
				t.Errorf("Extras should contain unknown field")
			}
		},
	)
	t.Run(
		"TestUnloadGremlinResultIntoStructInvalidExtrasType", func(t *testing.T) {
			t.Parallel()
			var v testVertexWithInvalidExtras
			err := UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{
					Data: map[any]any{
						"id":      "1",
						"name":    "test",
						"unknown": "extra",
					},
				},
			)
			if err != nil {
				t.Errorf("Error unloading gremlin result into struct: %v", err)
			}
			if v.Extras != nil {
				t.Errorf("Extras should remain nil when type is unsupported")
			}
		},
	)
	t.Run(
		"TestUnloadGremlinResultIntoStructSubTraversalPreferred", func(t *testing.T) {
			t.Parallel()
			var v testVertexWithSubTraversalPreference
			err := UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{
					Data: map[any]any{
						"value":     "base",
						"value_sub": "sub",
					},
				},
			)
			if err != nil {
				t.Errorf("Error unloading gremlin result into struct: %v", err)
			}
			if v.Value != "sub" {
				t.Errorf("Value should prefer subtraversal result, got %s", v.Value)
			}
		},
	)
	unloadGremlinResultIntoStructTests := []struct {
		testName  string
		result    *gremlingo.Result
		v         any
		shouldErr bool
	}{
		{
			result:    &gremlingo.Result{},
			v:         &testVertexForUtils{},
			shouldErr: true,
			testName:  "UnloadGremlinResultIntoStructWithError",
		},
		{
			result: &gremlingo.Result{
				Data: map[any]any{
					"id":           "1",
					"lastModified": 1,
					"name":         "test",
					"listTest":     []string{"test1", "test2"},
				},
			},
			v:         &testVertexForUtils{},
			shouldErr: false,
			testName:  "UnloadGremlinResultIntoStructTest",
		},
		{
			result: &gremlingo.Result{
				Data: map[any]any{
					"id":           "1",
					"lastModified": 1,
					"name":         "test",
					"listTest":     []any{"test1", "test2"},
				},
			},
			v:         testVertexForUtils{},
			shouldErr: true,
			testName:  "UnloadGremlinResultIntoStructTestWithoutPointer",
		},
		{
			testName: "UnloadGremlinResultIntoStructTestWithSlice",
			result: &gremlingo.Result{
				Data: map[any]any{
					"id":           "1",
					"lastModified": 1,
					"listInts":     []any{1.0, 2.0, 3.0},
				},
			},
			shouldErr: false,
			v:         &testVertexWithNumSlice{},
		},
	}

	for _, tt := range unloadGremlinResultIntoStructTests {
		t.Run(
			tt.testName, func(t *testing.T) {
				t.Parallel()
				err := UnloadGremlinResultIntoStruct(tt.v, tt.result)
				if (err != nil) != tt.shouldErr {
					t.Errorf(
						"UnloadGremlinResultIntoStruct() error = %v, shouldErr %v",
						err,
						tt.shouldErr,
					)
				}
			},
		)
	}

	t.Run(
		"TestStructToMap", func(t *testing.T) {
			t.Parallel()
			v := testVertexForUtils{
				Name: "test",
			}
			name, mapValue, err := structToMap(v)
			if err != nil {
				t.Errorf("Error getting struct name: %v", err)
			}
			if name != "test_vertex_for_utils" {
				t.Errorf("Struct name should be test_vertex_for_utils, got %s", name)
			}
			if mapValue["name"] != "test" {
				t.Errorf("Struct name should be test, got %s", mapValue["name"])
			}
		},
	)
	t.Run(
		"TestStructToMapPointer", func(t *testing.T) {
			t.Parallel()
			v := testVertexForUtils{
				Name: "test",
			}
			name, mapValue, err := structToMap(&v)
			if err != nil {
				t.Errorf("Error getting struct name: %v", err)
			}
			if name != "test_vertex_for_utils" {
				t.Errorf("Struct name should be test_vertex_for_utils, got %s", name)
			}
			if mapValue["name"] != "test" {
				t.Errorf("Struct name should be test, got %s", mapValue["name"])
			}
		},
	)
	t.Run(
		"TestStructToMapPointerError", func(t *testing.T) {
			t.Parallel()
			_, _, err := structToMap(1)
			if err == nil {
				t.Errorf("No error struct to map: %v", err)
			}
		},
	)
	t.Run(
		"TestStructToMapWithCustomLabel", func(t *testing.T) {
			t.Parallel()
			v := &testVertexWithCustomLabel{
				Name: "test",
			}
			label, mapValue, err := structToMap(v)
			if err != nil {
				t.Errorf("Error getting struct to map: %v", err)
			}
			// Verify custom label is used instead of normalized struct name
			if label != "customVertexLabel" {
				t.Errorf("Label should be customVertexLabel, got %s", label)
			}
			if mapValue["name"] != "test" {
				t.Errorf("Map value name should be test, got %s", mapValue["name"])
			}
		},
	)
	t.Run(
		"TestGetLabelFromValueWithCustomLabel", func(t *testing.T) {
			t.Parallel()
			v := &testVertexWithCustomLabel{
				Name: "test",
			}
			label := getLabelFromVertex(v)
			// Verify custom label is used
			if label != "customVertexLabel" {
				t.Errorf("Label should be customVertexLabel, got %s", label)
			}
		},
	)
	t.Run(
		"TestGetLabelFromValueWithDefaultLabel", func(t *testing.T) {
			t.Parallel()
			v := &testVertexForUtils{
				Name: "test",
			}
			label := getLabelFromVertex(v)
			// Verify default normalization is used when Label() returns empty
			if label != "test_vertex_for_utils" {
				t.Errorf("Label should be test_vertex_for_utils, got %s", label)
			}
		},
	)
	t.Run(
		"TestGetLabelFromValueWithCustomLabelPointer", func(t *testing.T) {
			t.Parallel()
			v := &testVertexWithCustomLabel{
				Name: "test",
			}
			label := getLabelFromVertex(v)
			// Verify custom label is used when passing a pointer
			if label != "customVertexLabel" {
				t.Errorf("Label should be customVertexLabel, got %s", label)
			}
		},
	)
	t.Run(
		"TestStructToMapWithCustomLabelPointer", func(t *testing.T) {
			t.Parallel()
			v := &testVertexWithCustomLabel{
				Name: "test",
			}
			label, mapValue, err := structToMap(v)
			if err != nil {
				t.Errorf("Error getting struct to map: %v", err)
			}
			// Verify custom label is used when passing a pointer
			if label != "customVertexLabel" {
				t.Errorf("Label should be customVertexLabel, got %s", label)
			}
			if mapValue["name"] != "test" {
				t.Errorf("Map value name should be test, got %s", mapValue["name"])
			}
		},
	)
	var i *int
	testsForValidatingStructPointer := []struct {
		testName  string
		v         any
		shouldErr bool
	}{
		{testName: "testNil", v: nil, shouldErr: true},
		{testName: "testStruct", v: gremlingo.Result{}, shouldErr: true},
		{testName: "testStructPointer", v: &gremlingo.Result{}, shouldErr: true},
		{testName: "testStructPointerPointer", v: &testVertexForUtils{}, shouldErr: false},
		{testName: "testStructPointerPointerError", v: i, shouldErr: true},
		{testName: "testStructPointerPointerErrorPointer", v: &i, shouldErr: true},
	}
	for _, tt := range testsForValidatingStructPointer {
		t.Run(
			tt.testName, func(t *testing.T) {
				t.Parallel()
				err := validateStructPointerWithAnonymousVertex(tt.v)
				if (err != nil) != tt.shouldErr {
					t.Errorf(
						"validateStructPointerWithAnonymousVertex() error = %v, shouldErr %v",
						err,
						tt.shouldErr,
					)
				}
			},
		)
	}
	t.Run(
		"TestUnloadingIntoGremlinStructWithSingleItem", func(t *testing.T) {
			t.Parallel()
			var test testVertexForUtils
			data := gremlingo.Result{
				Data: map[any]any{
					"id":       "2934234230234",
					"listTest": "1",
				},
			}
			err := UnloadGremlinResultIntoStruct(&test, &data)
			if err != nil {
				t.Errorf("Error unloading struct: %v", err)
			}
			if !slices.Contains(test.ListTest, "1") {
				t.Errorf("ListTest not found in struct")
			}
		},
	)
	t.Run(
		"TestStructToMapWithOmitEmpty", func(t *testing.T) {
			t.Parallel()
			v := testVertexForUtils{
				Name: "test",
			}
			_, mapValue, err := structToMap(v)
			if err != nil {
				t.Errorf("Error getting struct to map: %v", err)
			}
			if mapValue["name"] != "test" {
				t.Errorf("Map value name should be test, got %s", mapValue["name"])
			}
			if _, ok := mapValue["omitEmptyTest"]; ok {
				t.Errorf("OmitEmptyTest should be omitted, got %s", mapValue["omitEmptyTest"])
			}
		},
	)
	t.Run(
		"TestNilPointerOnUnloadGremlinResultIntoStruct", func(t *testing.T) {
			t.Parallel()
			var v *testVertexForUtils
			err := UnloadGremlinResultIntoStruct(v, &gremlingo.Result{Data: map[any]any{
				"something": "test",
			}})
			if err == nil {
				t.Errorf("Error should not be nil")
			}
		},
	)
	t.Run(
		"TestNilResultUnloadGremlinResultIntoStruct", func(t *testing.T) {
			t.Parallel()
			var v testVertexForUtils
			err := UnloadGremlinResultIntoStruct(v, nil)
			if err == nil {
				t.Errorf("Error should not be nil")
			}
		},
	)
}

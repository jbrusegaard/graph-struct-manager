package driver_test

import (
	"slices"
	"testing"
	"time"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

type testVertexWithoutAnonymousVertex struct {
	ID           string    `json:"id" gremlin:"id"`
	LastModified time.Time `json:"last_modified" gremlin:"last_modified"`
	CreatedAt    time.Time `json:"created_at"    gremlin:"created_at"`
	Name         string    `json:"name" gremlin:"name"`
}

func (t *testVertexWithoutAnonymousVertex) GetVertexID() any {
	return t.ID
}

func (t *testVertexWithoutAnonymousVertex) GetVertexLastModified() time.Time {
	return t.LastModified
}

func (t *testVertexWithoutAnonymousVertex) GetVertexCreatedAt() time.Time {
	return t.CreatedAt
}

func (t *testVertexWithoutAnonymousVertex) Label() string {
	return ""
}

func (t *testVertexWithoutAnonymousVertex) SetVertexID(id any) {
	t.ID = id.(string)
}

func (t *testVertexWithoutAnonymousVertex) SetVertexLastModified(lastModified time.Time) {
	t.LastModified = lastModified
}

func (t *testVertexWithoutAnonymousVertex) SetVertexCreatedAt(createdAt time.Time) {
	t.CreatedAt = createdAt
}

type testVertexForUtils struct {
	gsmtypes.Vertex
	Name              string   `json:"name"          gremlin:"name"`
	Ignore            string   `json:"-"             gremlin:"-"`
	ListTest          []string `json:"listTest"      gremlin:"listTest"`
	Unmapped          int      `json:"unmapped"      gremlin:"unmapped"`
	Sort              int      `json:"sort"          gremlin:"sort"`
	SubTraversalTest  string   `json:"testConstant"                                    gremlinSubTraversal:"subTraversalTest"`
	SubTraversalTest2 int      `json:"testConstant2"                                   gremlinSubTraversal:"subTraversalTest2"`
	OmitEmptyTest     string   `json:"omitEmptyTest" gremlin:"omitEmptyTest,omitempty"`
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

func (t *testVertexWithExtras) SetUnmappedProperties(props map[string]any) {
	t.Extras = props
}

type testVertexWithMultipleExtras struct {
	gsmtypes.Vertex
	Name     string         `json:"name"     gremlin:"name"`
	Extras   map[string]any `json:"extras"  `
	ExtrasV2 map[string]any `json:"extrasV2"`
}

func (t *testVertexWithMultipleExtras) SetUnmappedProperties(props map[string]any) {
	t.Extras = props
	t.ExtrasV2 = props
}

type testVertexWithInvalidExtras struct {
	gsmtypes.Vertex
	Name   string            `json:"name"   gremlin:"name"`
	Extras map[string]string `json:"extras" gremlin:"-,unmapped"`
}

type testVertexWithUnmappedInterface struct {
	gsmtypes.Vertex
	Name     string         `json:"name" gremlin:"name"`
	Unmapped map[string]any `json:"unmapped"`
}

func (t *testVertexWithUnmappedInterface) SetUnmappedProperties(props map[string]any) {
	t.Unmapped = props
}

type testVertexWithSubTraversalPreference struct {
	gsmtypes.Vertex
	Value string `json:"value" gremlin:"value" gremlinSubTraversal:"value_sub"`
}

// testVertexWithPointerFields models nullable properties as pointer fields.
type testVertexWithPointerFields struct {
	gsmtypes.Vertex
	Name     *string   `json:"name"     gremlin:"name"`
	Count    *int      `json:"count"    gremlin:"count"`
	Active   *bool     `json:"active"   gremlin:"active"`
	Tags     *[]string `json:"tags"     gremlin:"tags"`
	Skipped  *string   `json:"skipped"  gremlin:"skipped"`
	Nullable *string   `json:"nullable" gremlin:"nullable"`
	Nickname *string   `json:"nickname" gremlin:"nickname,omitempty"`
}

type testVertexWithScalarSlices struct {
	gsmtypes.Vertex
	Names []string `json:"names" gremlin:"names"`
}

// testVertexWithArrayField models a fixed-size array property.
type testVertexWithArrayField struct {
	gsmtypes.Vertex
	Fixed [4]byte `json:"fixed" gremlin:"fixed"`
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
			name, err := driver.GetStructNameForTest[testVertexForUtils]()
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
			_, err := driver.GetStructNameForTest[int]()
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
			err := driver.UnloadGremlinResultIntoStruct(
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
			err := driver.UnloadGremlinResultIntoStruct(
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
			err := driver.UnloadGremlinResultIntoStruct(
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
			err := driver.UnloadGremlinResultIntoStruct(
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
		"TestUnloadGremlinResultIntoStructUnmappedInterface", func(t *testing.T) {
			t.Parallel()
			var v testVertexWithUnmappedInterface
			err := driver.UnloadGremlinResultIntoStruct(
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
			if v.Unmapped == nil {
				t.Errorf("Unmapped properties should not be nil")
			}
			if v.Unmapped["unknown"] != "extra" {
				t.Errorf("Unmapped properties should contain unknown field")
			}
			if _, ok := v.Unmapped["name"]; ok {
				t.Errorf("Unmapped properties should not contain mapped fields")
			}
		},
	)
	t.Run(
		"TestUnloadGremlinResultIntoStructSubTraversalPreferred", func(t *testing.T) {
			t.Parallel()
			var v testVertexWithSubTraversalPreference
			err := driver.UnloadGremlinResultIntoStruct(
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
				err := driver.UnloadGremlinResultIntoStruct(tt.v, tt.result)
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
			mapValue, err := driver.StructToMapForTest(v)
			if err != nil {
				t.Errorf("Error getting struct name: %v", err)
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
			mapValue, err := driver.StructToMapForTest(&v)
			if err != nil {
				t.Errorf("Error getting struct name: %v", err)
			}
			if mapValue["name"] != "test" {
				t.Errorf("Struct name should be test, got %s", mapValue["name"])
			}
		},
	)
	t.Run(
		"TestStructToMapPointerError", func(t *testing.T) {
			t.Parallel()
			_, err := driver.StructToMapForTest(1)
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
			mapValue, err := driver.StructToMapForTest(v)
			if err != nil {
				t.Errorf("Error getting struct to map: %v", err)
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
			label := driver.GetLabelFromVertexForTest(v)
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
			label := driver.GetLabelFromVertexForTest(v)
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
			label := driver.GetLabelFromVertexForTest(v)
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
			mapValue, err := driver.StructToMapForTest(v)
			if err != nil {
				t.Errorf("Error getting struct to map: %v", err)
			}

			if mapValue["name"] != "test" {
				t.Errorf("Map value name should be test, got %s", mapValue["name"])
			}
			label := driver.GetLabel[testVertexWithCustomLabel]()
			if label != v.Label() {
				t.Errorf("Label should be customVertexLabel, got %s", label)
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
				err := driver.ValidateStructPointerWithAnonymousVertexForTest(tt.v)
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
			err := driver.UnloadGremlinResultIntoStruct(&test, &data)
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
			mapValue, err := driver.StructToMapForTest(v)
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
			err := driver.UnloadGremlinResultIntoStruct(
				v, &gremlingo.Result{
					Data: map[any]any{
						"something": "test",
					},
				},
			)
			if err == nil {
				t.Errorf("Error should not be nil")
			}
		},
	)
	t.Run(
		"TestNilResultUnloadGremlinResultIntoStruct", func(t *testing.T) {
			t.Parallel()
			var v testVertexForUtils
			err := driver.UnloadGremlinResultIntoStruct(v, nil)
			if err == nil {
				t.Errorf("Error should not be nil")
			}
		},
	)
}

func TestPointerFields(t *testing.T) {
	t.Parallel()
	t.Run(
		"UnloadAllocatesPointerFields", func(t *testing.T) {
			t.Parallel()
			var v testVertexWithPointerFields
			err := driver.UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{
					Data: map[any]any{
						"name":     "test",
						"count":    int64(3),
						"active":   false,
						"tags":     []any{"a", nil},
						"skipped":  nil,
						"nullable": (*string)(nil),
					},
				},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if v.Name == nil || *v.Name != "test" {
				t.Errorf("Name should be test, got %v", v.Name)
			}
			if v.Count == nil || *v.Count != 3 {
				t.Errorf("Count should be 3, got %v", v.Count)
			}
			if v.Active == nil || *v.Active != false {
				t.Errorf("Active should point to false, got %v", v.Active)
			}
			if v.Tags == nil || len(*v.Tags) != 2 || (*v.Tags)[0] != "a" ||
				(*v.Tags)[1] != "" {
				t.Errorf("Tags should be [a \"\"], got %v", v.Tags)
			}
			if v.Skipped != nil {
				t.Errorf("Nil property should leave Skipped unset, got %v", *v.Skipped)
			}
			if v.Nullable != nil {
				t.Errorf("Typed nil property should leave Nullable unset, got %v", *v.Nullable)
			}
		},
	)
	t.Run(
		"StructToMapKeepsPointerToZeroValue", func(t *testing.T) {
			t.Parallel()
			name := "test"
			active := false
			nickname := ""
			v := testVertexWithPointerFields{
				Name: &name, Active: &active, Nickname: &nickname,
			}
			mapValue, err := driver.StructToMapForTest(&v)
			if err != nil {
				t.Fatalf("Error converting struct to map: %v", err)
			}
			if value, ok := mapValue["active"]; !ok || value != false {
				t.Errorf("&false should be written as false, got %v (present: %v)", value, ok)
			}
			if value, ok := mapValue["nickname"]; !ok || value != "" {
				t.Errorf("&\"\" with omitempty should be written, got %v (present: %v)", value, ok)
			}
			if value, ok := mapValue["name"]; !ok || value != "test" {
				t.Errorf("Name should be test, got %v", value)
			}
			for _, property := range []string{"count", "skipped", "nullable"} {
				if _, ok := mapValue[property]; ok {
					t.Errorf("Nil pointer property %q should be skipped", property)
				}
			}
		},
	)
	t.Run(
		"StructToMapOmitsZeroValuePointersWithoutValue", func(t *testing.T) {
			t.Parallel()
			v := testVertexWithPointerFields{}
			mapValue, err := driver.StructToMapForTest(&v)
			if err != nil {
				t.Fatalf("Error converting struct to map: %v", err)
			}
			for _, property := range []string{"name", "nickname"} {
				if _, ok := mapValue[property]; ok {
					t.Errorf("Nil pointer property %q should be skipped", property)
				}
			}
		},
	)
	t.Run(
		"UnloadSliceWithNilElement", func(t *testing.T) {
			t.Parallel()
			var v testVertexWithScalarSlices
			err := driver.UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{
					Data: map[any]any{"names": []any{"a", nil, "c"}},
				},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			want := []string{"a", "", "c"}
			if !slices.Equal(v.Names, want) {
				t.Errorf("Names should be %v, got %v", want, v.Names)
			}
		},
	)
	t.Run(
		"UnloadMismatchesAreSkipped", func(t *testing.T) {
			t.Parallel()
			var v testVertexWithPointerFields
			err := driver.UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{
					// A list going into a single-value field is skipped.
					Data: map[any]any{"name": []any{"a", "b"}},
				},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if v.Name != nil {
				t.Errorf("List into single-value field should leave Name unset, got %v", *v.Name)
			}

			var s testVertexWithScalarSlices
			err = driver.UnloadGremlinResultIntoStruct(
				&s, &gremlingo.Result{
					// An element that cannot convert keeps its zero value.
					Data: map[any]any{"names": []any{"a", true, "c"}},
				},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			want := []string{"a", "", "c"}
			if !slices.Equal(s.Names, want) {
				t.Errorf("Names should be %v, got %v", want, s.Names)
			}

			var a testVertexWithArrayField
			err = driver.UnloadGremlinResultIntoStruct(
				&a, &gremlingo.Result{
					// A short list into a fixed-size array is skipped, not a panic.
					Data: map[any]any{"fixed": []byte{1, 2}},
				},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if a.Fixed != [4]byte{} {
				t.Errorf("Mismatched array should be left zero, got %v", a.Fixed)
			}
		},
	)
}

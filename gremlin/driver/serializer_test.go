package driver_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/comparator"
	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

// testAttributes is a map alias persisted as a JSON string through the
// custom serializer interfaces (gremlin has no native map property support).
type testAttributes map[string]string

func (a testAttributes) SerializeGremlinValue() (any, error) {
	encoded, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return string(encoded), nil
}

func (a *testAttributes) DeserializeGremlinValue(value any) error {
	encoded, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", value)
	}
	return json.Unmarshal([]byte(encoded), a)
}

// testPointerSerializer implements both interfaces with pointer receivers
// only, exercising the addressability handling.
type testPointerSerializer struct {
	Value string
}

func (p *testPointerSerializer) SerializeGremlinValue() (any, error) {
	return "ptr:" + p.Value, nil
}

func (p *testPointerSerializer) DeserializeGremlinValue(value any) error {
	encoded, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", value)
	}
	p.Value = strings.TrimPrefix(encoded, "ptr:")
	return nil
}

var errSerializerBoom = errors.New("boom")

type testFailingSerializer struct{}

func (testFailingSerializer) SerializeGremlinValue() (any, error) {
	return nil, errSerializerBoom
}

func (*testFailingSerializer) DeserializeGremlinValue(any) error {
	return errSerializerBoom
}

type testVertexWithSerializer struct {
	gsmtypes.Vertex
	Name       string                `gremlin:"name"`
	Attributes testAttributes        `gremlin:"attributes"`
	Optional   *testAttributes       `gremlin:"optional_attributes"`
	Custom     testPointerSerializer `gremlin:"custom"`
	Empty      testAttributes        `gremlin:"empty,omitempty"`
}

type testVertexWithFailingSerializer struct {
	gsmtypes.Vertex
	Broken testFailingSerializer `gremlin:"broken"`
}

func TestStructToMapSerialization(t *testing.T) {
	t.Parallel()
	t.Run(
		"SerializesCustomTypes", func(t *testing.T) {
			t.Parallel()
			v := testVertexWithSerializer{
				Name:       "test",
				Attributes: testAttributes{"env": "prod", "region": "us-east-1"},
				Custom:     testPointerSerializer{Value: "hello"},
			}
			mapValue, err := driver.StructToMapForTest(&v)
			if err != nil {
				t.Fatalf("Error converting struct to map: %v", err)
			}
			encoded, ok := mapValue["attributes"].(string)
			if !ok {
				t.Fatalf("attributes should be serialized to string, got %T", mapValue["attributes"])
			}
			var decoded testAttributes
			if err := json.Unmarshal([]byte(encoded), &decoded); err != nil {
				t.Fatalf("attributes should be valid JSON: %v", err)
			}
			if !reflect.DeepEqual(decoded, v.Attributes) {
				t.Errorf("attributes round trip mismatch: got %v want %v", decoded, v.Attributes)
			}
			if mapValue["custom"] != "ptr:hello" {
				t.Errorf("custom should be ptr:hello, got %v", mapValue["custom"])
			}
			if _, ok := mapValue["optional_attributes"]; ok {
				t.Errorf("nil pointer field should be skipped")
			}
			if _, ok := mapValue["empty"]; ok {
				t.Errorf("empty omitempty field should be skipped")
			}
		},
	)
	t.Run(
		"SerializesPointerField", func(t *testing.T) {
			t.Parallel()
			v := testVertexWithSerializer{
				Optional: &testAttributes{"a": "b"},
			}
			mapValue, err := driver.StructToMapForTest(&v)
			if err != nil {
				t.Fatalf("Error converting struct to map: %v", err)
			}
			if mapValue["optional_attributes"] != `{"a":"b"}` {
				t.Errorf(
					"optional_attributes should be serialized JSON, got %v",
					mapValue["optional_attributes"],
				)
			}
		},
	)
	t.Run(
		"SerializesNonAddressableValue", func(t *testing.T) {
			t.Parallel()
			// Passing the struct by value means fields are not addressable;
			// pointer-receiver serializers must still be invoked via a copy.
			v := testVertexWithSerializer{
				Custom: testPointerSerializer{Value: "copy"},
			}
			mapValue, err := driver.StructToMapForTest(v)
			if err != nil {
				t.Fatalf("Error converting struct to map: %v", err)
			}
			if mapValue["custom"] != "ptr:copy" {
				t.Errorf("custom should be ptr:copy, got %v", mapValue["custom"])
			}
		},
	)
	t.Run(
		"SerializerErrorPropagates", func(t *testing.T) {
			t.Parallel()
			v := testVertexWithFailingSerializer{}
			_, err := driver.StructToMapForTest(&v)
			if !errors.Is(err, errSerializerBoom) {
				t.Errorf("expected serializer error, got %v", err)
			}
			if err != nil && !strings.Contains(err.Error(), "broken") {
				t.Errorf("error should name the failing property, got %v", err)
			}
		},
	)
}

func TestUnloadDeserialization(t *testing.T) {
	t.Parallel()
	t.Run(
		"DeserializesCustomTypes", func(t *testing.T) {
			t.Parallel()
			var v testVertexWithSerializer
			err := driver.UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{
					Data: map[any]any{
						"id":                  "1",
						"name":                "test",
						"attributes":          `{"env":"prod"}`,
						"optional_attributes": `{"a":"b"}`,
						"custom":              "ptr:hello",
					},
				},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if v.Attributes["env"] != "prod" {
				t.Errorf("attributes should be deserialized, got %v", v.Attributes)
			}
			if v.Optional == nil || (*v.Optional)["a"] != "b" {
				t.Errorf("optional_attributes should populate pointer field, got %v", v.Optional)
			}
			if v.Custom.Value != "hello" {
				t.Errorf("custom should be hello, got %v", v.Custom.Value)
			}
		},
	)
	t.Run(
		"DeserializerErrorPropagates", func(t *testing.T) {
			t.Parallel()
			var v testVertexWithFailingSerializer
			err := driver.UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{
					Data: map[any]any{
						"broken": "anything",
					},
				},
			)
			if !errors.Is(err, errSerializerBoom) {
				t.Errorf("expected deserializer error, got %v", err)
			}
		},
	)
	t.Run(
		"InvalidValueTypeErrorPropagates", func(t *testing.T) {
			t.Parallel()
			var v testVertexWithSerializer
			err := driver.UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{
					Data: map[any]any{
						"attributes": 42,
					},
				},
			)
			if err == nil {
				t.Errorf("expected error for non-string property value")
			}
		},
	)
	t.Run(
		"RoundTrip", func(t *testing.T) {
			t.Parallel()
			original := testVertexWithSerializer{
				Name:       "round-trip",
				Attributes: testAttributes{"k1": "v1", "k2": "v2"},
				Custom:     testPointerSerializer{Value: "rt"},
			}
			mapValue, err := driver.StructToMapForTest(&original)
			if err != nil {
				t.Fatalf("Error converting struct to map: %v", err)
			}
			data := make(map[any]any, len(mapValue))
			for k, val := range mapValue {
				data[k] = val
			}
			var loaded testVertexWithSerializer
			err = driver.UnloadGremlinResultIntoStruct(&loaded, &gremlingo.Result{Data: data})
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if !reflect.DeepEqual(loaded.Attributes, original.Attributes) {
				t.Errorf(
					"attributes round trip mismatch: got %v want %v",
					loaded.Attributes, original.Attributes,
				)
			}
			if loaded.Custom.Value != original.Custom.Value {
				t.Errorf(
					"custom round trip mismatch: got %v want %v",
					loaded.Custom.Value, original.Custom.Value,
				)
			}
		},
	)
}

func TestSerializerIntegration(t *testing.T) {
	db, err := driver.Open(
		DbURL, driver.Config{
			Driver: dbDriver,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	created := &testVertexWithSerializer{
		Name:       "serializer-integration",
		Attributes: testAttributes{"env": "prod", "region": "us-east-1"},
		Custom:     testPointerSerializer{Value: "integration"},
	}
	if err := driver.Create(db, created); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() {
		if err := driver.Model[testVertexWithSerializer](db).IDs(created.ID).Delete(); err != nil {
			t.Errorf("cleanup Delete() error = %v", err)
		}
	}()

	loaded, err := driver.Model[testVertexWithSerializer](db).ID(created.ID)
	if err != nil {
		t.Fatalf("ID() error = %v", err)
	}
	if !reflect.DeepEqual(loaded.Attributes, created.Attributes) {
		t.Errorf(
			"attributes round trip mismatch: got %v want %v",
			loaded.Attributes, created.Attributes,
		)
	}
	if loaded.Custom.Value != created.Custom.Value {
		t.Errorf(
			"custom round trip mismatch: got %v want %v",
			loaded.Custom.Value, created.Custom.Value,
		)
	}

	// Where serializes the custom type, so equality matches the stored string.
	found, err := driver.Model[testVertexWithSerializer](db).
		Where("attributes", comparator.EQ, created.Attributes).
		Where("name", comparator.EQ, created.Name).
		Take()
	if err != nil {
		t.Fatalf("Where() on serialized property error = %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("Where() found vertex %v, want %v", found.ID, created.ID)
	}

	// Updates serializes custom-typed values before writing.
	updatedAttributes := testAttributes{"env": "staging"}
	err = driver.Model[testVertexWithSerializer](db).
		IDs(created.ID).
		Updates(map[string]any{"attributes": updatedAttributes})
	if err != nil {
		t.Fatalf("Updates() error = %v", err)
	}
	reloaded, err := driver.Model[testVertexWithSerializer](db).ID(created.ID)
	if err != nil {
		t.Fatalf("ID() after update error = %v", err)
	}
	if !reflect.DeepEqual(reloaded.Attributes, updatedAttributes) {
		t.Errorf(
			"attributes after update mismatch: got %v want %v",
			reloaded.Attributes, updatedAttributes,
		)
	}
}

func TestMaybeSerializeValue(t *testing.T) {
	t.Parallel()
	var nilAttributes *testAttributes
	tests := []struct {
		testName       string
		value          any
		want           any
		wantSerialized bool
		wantErr        bool
	}{
		{
			testName:       "PlainValuePassesThrough",
			value:          "plain",
			want:           "plain",
			wantSerialized: false,
		},
		{
			testName:       "ValueReceiverSerialized",
			value:          testAttributes{"a": "b"},
			want:           `{"a":"b"}`,
			wantSerialized: true,
		},
		{
			testName:       "PointerSerialized",
			value:          &testPointerSerializer{Value: "x"},
			want:           "ptr:x",
			wantSerialized: true,
		},
		{
			testName:       "NonPointerValueOfPointerReceiverSerialized",
			value:          testPointerSerializer{Value: "y"},
			want:           "ptr:y",
			wantSerialized: true,
		},
		{
			testName:       "NilPassesThrough",
			value:          nil,
			want:           nil,
			wantSerialized: false,
		},
		{
			testName:       "TypedNilPointerPassesThrough",
			value:          nilAttributes,
			want:           nilAttributes,
			wantSerialized: false,
		},
		{
			testName:       "SerializerErrorPropagates",
			value:          testFailingSerializer{},
			wantSerialized: true,
			wantErr:        true,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.testName, func(t *testing.T) {
				t.Parallel()
				got, serialized, err := driver.MaybeSerializeValueForTest(tt.value)
				if (err != nil) != tt.wantErr {
					t.Fatalf("maybeSerializeValue() error = %v, wantErr %v", err, tt.wantErr)
				}
				if serialized != tt.wantSerialized {
					t.Errorf(
						"maybeSerializeValue() serialized = %v, want %v",
						serialized, tt.wantSerialized,
					)
				}
				if tt.wantErr {
					return
				}
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("maybeSerializeValue() = %v, want %v", got, tt.want)
				}
			},
		)
	}
}

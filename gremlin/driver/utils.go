package driver

import (
	"errors"
	"fmt"
	"maps"
	"reflect"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/gobeam/stringy"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

var (
	anonymousTraversal = gremlingo.T__
	P                  = gremlingo.P
	Order              = gremlingo.Order
	Scope              = gremlingo.Scope
)

type GremlinOrder int

const (
	Asc GremlinOrder = iota
	Desc
)

// getStructName takes a generic type T, confirms it's a struct, and returns its name
func getStructName[T any]() (string, error) {
	var s T
	t := reflect.TypeOf(s)
	// Check if T is a struct type
	if t.Kind() != reflect.Struct {
		return "", fmt.Errorf("type %s is not a struct, it's a %s", t.Name(), t.Kind())
	}
	return t.Name(), nil
}

// UnloadGremlinResultIntoStruct unloads a gremlin result into a struct
// it will recursively unload the result into the struct
// v any must be a pointer to a struct
// result *gremlingo.Result is the gremlin result to unload which must be a map
// note the struct must have gremlin tags on the fields to be unloaded
func UnloadGremlinResultIntoStruct(
	v any,
	result *gremlingo.Result,
) error {
	mapResult, ok := result.GetInterface().(map[any]any)
	if !ok {
		return errors.New("result is not a map")
	}
	// make string map
	stringMap := make(map[string]any, len(mapResult))
	for key, value := range mapResult {
		keyStr, keyOk := key.(string)
		if !keyOk {
			return errors.New("gremlin key is not a string")
		}
		stringMap[keyStr] = value
	}
	rv := reflect.ValueOf(v)

	if rv.Kind() != reflect.Ptr {
		return errors.New("v must be a pointer")
	}
	recursivelyUnloadIntoStruct(v, stringMap)
	return nil
}

func recursivelyUnloadIntoStruct(v any, stringMap map[string]any) {
	rv := reflect.ValueOf(v).Elem()
	rt := rv.Type()

	for i := range rv.NumField() {
		field := rv.Field(i)
		fieldType := rt.Field(i)
		// handle anonymous Vertex field
		if fieldType.Anonymous {
			recursivelyUnloadIntoStruct(field.Addr().Interface(), stringMap)
		}

		gremlinTag := rt.Field(i).Tag.Get(gsmtypes.GremlinTag)
		gremlinSubTraversalTag := rt.Field(i).Tag.Get(gsmtypes.GremlinSubTraversalTag)
		if !field.CanInterface() || !field.CanSet() || ((gremlinTag == "" || gremlinTag == "-") &&
			(gremlinSubTraversalTag == "" || gremlinSubTraversalTag == "-")) {
			continue
		}
		_, gremlinTagOk := stringMap[gremlinTag]
		_, subtraversalTagOk := stringMap[gremlinSubTraversalTag]
		if !gremlinTagOk && !subtraversalTagOk {
			continue
		}
		if subtraversalTagOk {
			gremlinTag = gremlinSubTraversalTag
		}
		gType := reflect.TypeOf(stringMap[gremlinTag])

		switch {
		case gType.ConvertibleTo(field.Type()):
			field.Set(reflect.ValueOf(stringMap[gremlinTag]).Convert(field.Type()))
		case gType.Kind() == reflect.Slice:
			strSlice := stringMap[gremlinTag].([]any) //nolint:errcheck // we already validated via reflect type check
			slice := reflect.MakeSlice(
				field.Type(), len(strSlice), len(strSlice),
			)
			for i, v := range strSlice {
				slice.Index(i).Set(reflect.ValueOf(v).Convert(field.Type().Elem()))
			}
			field.Set(slice)
		case field.Type().Kind() == reflect.Slice && gType.ConvertibleTo(field.Type().Elem()):
			// Handle case where field is a slice but gremlin result is a single value
			// Create a slice with one element
			slice := reflect.MakeSlice(field.Type(), 1, 1)
			slice.Index(0).Set(reflect.ValueOf(stringMap[gremlinTag]).Convert(field.Type().Elem()))
			field.Set(slice)
		}
	}
}

func getLabelFromVertex(value gsmtypes.VertexType) string {
	label := value.Label()
	if label == "" {
		// Get the concrete type from the interface
		concreteType := reflect.ValueOf(value).Type()
		// Handle pointer types
		if concreteType.Kind() == reflect.Ptr {
			concreteType = concreteType.Elem()
		}
		return stringy.New(concreteType.Name()).SnakeCase().ToLower()
	}
	return label
}

func getLabelFromEdge(value gsmtypes.EdgeType) string {
	label := value.Label()
	if label == "" {
		// Get the concrete type from the interface
		concreteType := reflect.ValueOf(value).Type()
		// Handle pointer types
		if concreteType.Kind() == reflect.Ptr {
			concreteType = concreteType.Elem()
		}
		return stringy.New(concreteType.Name()).SnakeCase().ToLower()
	}
	return label
}

// structToMap converts a struct to a map[string]any and returns the label and the map
// the label is determined by calling Label() method if available, otherwise the name of the struct converted to snake case
// the map is the map of the struct
// the error is the error if any
func structToMap( //nolint:gocognit
	value any,
) (string, map[string]any, error) {
	mapValue := make(map[string]any)
	var err error

	// Get the reflection value
	rv := reflect.ValueOf(value)

	// Check if it's a pointer and get the underlying value
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return "", nil, errors.New("value is not a struct")
	}
	// Get the type information
	rt := rv.Type()

	// Get the label using the helper function
	var label string
	if vertexType, vertexOk := value.(gsmtypes.VertexType); vertexOk {
		label = getLabelFromVertex(vertexType)
	} else if edgeType, edgeOk := value.(gsmtypes.EdgeType); edgeOk {
		label = getLabelFromEdge(edgeType)
	} else {
		return "", nil, errors.New("value must implement either VertexType or EdgeType")
	}

	// Loop through all fields
	for i := range rv.NumField() {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		if field.Anonymous && fieldValue.Kind() == reflect.Struct {
			// Recursively process the anonymous struct
			_, anonymousMap, structMapErr := structToMap(fieldValue.Interface())
			if structMapErr != nil {
				return "", nil, fmt.Errorf(
					"error processing anonymous field %s: %w",
					field.Name,
					err,
				)
			}
			maps.Copy(mapValue, anonymousMap)
			continue
		}

		// Skip sub traversal tags so this doesnt get included when creating vertices
		if field.Tag.Get(gsmtypes.GremlinSubTraversalTag) != "" {
			continue
		}

		// Get the gremlin tag
		gremlinTag := field.Tag.Get(gsmtypes.GremlinTag)

		// Skip if no gremlin tag or if field is not exported
		if gremlinTag == "" || gremlinTag == "-" || !fieldValue.CanInterface() {
			continue
		}

		// Parse tag options (e.g., "field_name,omitempty")
		tagParts := parseGremlinTag(gremlinTag)

		// Check if field is a pointer and is nil (unset)
		if fieldValue.Kind() == reflect.Ptr {
			if fieldValue.IsNil() {
				// Skip unset pointer fields
				continue
			}
			// Dereference the pointer to get the actual value
			fieldValue = fieldValue.Elem()
		}

		// If omitempty is set, skip zero values
		if tagParts.omitEmpty && fieldValue.IsZero() {
			continue
		}

		// Get the field value
		fieldInterface := fieldValue.Interface()

		// Use the gremlin tag as the property name
		mapValue[tagParts.name] = fieldInterface
	}

	return label, mapValue, nil
}

func validateStructPointerWithAnonymousVertex(value any) error {
	rv := reflect.ValueOf(value)

	// Check if it's a pointer
	if rv.Kind() != reflect.Ptr {
		return errors.New("value must be a pointer")
	}

	// Check if it's a nil pointer
	if rv.IsNil() {
		return errors.New("value cannot be nil")
	}

	// Check if it points to a struct
	if rv.Elem().Kind() != reflect.Struct {
		return errors.New("value must point to a struct")
	}

	// Get the struct type
	rt := rv.Elem().Type()

	// Check for anonymous Vertex field
	for i := range rv.Elem().NumField() {
		field := rt.Field(i)

		if field.Anonymous && field.Type == reflect.TypeFor[gsmtypes.Vertex]() {
			return nil
		}
	}

	return errors.New("struct must contain anonymous types.Vertex field")
}

func getStructFieldNameAndType[T any](tag string) (string, reflect.Type, error) {
	var s T
	rt := reflect.TypeOf(s)
	for i := range rt.NumField() {
		field := rt.Field(i)
		if field.Tag.Get(gsmtypes.GremlinTag) == tag {
			return field.Name, field.Type, nil
		}
	}
	return "", nil, errors.New("field not found")
}

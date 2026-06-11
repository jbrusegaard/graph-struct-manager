package driver

import (
	"errors"
	"fmt"
	"reflect"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
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
	if result == nil {
		return errors.New("gremlin result is nil")
	}
	mapResult, ok := result.GetInterface().(map[any]any)
	if !ok {
		return errors.New("result is not a map")
	}
	return unloadGremlinMapIntoStruct(v, mapResult)
}

// unloadGremlinMapIntoStruct unloads a gremlin result map into a struct.
// v must be a non-nil pointer to a struct with gremlin tags on the fields.
func unloadGremlinMapIntoStruct(v any, mapResult map[any]any) error {
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

	if rv.Kind() != reflect.Pointer {
		return errors.New("v must be a pointer")
	}

	if rv.IsNil() {
		return errors.New("v must be a non-nil pointer")
	}

	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		return errors.New("v must point to a struct")
	}

	unmappedCollector, collectUnmapped := v.(gsmtypes.UnmappedPropertiesType)
	var usedKeys map[string]struct{}
	if collectUnmapped {
		usedKeys = make(map[string]struct{}, len(stringMap))
	}

	unloadSchemaFields(elem, stringMap, usedKeys)

	if collectUnmapped {
		extras := make(map[string]any, len(stringMap))
		for key, value := range stringMap {
			if _, ok := usedKeys[key]; ok {
				continue
			}
			extras[key] = value
		}
		unmappedCollector.SetUnmappedProperties(extras)
	}
	return nil
}

// unloadSchemaFields sets struct fields from the gremlin result map using
// the cached field plan. Keys consumed by mapped fields are recorded in
// usedKeys when it is non-nil.
func unloadSchemaFields(
	elem reflect.Value,
	stringMap map[string]any,
	usedKeys map[string]struct{},
) {
	schema := schemaFor(elem.Type())
	for i := range schema.unloadFields {
		fieldSchema := &schema.unloadFields[i]
		field := elem.FieldByIndex(fieldSchema.index)
		if !field.CanInterface() || !field.CanSet() {
			continue
		}

		// gremlinEdge fields are loaded via Preload and keyed by the Go field name.
		if fieldSchema.isEdge {
			value, ok := stringMap[fieldSchema.goName]
			if !ok {
				continue
			}
			if usedKeys != nil {
				usedKeys[fieldSchema.goName] = struct{}{}
			}
			setEdgeFieldFromValue(field, value)
			continue
		}

		selectedKey, ok := selectGremlinKey(
			fieldSchema.tagName,
			fieldSchema.subTraversalTag,
			stringMap,
		)
		if !ok {
			continue
		}
		if usedKeys != nil {
			usedKeys[selectedKey] = struct{}{}
		}
		setFieldFromValue(field, stringMap[selectedKey])
	}
}

var unmappedPropertiesType = reflect.TypeFor[gsmtypes.UnmappedPropertiesType]()

func typeImplementsUnmappedProperties(rt reflect.Type) bool {
	if rt == nil {
		return false
	}
	if rt.Implements(unmappedPropertiesType) {
		return true
	}
	if rt.Kind() != reflect.Pointer && reflect.PointerTo(rt).Implements(unmappedPropertiesType) {
		return true
	}
	return false
}

func selectGremlinKey(tagName, subTraversalTag string, stringMap map[string]any) (string, bool) {
	// Pick the key to use (subtraversal wins if present in the result).
	if subTraversalTag != "" && subTraversalTag != "-" {
		if _, ok := stringMap[subTraversalTag]; ok {
			return subTraversalTag, true
		}
	}
	if tagName != "" && tagName != "-" {
		if _, ok := stringMap[tagName]; ok {
			return tagName, true
		}
	}
	return "", false
}

func setFieldFromValue(field reflect.Value, value any) {
	// Assign a Gremlin value into a field, handling slice conversions.
	gType := reflect.TypeOf(value)

	switch {
	case gType.ConvertibleTo(field.Type()):
		field.Set(reflect.ValueOf(value).Convert(field.Type()))
	case gType.Kind() == reflect.Slice:
		strSlice := value.([]any) //nolint:errcheck // we already validated via reflect type check
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
		slice.Index(0).Set(
			reflect.ValueOf(value).Convert(field.Type().Elem()),
		)
		field.Set(slice)
	}
}

// setEdgeFieldFromValue assigns preloaded related vertices into a gremlinEdge
// tagged field. The gremlin value is a folded list of vertex value maps. For
// slice fields all related vertices are loaded, otherwise the first one is.
func setEdgeFieldFromValue(field reflect.Value, value any) {
	relatedMaps, ok := value.([]any)
	if !ok {
		return
	}
	fieldType := field.Type()
	switch fieldType.Kind() { //nolint:exhaustive // only struct-like edge field types are supported
	case reflect.Slice:
		slice := reflect.MakeSlice(fieldType, 0, len(relatedMaps))
		for _, relatedMap := range relatedMaps {
			related, relatedOk := newStructFromGremlinMap(fieldType.Elem(), relatedMap)
			if !relatedOk {
				continue
			}
			slice = reflect.Append(slice, related)
		}
		field.Set(slice)
	case reflect.Pointer, reflect.Struct:
		if len(relatedMaps) == 0 {
			return
		}
		related, relatedOk := newStructFromGremlinMap(fieldType, relatedMaps[0])
		if relatedOk {
			field.Set(related)
		}
	}
}

// newStructFromGremlinMap builds a value of elemType (a struct or pointer to
// struct) from a gremlin value map result.
func newStructFromGremlinMap(elemType reflect.Type, value any) (reflect.Value, bool) {
	mapValue, ok := value.(map[any]any)
	if !ok {
		return reflect.Value{}, false
	}
	isPointer := elemType.Kind() == reflect.Pointer
	structType := elemType
	if isPointer {
		structType = elemType.Elem()
	}
	if structType.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}
	structPointer := reflect.New(structType)
	if err := unloadGremlinMapIntoStruct(structPointer.Interface(), mapValue); err != nil {
		return reflect.Value{}, false
	}
	if isPointer {
		return structPointer, true
	}
	return structPointer.Elem(), true
}

func getLabelFromVertex(value any) string {
	if value == nil {
		return ""
	}
	// Calling Label() on the actual value preserves instance-specific labels.
	if labeler, ok := value.(gsmtypes.CustomLabelType); ok {
		if label := labeler.Label(); label != "" {
			return label
		}
		return schemaFor(reflect.TypeOf(value)).snakeName
	}
	// The value itself does not implement CustomLabelType; the cached label
	// covers the pointer-receiver case and the snake-case fallback.
	return schemaFor(reflect.TypeOf(value)).zeroLabel
}

// func getLabelFromEdge(value gsmtypes.EdgeType) string {
// 	label := value.Label()
// 	if label == "" {
// 		// Get the concrete type from the interface
// 		concreteType := reflect.ValueOf(value).Type()
// 		// Handle pointer types
// 		if concreteType.Kind() == reflect.Ptr {
// 			concreteType = concreteType.Elem()
// 		}
// 		return stringy.New(concreteType.Name()).SnakeCase().ToLower()
// 	}
// 	return label
// }

// structToMap converts a struct to a map[string]any and returns the label and the map
// the label is determined by calling Label() method if available, otherwise the name of the struct converted to snake case
// the map is the map of the struct
// the error is the error if any
func structToMap(
	value any,
) (map[string]any, error) {
	// Get the reflection value
	rv := reflect.ValueOf(value)

	// Check if it's a pointer and get the underlying value
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return nil, errors.New("value is not a struct")
	}

	schema := schemaFor(rv.Type())
	mapValue := make(map[string]any, len(schema.mapFields))

	for i := range schema.mapFields {
		fieldSchema := &schema.mapFields[i]
		fieldValue := rv.FieldByIndex(fieldSchema.index)

		// Check if field is a pointer and is nil (unset)
		if fieldValue.Kind() == reflect.Pointer {
			if fieldValue.IsNil() {
				// Skip unset pointer fields
				continue
			}
			// Dereference the pointer to get the actual value
			fieldValue = fieldValue.Elem()
		}

		// If omitempty is set, skip zero values
		if fieldSchema.omitEmpty && fieldValue.IsZero() {
			continue
		}

		// Use the gremlin tag as the property name
		mapValue[fieldSchema.tagName] = fieldValue.Interface()
	}

	return mapValue, nil
}

func validateStructPointerWithAnonymousVertex(value any) error {
	rv := reflect.ValueOf(value)

	// Check if it's a pointer
	if rv.Kind() != reflect.Pointer {
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

func nextWithDefaultValue[T any](
	query *gremlingo.GraphTraversal,
	defaultVal T,
) (*gremlingo.Result, T, error) {
	set, err := query.GetResultSet()
	if err != nil {
		return nil, defaultVal, err
	}
	if set.IsEmpty() {
		return nil, defaultVal, nil
	}
	result, _, err := set.One()
	if err != nil {
		return nil, defaultVal, err
	}
	return result, defaultVal, nil
}

func SliceToAnySlice[T any](slice []T) []any {
	anySlice := make([]any, len(slice))
	for i, v := range slice {
		anySlice[i] = v
	}
	return anySlice
}

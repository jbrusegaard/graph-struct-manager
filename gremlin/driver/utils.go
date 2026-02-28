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

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errors.New("v must be a non-nil pointer")
	}

	usedKeys := make(map[string]struct{}, len(mapResult))

	recursivelyUnloadIntoStruct(rv.Elem(), mapResult, usedKeys)

	if unmappedCollector, ok2 := v.(gsmtypes.UnmappedPropertiesType); ok2 {
		extras := make(map[string]any, len(mapResult)-len(usedKeys))
		for key, value := range mapResult {
			keyStr, strOk := key.(string)
			if !strOk {
				continue
			}
			if _, used := usedKeys[keyStr]; !used {
				extras[keyStr] = value
			}
		}
		unmappedCollector.SetUnmappedProperties(extras)
	}

	return nil
}

func recursivelyUnloadIntoStruct(
	rv reflect.Value,
	stringMap map[any]any,
	usedKeys map[string]struct{},
) {
	rt := rv.Type()

	for i := range rv.NumField() {
		field := rv.Field(i)
		fieldType := rt.Field(i)

		if fieldType.Anonymous && field.Kind() == reflect.Struct {
			recursivelyUnloadIntoStruct(field, stringMap, usedKeys)
		}

		unloadFieldFromResult(
			field,
			fieldType,
			stringMap,
			usedKeys,
		)
	}
}

// typeImplementsUnmappedProperties checks if the given type implements UnmappedPropertiesType
func typeImplementsUnmappedProperties(rt reflect.Type) bool {
	if rt == nil {
		return false
	}
	unmappedType := reflect.TypeFor[gsmtypes.UnmappedPropertiesType]()
	if rt.Implements(unmappedType) {
		return true
	}
	if rt.Kind() != reflect.Ptr && reflect.PointerTo(rt).Implements(unmappedType) {
		return true
	}
	return false
}

func unloadFieldFromResult(
	field reflect.Value,
	fieldType reflect.StructField,
	stringMap map[any]any,
	usedKeys map[string]struct{},
) {
	if !field.CanInterface() || !field.CanSet() {
		return
	}

	gremlinTag := fieldType.Tag.Get(gsmtypes.GremlinTag)
	gremlinSubTraversalTag := fieldType.Tag.Get(gsmtypes.GremlinSubTraversalTag)

	tagName := gremlinTag
	if tagName != "" && tagName != "-" {
		parts := parseGremlinTag(gremlinTag)
		tagName = parts.name
	}

	if (tagName == "" || tagName == "-") &&
		(gremlinSubTraversalTag == "" || gremlinSubTraversalTag == "-") {
		return
	}

	selectedKey, ok := selectGremlinKey(tagName, gremlinSubTraversalTag, stringMap)
	if !ok {
		return
	}

	usedKeys[selectedKey] = struct{}{}

	value := stringMap[selectedKey]
	setFieldFromValue(field, value)
}

func selectGremlinKey(tagName, subTraversalTag string, stringMap map[any]any) (string, bool) {
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
	if !field.CanSet() {
		return
	}

	gType := reflect.TypeOf(value)
	if gType == nil {
		return
	}

	targetType := field.Type()

	// Direct assignment - most common case
	if gType.AssignableTo(targetType) {
		field.Set(reflect.ValueOf(value))
		return
	}

	// Convertible types (e.g., string to string type, int to int64)
	if gType.ConvertibleTo(targetType) {
		field.Set(reflect.ValueOf(value).Convert(targetType))
		return
	}

	// Handle float to int conversion
	if isFloat(gType) && targetType.Kind() >= reflect.Int && targetType.Kind() <= reflect.Uint32 {
		setFieldAsInt(field, value, gType, targetType)
		return
	}

	// Handle slice conversions
	if gType.Kind() == reflect.Slice {
		handleSliceToSliceConversion(field, targetType, value)
		return
	}

	if targetType.Kind() == reflect.Slice {
		setFieldFromSingleValue(field, targetType, value, gType)
	}
}

func isFloat(gType reflect.Type) bool {
	return gType.Kind() == reflect.Float32 || gType.Kind() == reflect.Float64
}

func setFieldAsInt(field reflect.Value, value any, _ reflect.Type, targetType reflect.Type) {
	fval := reflect.ValueOf(value).Float()
	switch targetType.Kind() { //nolint:exhaustive // only handling int conversions
	case reflect.Int:
		field.SetInt(int64(fval))
	case reflect.Int8:
		field.SetInt(int64(fval))
	case reflect.Int16:
		field.SetInt(int64(fval))
	case reflect.Int32:
		field.SetInt(int64(fval))
	case reflect.Uint:
		field.SetUint(uint64(fval))
	case reflect.Uint8:
		field.SetUint(uint64(fval))
	case reflect.Uint16:
		field.SetUint(uint64(fval))
	case reflect.Uint32:
		field.SetUint(uint64(fval))
	}
}

func handleSliceToSliceConversion(field reflect.Value, targetType reflect.Type, value any) {
	strSlice := value.([]any) //nolint:errcheck // validated by caller via gType.Kind() check
	slice := reflect.MakeSlice(targetType, len(strSlice), cap(strSlice))

	elemType := targetType.Elem()
	for i, v := range strSlice {
		vType := reflect.TypeOf(v)
		if vType == nil {
			continue
		}

		// Handle float to int conversion for slice elements
		if isFloat(vType) && elemType.Kind() >= reflect.Int && elemType.Kind() <= reflect.Uint32 {
			fval := reflect.ValueOf(v).Float()
			switch elemType.Kind() { //nolint:exhaustive // only handling int/uint types here
			case reflect.Int:
				slice.Index(i).SetInt(int64(fval))
			case reflect.Int8:
				slice.Index(i).SetInt(int64(fval))
			case reflect.Int16:
				slice.Index(i).SetInt(int64(fval))
			case reflect.Int32:
				slice.Index(i).SetInt(int64(fval))
			case reflect.Uint:
				slice.Index(i).SetUint(uint64(fval))
			case reflect.Uint8:
				slice.Index(i).SetUint(uint64(fval))
			case reflect.Uint16:
				slice.Index(i).SetUint(uint64(fval))
			case reflect.Uint32:
				slice.Index(i).SetUint(uint64(fval))
			}
			continue
		}

		if !vType.AssignableTo(elemType) && !vType.ConvertibleTo(elemType) {
			continue
		}
		slice.Index(i).Set(reflect.ValueOf(v))
		if vType.ConvertibleTo(elemType) {
			slice.Index(i).Set(reflect.ValueOf(v).Convert(elemType))
		}
	}

	field.Set(slice)
}

func setFieldFromSingleValue(
	field reflect.Value,
	targetType reflect.Type,
	value any,
	gType reflect.Type,
) {
	elemType := targetType.Elem()
	slice := reflect.MakeSlice(targetType, 1, 1)

	// Handle float to int conversion for single value to slice
	if isFloat(gType) && elemType.Kind() >= reflect.Int && elemType.Kind() <= reflect.Uint32 {
		fval := reflect.ValueOf(value).Float()
		switch elemType.Kind() { //nolint:exhaustive // only handling int/uint types here
		case reflect.Int:
			slice.Index(0).SetInt(int64(fval))
		case reflect.Int8:
			slice.Index(0).SetInt(int64(fval))
		case reflect.Int16:
			slice.Index(0).SetInt(int64(fval))
		case reflect.Int32:
			slice.Index(0).SetInt(int64(fval))
		case reflect.Uint:
			slice.Index(0).SetUint(uint64(fval))
		case reflect.Uint8:
			slice.Index(0).SetUint(uint64(fval))
		case reflect.Uint16:
			slice.Index(0).SetUint(uint64(fval))
		case reflect.Uint32:
			slice.Index(0).SetUint(uint64(fval))
		}
		field.Set(slice)
		return
	}

	if gType.AssignableTo(elemType) || gType.ConvertibleTo(elemType) {
		slice.Index(0).Set(reflect.ValueOf(value))
		if gType.ConvertibleTo(elemType) {
			slice.Index(0).Set(reflect.ValueOf(value).Convert(elemType))
		}
		field.Set(slice)
	}
}

func getLabelFromVertex(value any) string {
	var label string
	if value != nil { //nolint:nestif
		vertexValue, ok := value.(gsmtypes.CustomLabelType)
		if ok {
			label = vertexValue.Label()
		} else {
			customLabelType := reflect.TypeFor[gsmtypes.CustomLabelType]()
			valueType := reflect.TypeOf(value)
			if valueType != nil && valueType.Kind() != reflect.Ptr &&
				reflect.PointerTo(valueType).Implements(customLabelType) {
				pointerValue := reflect.New(valueType)
				if pointerLabel, ptrOk := pointerValue.Interface().(gsmtypes.CustomLabelType); ptrOk {
					label = pointerLabel.Label()
				}
			}
		}
	}
	if label == "" {
		// Get the concrete type from the interface
		concreteValue := reflect.ValueOf(value)
		if !concreteValue.IsValid() {
			return ""
		}
		concreteType := concreteValue.Type()
		// Handle pointer types
		if concreteType.Kind() == reflect.Ptr {
			concreteType = concreteType.Elem()
		}
		return stringy.New(concreteType.Name()).SnakeCase().ToLower()
	}
	return label
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
func structToMap( //nolint:gocognit
	value any,
) (map[string]any, error) {
	mapValue := make(map[string]any)

	// Get the reflection value
	rv := reflect.ValueOf(value)

	// Check if it's a pointer and get the underlying value
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return nil, errors.New("value is not a struct")
	}
	// Get the type information
	rt := rv.Type()

	// Loop through all fields
	for i := range rv.NumField() {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		if field.Anonymous && fieldValue.Kind() == reflect.Struct {
			// Recursively process the anonymous struct
			anonymousMap, structMapErr := structToMap(fieldValue.Interface())
			if structMapErr != nil {
				return nil, fmt.Errorf(
					"error processing anonymous field %s: %w",
					field.Name,
					structMapErr,
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
		if tagParts.unmapped {
			continue
		}

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

	return mapValue, nil
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

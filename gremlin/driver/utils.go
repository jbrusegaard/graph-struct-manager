package driver

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"sync"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/gobeam/stringy"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

// cachedFieldInfo holds pre-computed metadata for a single struct field,
// including fields flattened from anonymous embeddings.
type cachedFieldInfo struct {
	index           []int
	gremlinTag      string
	subTraversalTag string
	isUnmapped      bool
}

type cachedStructInfo struct {
	fields []cachedFieldInfo
}

var structInfoCache sync.Map

func getStructInfo(rt reflect.Type) *cachedStructInfo {
	if cached, ok := structInfoCache.Load(rt); ok {
		res, _ := cached.(*cachedStructInfo)
		return res
	}
	info := buildStructInfo(rt, nil)
	actual, _ := structInfoCache.LoadOrStore(rt, info)
	res, _ := actual.(*cachedStructInfo)
	return res
}

func buildStructInfo(rt reflect.Type, parentIndex []int) *cachedStructInfo {
	info := &cachedStructInfo{}
	for i := range rt.NumField() {
		field := rt.Field(i)
		idx := make([]int, len(parentIndex)+1)
		copy(idx, parentIndex)
		idx[len(parentIndex)] = i

		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			embedded := buildStructInfo(field.Type, idx)
			info.fields = append(info.fields, embedded.fields...)
			continue
		}

		if !field.IsExported() {
			continue
		}

		gremlinTag := field.Tag.Get(gsmtypes.GremlinTag)
		subTraversalTag := field.Tag.Get(gsmtypes.GremlinSubTraversalTag)

		tagParts := gremlinTagOptions{name: gremlinTag}
		if gremlinTag != "" {
			tagParts = parseGremlinTag(gremlinTag)
		}

		if tagParts.unmapped {
			info.fields = append(info.fields, cachedFieldInfo{
				index:      idx,
				isUnmapped: true,
			})
			continue
		}

		if (tagParts.name == "" || tagParts.name == "-") &&
			(subTraversalTag == "" || subTraversalTag == "-") {
			continue
		}

		info.fields = append(info.fields, cachedFieldInfo{
			index:           idx,
			gremlinTag:      tagParts.name,
			subTraversalTag: subTraversalTag,
		})
	}
	return info
}

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

	if rv.IsNil() {
		return errors.New("v must be a non-nil pointer")
	}

	usedKeys := make(map[string]struct{}, len(stringMap))
	extrasFields := make([]reflect.Value, 0)
	recursivelyUnloadIntoStruct(v, stringMap, usedKeys, &extrasFields)
	unmappedCollector, collectUnmapped := v.(gsmtypes.UnmappedPropertiesType)
	if collectUnmapped {
		extras := make(map[string]any, len(stringMap))
		for key, value := range stringMap {
			if _, ok = usedKeys[key]; ok {
				continue
			}
			extras[key] = value
		}
		unmappedCollector.SetUnmappedProperties(extras)
	}
	return nil
}

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

func recursivelyUnloadIntoStruct(
	v any,
	stringMap map[string]any,
	usedKeys map[string]struct{},
	extrasFields *[]reflect.Value,
) {
	rv := reflect.ValueOf(v).Elem()
	info := getStructInfo(rv.Type())

	for i := range info.fields {
		fi := &info.fields[i]
		field := rv.FieldByIndex(fi.index)

		if fi.isUnmapped {
			*extrasFields = append(*extrasFields, field)
			continue
		}

		selectedKey, ok := selectGremlinKey(fi.gremlinTag, fi.subTraversalTag, stringMap)
		if !ok {
			continue
		}
		usedKeys[selectedKey] = struct{}{}
		setFieldFromValue(field, stringMap[selectedKey])
	}
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
	if value == nil {
		return
	}

	rv := reflect.ValueOf(value)
	gType := rv.Type()
	fType := field.Type()

	if gType == fType {
		field.Set(rv)
		return
	}

	if fType.Kind() == reflect.Interface {
		field.Set(rv)
		return
	}

	switch {
	case gType.ConvertibleTo(fType):
		field.Set(rv.Convert(fType))
	case gType.Kind() == reflect.Slice:
		strSlice := value.([]any) //nolint:errcheck // we already validated via reflect type check
		slice := reflect.MakeSlice(fType, len(strSlice), len(strSlice))
		elemType := fType.Elem()
		for i, v := range strSlice {
			slice.Index(i).Set(reflect.ValueOf(v).Convert(elemType))
		}
		field.Set(slice)
	case fType.Kind() == reflect.Slice && gType.ConvertibleTo(fType.Elem()):
		slice := reflect.MakeSlice(fType, 1, 1)
		slice.Index(0).Set(rv.Convert(fType.Elem()))
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

package driver

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"

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

// NarrowedProperty records a gremlin property that arrived with several values
// for a field that holds one. The first value was kept and Discarded counts
// the rest.
type NarrowedProperty struct {
	Property  string
	Field     string
	Discarded int
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
	_, err := unloadGremlinMapIntoStruct(v, mapResult)
	return err
}

// UnloadGremlinResultIntoStructReportingNarrowed is UnloadGremlinResultIntoStruct
// and additionally reports the properties whose extra values were discarded to
// fit a single-valued field, so callers holding a logger can surface them.
func UnloadGremlinResultIntoStructReportingNarrowed(
	v any,
	result *gremlingo.Result,
) ([]NarrowedProperty, error) {
	if result == nil {
		return nil, errors.New("gremlin result is nil")
	}
	mapResult, ok := result.GetInterface().(map[any]any)
	if !ok {
		return nil, errors.New("result is not a map")
	}
	return unloadGremlinMapIntoStruct(v, mapResult)
}

// unloadGremlinMapIntoStruct unloads a gremlin result map into a struct.
// v must be a non-nil pointer to a struct with gremlin tags on the fields.
func unloadGremlinMapIntoStruct(v any, mapResult map[any]any) ([]NarrowedProperty, error) {
	// make string map
	stringMap := make(map[string]any, len(mapResult))
	for key, value := range mapResult {
		keyStr, keyOk := key.(string)
		if !keyOk {
			return nil, errors.New("gremlin key is not a string")
		}
		stringMap[keyStr] = value
	}
	rv := reflect.ValueOf(v)

	if rv.Kind() != reflect.Pointer {
		return nil, errors.New("v must be a pointer")
	}

	if rv.IsNil() {
		return nil, errors.New("v must be a non-nil pointer")
	}

	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		return nil, errors.New("v must point to a struct")
	}

	unmappedCollector, collectUnmapped := v.(gsmtypes.UnmappedPropertiesType)
	var usedKeys map[string]struct{}
	if collectUnmapped {
		usedKeys = make(map[string]struct{}, len(stringMap))
	}

	narrowed, err := unloadSchemaFields(elem, stringMap, usedKeys)
	if err != nil {
		return nil, err
	}

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
	return narrowed, nil
}

// unloadEdgeField loads a gremlinEdge field from the preloaded value keyed by
// the Go field name, recording the key as used when a used-keys set is being
// tracked. Missing keys leave the field untouched.
func unloadEdgeField(
	fieldSchema *fieldSchema,
	field reflect.Value,
	stringMap map[string]any,
	usedKeys map[string]struct{},
) {
	value, ok := stringMap[fieldSchema.goName]
	if !ok {
		return
	}
	if usedKeys != nil {
		usedKeys[fieldSchema.goName] = struct{}{}
	}
	setEdgeFieldFromValue(field, value)
}

// unloadSchemaFields sets struct fields from the gremlin result map using
// the cached field plan. Keys consumed by mapped fields are recorded in
// usedKeys when it is non-nil. The returned slice lists properties whose
// extra values were discarded to fit a single-valued field.
func unloadSchemaFields(
	elem reflect.Value,
	stringMap map[string]any,
	usedKeys map[string]struct{},
) ([]NarrowedProperty, error) {
	var narrowed []NarrowedProperty

	schema := schemaFor(elem.Type())
	for i := range schema.unloadFields {
		fieldSchema := &schema.unloadFields[i]
		field := elem.FieldByIndex(fieldSchema.index)
		if !field.CanInterface() || !field.CanSet() {
			continue
		}

		// gremlinEdge fields are loaded via Preload and keyed by the Go field name.
		if fieldSchema.isEdge {
			unloadEdgeField(fieldSchema, field, stringMap, usedKeys)
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
		if fieldSchema.isDeserializer {
			if err := deserializeGremlinValue(field, stringMap[selectedKey]); err != nil {
				return nil, fmt.Errorf(
					"deserialize property %q into field %s: %w",
					selectedKey, fieldSchema.goName, err,
				)
			}
			continue
		}
		narrowed = recordNarrowed(
			narrowed,
			selectedKey,
			fieldSchema.goName,
			setFieldFromValue(field, stringMap[selectedKey]),
		)
	}
	return narrowed, nil
}

// recordNarrowed appends a report when values were discarded to fit a
// single-valued field, and leaves the slice alone otherwise.
func recordNarrowed(
	narrowed []NarrowedProperty,
	property, field string,
	discarded int,
) []NarrowedProperty {
	if discarded <= 0 {
		return narrowed
	}
	return append(narrowed, NarrowedProperty{
		Property:  property,
		Field:     field,
		Discarded: discarded,
	})
}

// serializeGremlinValue converts a field value whose type implements
// gsmtypes.SerializerType into its string gremlin representation.
// Non-addressable values with pointer-receiver implementations are copied so
// the method can be invoked.
func serializeGremlinValue(rv reflect.Value) (string, error) {
	if serializer, ok := reflect.TypeAssert[gsmtypes.SerializerType](rv); ok {
		return serializer.SerializeGremlinValue()
	}
	if rv.CanAddr() {
		if serializer, ok := reflect.TypeAssert[gsmtypes.SerializerType](rv.Addr()); ok {
			return serializer.SerializeGremlinValue()
		}
	}
	ptr := reflect.New(rv.Type())
	ptr.Elem().Set(rv)
	serializer, ok := reflect.TypeAssert[gsmtypes.SerializerType](ptr)
	if !ok {
		return "", fmt.Errorf("type %s does not implement gsmtypes.SerializerType", rv.Type())
	}
	return serializer.SerializeGremlinValue()
}

// maybeSerializeValue serializes value when it implements
// gsmtypes.SerializerType, directly or through its pointer method set. The
// second return reports whether serialization happened; values that do not
// implement the interface pass through unchanged.
func maybeSerializeValue(value any) (any, bool, error) {
	if value == nil {
		return value, false, nil
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Pointer && rv.IsNil() {
		return value, false, nil
	}
	if serializer, ok := value.(gsmtypes.SerializerType); ok {
		serialized, err := serializer.SerializeGremlinValue()
		return serialized, true, err
	}
	if rv.Kind() == reflect.Pointer ||
		!reflect.PointerTo(rv.Type()).Implements(serializerInterfaceType) {
		return value, false, nil
	}
	serialized, err := serializeGremlinValue(rv)
	return serialized, true, err
}

// deserializeGremlinValue loads a raw gremlin property value into a field
// whose type implements gsmtypes.DeserializerType. Pointer fields are
// allocated before delegating to the implementation.
func deserializeGremlinValue(field reflect.Value, value any) error {
	target := field
	if field.Kind() == reflect.Pointer {
		target = reflect.New(field.Type().Elem()).Elem()
	}
	deserializer, ok := reflect.TypeAssert[gsmtypes.DeserializerType](target.Addr())
	if !ok {
		return fmt.Errorf("type %s does not implement gsmtypes.DeserializerType", field.Type())
	}
	if err := deserializer.DeserializeGremlinValue(value); err != nil {
		return err
	}
	if field.Kind() == reflect.Pointer {
		field.Set(target.Addr())
	}
	return nil
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

// setFieldFromValue assigns a gremlin property value to a struct field. The
// returned count is the number of values that were discarded to fit a
// multi-valued property into a single-valued field, and is zero in every
// other case.
func setFieldFromValue(field reflect.Value, value any) int {
	// Gremlin results are external data: nil, or a typed-nil pointer, carries
	// no value to assign and is skipped instead of converted. Unconvertible
	// values are likewise skipped rather than panicked on.
	src := concreteValue(reflect.ValueOf(value))
	if !src.IsValid() {
		return 0
	}

	// Nullable properties are modelled as pointer fields: convert to the
	// element type and allocate, otherwise the value is silently dropped.
	fieldType := field.Type()
	isPointer := fieldType.Kind() == reflect.Pointer
	targetType := fieldType
	if isPointer {
		targetType = fieldType.Elem()
	}

	converted, discarded, ok := convertFieldValue(targetType, src)
	if !ok {
		return 0
	}
	if isPointer {
		field.Set(allocatePointer(targetType, converted))

		return discarded
	}
	field.Set(converted)

	return discarded
}

// allocatePointer wraps converted in a new pointer to elemType, which must be
// converted's type.
func allocatePointer(elemType reflect.Type, converted reflect.Value) reflect.Value {
	pointer := reflect.New(elemType)
	pointer.Elem().Set(converted)
	return pointer
}

// concreteValue unwraps interfaces and non-nil pointers down to the value
// beneath them. Nil at any level yields an invalid value, which callers skip.
func concreteValue(v reflect.Value) reflect.Value {
	for v.IsValid() {
		switch v.Kind() { //nolint:exhaustive // only wrappers are unwrapped
		case reflect.Interface, reflect.Pointer:
			if v.IsNil() {
				return reflect.Value{}
			}
			v = v.Elem()
		default:
			return v
		}
	}
	return v
}

// safeConvertible reports whether a value of type from may be silently
// converted to type t. A string may only pair with string or []byte: reflect
// also classifies string -> []rune (== []int32) as convertible, mapping each
// character to its code point, so "80" would load into []int32 as {56, 48}.
// Integer -> string is formatted as decimal text by convertValue before this
// check; everything it rejects would otherwise arrive as rune mojibake.
// Legitimate conversions — numeric widening/narrowing, []byte <-> string —
// stay allowed.
func safeConvertible(from, t reflect.Type) bool {
	if !from.ConvertibleTo(t) {
		return false
	}
	if t.Kind() == reflect.Interface {
		// Boxing a value into an interface holds it unchanged.
		return true
	}
	if from.Kind() == reflect.String || t.Kind() == reflect.String {
		other := from
		if from.Kind() == reflect.String {
			other = t
		}
		return other.Kind() == reflect.String ||
			(other.Kind() == reflect.Slice && other.Elem().Kind() == reflect.Uint8)
	}
	return true
}

// convertValue converts src to type t, rendering integer sources as decimal
// text when the target is a string — reflect's own conversion is a rune
// mapping, so int32(42) would become "*" and int64(2026) would become "ߪ".
// Formatting the digits keeps the load -> save round trip lossless: a skipped
// field would save its zero value back over the stored number.
func convertValue(src reflect.Value, t reflect.Type) (reflect.Value, bool) {
	if !src.IsValid() {
		return reflect.Value{}, false
	}
	if t.Kind() == reflect.String {
		switch {
		case src.CanInt():
			return reflect.ValueOf(strconv.FormatInt(src.Int(), 10)).Convert(t), true
		case src.CanUint():
			return reflect.ValueOf(strconv.FormatUint(src.Uint(), 10)).Convert(t), true
		}
	}
	if !safeConvertible(src.Type(), t) {
		return reflect.Value{}, false
	}
	return src.Convert(t), true
}

// convertFieldValue converts src into a value of t. Slices are converted
// element-wise, keeping the zero value for elements that cannot convert and
// allocating a pointer for elements that convert to a pointer element's base
// type, and a single value is wrapped in a one-element slice when the target
// is a slice. A multi-valued property arriving at a single-valued field keeps
// the first value, matching how Gremlin itself narrows a property reference to
// one value.
//
// It returns the converted value, the number of values discarded to fit a
// single-valued field, and whether src fits in t at all.
func convertFieldValue(t reflect.Type, src reflect.Value) (reflect.Value, int, bool) {
	srcType := src.Type()
	srcIsSlice := srcType.Kind() == reflect.Slice
	targetIsSlice := t.Kind() == reflect.Slice

	if srcIsSlice && t.Kind() == reflect.Array && src.Len() != t.Len() {
		// Slice-to-array conversion panics on a length mismatch; leave the
		// field untouched instead.
		return reflect.Value{}, 0, false
	}
	if converted, ok := convertValue(src, t); ok {
		return converted, 0, true
	}
	switch {
	case srcIsSlice && targetIsSlice:
		return convertSliceElements(t, src)
	case targetIsSlice:
		// Handle case where field is a slice but gremlin result is a single value
		return wrapSingleValueIntoSlice(t, src)
	case srcIsSlice && src.Len() > 0:
		return narrowToFirstValue(t, src)
	}
	return reflect.Value{}, 0, false
}

// convertSliceElements builds a []t-shaped slice from src converting each
// element, keeping the zero value for elements that cannot convert and
// allocating a pointer for elements that convert to a pointer element's base
// type.
func convertSliceElements(t reflect.Type, src reflect.Value) (reflect.Value, int, bool) {
	slice := reflect.MakeSlice(t, src.Len(), src.Len())
	elemType := t.Elem()
	elemIsPointer := elemType.Kind() == reflect.Pointer
	if elemIsPointer {
		elemType = elemType.Elem()
	}
	for i := range src.Len() {
		converted, ok := convertValue(concreteValue(src.Index(i)), elemType)
		if !ok {
			continue
		}
		if elemIsPointer {
			slice.Index(i).Set(allocatePointer(elemType, converted))
			continue
		}
		slice.Index(i).Set(converted)
	}
	return slice, 0, true
}

// wrapSingleValueIntoSlice puts a single value into a one-element slice field,
// allocating a pointer when the element type is one.
func wrapSingleValueIntoSlice(t reflect.Type, src reflect.Value) (reflect.Value, int, bool) {
	elemType := t.Elem()
	if elemType.Kind() == reflect.Pointer {
		converted, ok := convertValue(src, elemType.Elem())
		if !ok {
			return reflect.Value{}, 0, false
		}
		slice := reflect.MakeSlice(t, 1, 1)
		slice.Index(0).Set(allocatePointer(elemType.Elem(), converted))
		return slice, 0, true
	}
	converted, ok := convertValue(src, elemType)
	if !ok {
		return reflect.Value{}, 0, false
	}
	slice := reflect.MakeSlice(t, 1, 1)
	slice.Index(0).Set(converted)
	return slice, 0, true
}

// narrowToFirstValue fits a multi-valued property into a single-valued field.
// Every Neptune vertex property is set-cardinality, so a property that models a
// single value still arrives as a list once it holds more than one; keeping the
// first value loses less than dropping the property, which would load the field
// as its zero value.
func narrowToFirstValue(t reflect.Type, src reflect.Value) (reflect.Value, int, bool) {
	converted, ok := convertValue(concreteValue(src.Index(0)), t)
	if !ok {
		return reflect.Value{}, 0, false
	}
	return converted, src.Len() - 1, true
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
	// Narrowing inside a preloaded vertex is not reported, for the same reason
	// its errors are not: this path has no way to surface either to the caller.
	if _, err := unloadGremlinMapIntoStruct(structPointer.Interface(), mapValue); err != nil {
		return reflect.Value{}, false
	}
	if isPointer {
		return structPointer, true
	}
	return structPointer.Elem(), true
}

// getLabelFromValue resolves the label for a vertex or edge model value,
// preferring a custom Label() implementation over the snake-cased struct name.
func getLabelFromValue(value any) string {
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

		// omitempty is evaluated on the field itself: a pointer to a zero
		// value was set deliberately and is written; only nil pointers and
		// other zero values are skipped. Unlike encoding/json, struct fields
		// (e.g. a zero time.Time) are also omitted.
		if fieldSchema.omitEmpty && fieldValue.IsZero() {
			continue
		}

		// Check if field is a pointer: unset (nil) pointer fields are skipped,
		// otherwise dereference to get the actual value.
		if fieldValue.Kind() == reflect.Pointer {
			dereferenced := concreteValue(fieldValue)
			if !dereferenced.IsValid() {
				continue
			}
			fieldValue = dereferenced
		}

		if fieldSchema.isSerializer {
			serialized, err := serializeGremlinValue(fieldValue)
			if err != nil {
				return nil, fmt.Errorf("serialize property %q: %w", fieldSchema.tagName, err)
			}
			mapValue[fieldSchema.tagName] = serialized
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

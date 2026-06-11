package driver

import (
	"reflect"
	"sync"

	"github.com/gobeam/stringy"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

// schemaCache caches per-type schema information so reflection and tag
// parsing happen once per type instead of on every query, create/update,
// and result row.
var schemaCache sync.Map // reflect.Type -> *typeSchema

// fieldSchema describes how a single struct field maps to gremlin data.
// index is the field's index path from the root struct, flattened through
// anonymous struct embeds, suitable for reflect.Value.FieldByIndex.
type fieldSchema struct {
	index           []int
	goName          string
	tagName         string
	subTraversalTag string
	omitEmpty       bool
	isEdge          bool
}

// typeSchema holds everything the driver needs to know about a model type.
type typeSchema struct {
	// zeroLabel is the label resolved from a zero value of the type:
	// custom Label() if implemented (value or pointer receiver), falling
	// back to snakeName.
	zeroLabel string
	// snakeName is the snake-cased struct name fallback label.
	snakeName string
	// implementsUnmapped reports whether the type (or its pointer)
	// implements gsmtypes.UnmappedPropertiesType.
	implementsUnmapped bool
	// selectedFields is the ValueMap argument list ("true" followed by all
	// gremlin property names). It is nil when the type collects unmapped
	// properties or has no tagged fields, in which case full value maps are
	// fetched. Consumers must treat it as read-only.
	selectedFields []any
	// unloadFields drives unmarshaling of gremlin result maps into structs.
	unloadFields []fieldSchema
	// mapFields drives structToMap for create/update.
	mapFields []fieldSchema
}

// schemaFor returns the cached schema for rt, computing it on first use.
// Pointer types are normalized to their element type.
func schemaFor(rt reflect.Type) *typeSchema {
	if rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	if cached, ok := schemaCache.Load(rt); ok {
		return cached.(*typeSchema) //nolint:errcheck // cache only stores *typeSchema
	}
	schema, _ := schemaCache.LoadOrStore(rt, buildSchema(rt))
	return schema.(*typeSchema) //nolint:errcheck // cache only stores *typeSchema
}

// mapFieldByTag returns the persisted-field schema whose gremlin tag matches
// tagName, including fields from anonymous struct embeds.
func (s *typeSchema) mapFieldByTag(tagName string) (*fieldSchema, bool) {
	for i := range s.mapFields {
		if s.mapFields[i].tagName == tagName {
			return &s.mapFields[i], true
		}
	}
	return nil, false
}

func buildSchema(rt reflect.Type) *typeSchema {
	schema := &typeSchema{
		snakeName:          stringy.New(rt.Name()).SnakeCase().ToLower(),
		implementsUnmapped: typeImplementsUnmappedProperties(rt),
	}
	schema.zeroLabel = zeroValueLabel(rt, schema.snakeName)
	if rt.Kind() != reflect.Struct {
		return schema
	}
	if !schema.implementsUnmapped {
		if fields := collectGremlinTagFields(rt); len(fields) > 0 {
			schema.selectedFields = append([]any{true}, fields...)
		}
	}
	schema.unloadFields = collectUnloadFields(rt, nil)
	schema.mapFields = collectMapFields(rt, nil)
	return schema
}

// zeroValueLabel resolves the label for a zero value of rt, preferring a
// custom Label() implementation over the snake-cased struct name.
func zeroValueLabel(rt reflect.Type, snakeName string) string {
	// reflect.New covers both value and pointer receiver Label methods.
	if labeler, ok := reflect.New(rt).Interface().(gsmtypes.CustomLabelType); ok {
		if label := labeler.Label(); label != "" {
			return label
		}
	}
	return snakeName
}

func collectGremlinTagFields(rt reflect.Type) []any { //nolint:gocognit
	if rt == nil {
		return nil
	}
	if rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return nil
	}

	fields := make([]any, 0)
	for i := range rt.NumField() {
		field := rt.Field(i)

		if field.Anonymous {
			anonymousType := field.Type
			if anonymousType.Kind() == reflect.Pointer {
				anonymousType = anonymousType.Elem()
			}
			if anonymousType.Kind() == reflect.Struct {
				fields = append(fields, collectGremlinTagFields(anonymousType)...)
				continue
			}
		}

		if field.PkgPath != "" {
			continue
		}

		if field.Tag.Get(gsmtypes.GremlinSubTraversalTag) != "" ||
			field.Tag.Get(gsmtypes.GremlinEdgeTag) != "" {
			continue
		}

		gremlinTag := field.Tag.Get(gsmtypes.GremlinTag)
		if gremlinTag == "" || gremlinTag == "-" {
			continue
		}

		tagParts := parseGremlinTag(gremlinTag)
		if tagParts.unmapped || tagParts.name == "" || tagParts.name == "-" {
			continue
		}

		fields = append(fields, tagParts.name)
	}

	return fields
}

// collectUnloadFields flattens the fields used when unmarshaling gremlin
// result maps, recursing through anonymous struct embeds.
func collectUnloadFields(rt reflect.Type, index []int) []fieldSchema {
	fields := make([]fieldSchema, 0, rt.NumField())
	for i := range rt.NumField() {
		field := rt.Field(i)
		fieldIndex := childIndex(index, i)

		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			fields = append(fields, collectUnloadFields(field.Type, fieldIndex)...)
		}

		if schema, ok := unloadFieldSchema(field, fieldIndex); ok {
			fields = append(fields, schema)
		}
	}
	return fields
}

func unloadFieldSchema(field reflect.StructField, index []int) (fieldSchema, bool) {
	// gremlinEdge fields are loaded via Preload and keyed by the Go field name.
	if field.Tag.Get(gsmtypes.GremlinEdgeTag) != "" {
		return fieldSchema{index: index, goName: field.Name, isEdge: true}, true
	}

	subTraversalTag := field.Tag.Get(gsmtypes.GremlinSubTraversalTag)
	tagName := field.Tag.Get(gsmtypes.GremlinTag)
	if tagName != "" {
		tagParts := parseGremlinTag(tagName)
		if tagParts.unmapped {
			return fieldSchema{}, false
		}
		tagName = tagParts.name
	}

	if (tagName == "" || tagName == "-") &&
		(subTraversalTag == "" || subTraversalTag == "-") {
		return fieldSchema{}, false
	}

	return fieldSchema{
		index:           index,
		goName:          field.Name,
		tagName:         tagName,
		subTraversalTag: subTraversalTag,
	}, true
}

// collectMapFields flattens the fields used by structToMap when persisting a
// struct, recursing through anonymous struct embeds.
func collectMapFields(rt reflect.Type, index []int) []fieldSchema {
	fields := make([]fieldSchema, 0, rt.NumField())
	for i := range rt.NumField() {
		field := rt.Field(i)
		fieldIndex := childIndex(index, i)

		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			fields = append(fields, collectMapFields(field.Type, fieldIndex)...)
			continue
		}

		// Skip sub traversal and edge tags so these dont get included when creating vertices
		if field.Tag.Get(gsmtypes.GremlinSubTraversalTag) != "" ||
			field.Tag.Get(gsmtypes.GremlinEdgeTag) != "" {
			continue
		}

		gremlinTag := field.Tag.Get(gsmtypes.GremlinTag)
		if gremlinTag == "" || gremlinTag == "-" || field.PkgPath != "" {
			continue
		}

		tagParts := parseGremlinTag(gremlinTag)
		if tagParts.unmapped {
			continue
		}

		fields = append(fields, fieldSchema{
			index:     fieldIndex,
			goName:    field.Name,
			tagName:   tagParts.name,
			omitEmpty: tagParts.omitEmpty,
		})
	}
	return fields
}

// childIndex returns a fresh index path for field i nested under parent.
func childIndex(parent []int, i int) []int {
	index := make([]int, len(parent)+1)
	copy(index, parent)
	index[len(parent)] = i
	return index
}

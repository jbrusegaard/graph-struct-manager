package driver

import (
	"fmt"
	"reflect"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

// Preload eagerly loads related GSM vertex structs over edges declared with
// the gremlinEdge struct tag. The tag declares the edge label and optionally
// the direction ("out" by default, or "in"/"both"):
//
//	type Topic struct {
//		gsmtypes.Vertex
//		Title string `gremlin:"title"`
//	}
//
//	type Person struct {
//		gsmtypes.Vertex
//		Name   string  `gremlin:"name"`
//		Topics []Topic `gremlinEdge:"subscribed,out"`
//	}
//
//	people, err := driver.Model[Person](db).Preload("Topics").Find()
//
// Preload takes Go struct field names. Supported field types are a struct,
// pointer to struct, or slice of (pointers to) structs, where the related
// struct is a GSM vertex struct. For non-slice fields the first related
// vertex is loaded. Invalid preloads surface as errors from Find/Take/ID.
func (q *Query[T]) Preload(fieldNames ...string) *Query[T] {
	modelType := reflect.TypeFor[T]()
	if modelType.Kind() == reflect.Pointer {
		modelType = modelType.Elem()
	}
	for _, fieldName := range fieldNames {
		traversal, err := buildPreloadTraversal(modelType, fieldName)
		if err != nil {
			q.err = err
			return q
		}
		q.writeDebugString(".Preload(")
		q.writeDebugString(fieldName)
		q.writeDebugString(")")
		q.subTraversals[fieldName] = traversal
	}
	return q
}

// buildPreloadTraversal builds the subtraversal that fetches the related
// vertices for a gremlinEdge tagged field as a folded list of value maps.
func buildPreloadTraversal(
	modelType reflect.Type,
	fieldName string,
) (*gremlingo.GraphTraversal, error) {
	if modelType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("preload: type %s is not a struct", modelType.Name())
	}
	field, ok := modelType.FieldByName(fieldName)
	if !ok {
		return nil, fmt.Errorf(
			"preload: field %s not found on struct %s",
			fieldName,
			modelType.Name(),
		)
	}
	edgeTag := field.Tag.Get(gsmtypes.GremlinEdgeTag)
	if edgeTag == "" {
		return nil, fmt.Errorf(
			"preload: field %s on struct %s is missing the %s tag",
			fieldName,
			modelType.Name(),
			gsmtypes.GremlinEdgeTag,
		)
	}
	tagOpts, err := parseGremlinEdgeTag(edgeTag)
	if err != nil {
		return nil, fmt.Errorf("preload: field %s: %w", fieldName, err)
	}
	relatedType, err := edgeFieldStructType(field.Type)
	if err != nil {
		return nil, fmt.Errorf("preload: field %s: %w", fieldName, err)
	}

	var traversal *gremlingo.GraphTraversal
	switch tagOpts.direction {
	case edgeDirectionIn:
		traversal = anonymousTraversal.In(tagOpts.label)
	case edgeDirectionBoth:
		traversal = anonymousTraversal.Both(tagOpts.label)
	case edgeDirectionOut:
		traversal = anonymousTraversal.Out(tagOpts.label)
	default:
		traversal = anonymousTraversal.Out(tagOpts.label)
	}

	relatedLabel := getLabelFromVertex(reflect.New(relatedType).Elem().Interface())
	if relatedLabel != "" {
		traversal = traversal.HasLabel(relatedLabel)
	}

	valueMapArgs := []any{true}
	if !typeImplementsUnmappedProperties(relatedType) {
		valueMapArgs = append(valueMapArgs, collectGremlinTagFields(relatedType)...)
	}

	traversal = traversal.ValueMap(valueMapArgs...).By(
		anonymousTraversal.Choose(
			anonymousTraversal.Count(Scope.Local).Is(P.Eq(1)),
			anonymousTraversal.Unfold(),
			anonymousTraversal.Identity(),
		),
	).Fold()
	return traversal, nil
}

// edgeFieldStructType resolves the underlying struct type of a gremlinEdge
// tagged field ([]T, []*T, *T, or T).
func edgeFieldStructType(fieldType reflect.Type) (reflect.Type, error) {
	if fieldType.Kind() == reflect.Slice {
		fieldType = fieldType.Elem()
	}
	if fieldType.Kind() == reflect.Pointer {
		fieldType = fieldType.Elem()
	}
	if fieldType.Kind() != reflect.Struct {
		return nil, fmt.Errorf(
			"gremlinEdge field must be a struct, pointer to struct, or slice of structs, got %s",
			fieldType.Kind(),
		)
	}
	return fieldType, nil
}

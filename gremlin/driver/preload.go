package driver

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

// preloadNode is a node in the tree of preload paths. Each key in children is
// a Go struct field name on the related type of the parent node.
type preloadNode struct {
	children map[string]*preloadNode
}

func newPreloadNode() *preloadNode {
	return &preloadNode{children: make(map[string]*preloadNode)}
}

// Preload eagerly loads related GSM vertex structs over edges declared with
// the gremlinEdge struct tag. The tag declares the edge label and optionally
// the direction ("out" by default, or "in"/"both"):
//
//	type Topic struct {
//		gsmtypes.Vertex
//		Title string `gremlin:"title"`
//		Posts []Post `gremlinEdge:"contains"`
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
// Preload takes Go struct field names. Nested relationships are loaded with
// dot separated paths; intermediate levels are preloaded implicitly:
//
//	people, err := driver.Model[Person](db).Preload("Topics.Posts").Find()
//
// Supported field types are a struct, pointer to struct, or slice of
// (pointers to) structs, where the related struct is a GSM vertex struct.
// For non-slice fields the first related vertex is loaded. Invalid preloads
// surface as errors from Find/Take/ID.
func (q *Query[T]) Preload(fieldPaths ...string) *Query[T] {
	modelType := reflect.TypeFor[T]()
	if modelType.Kind() == reflect.Pointer {
		modelType = modelType.Elem()
	}
	for _, fieldPath := range fieldPaths {
		rootField, err := q.mergePreloadPath(fieldPath)
		if err != nil {
			q.err = err
			return q
		}
		traversal, err := buildPreloadTraversal(modelType, rootField, q.preloads[rootField])
		if err != nil {
			q.err = err
			return q
		}
		q.writeDebugString(".Preload(")
		q.writeDebugString(fieldPath)
		q.writeDebugString(")")
		q.subTraversals[rootField] = traversal
	}
	return q
}

// mergePreloadPath merges a dot separated preload path into the query's
// preload tree and returns the root field name.
func (q *Query[T]) mergePreloadPath(fieldPath string) (string, error) {
	parts := strings.Split(fieldPath, ".")
	if slices.Contains(parts, "") {
		return "", fmt.Errorf("preload: invalid path %q", fieldPath)
	}
	if q.preloads == nil {
		q.preloads = make(map[string]*preloadNode)
	}
	children := q.preloads
	for _, part := range parts {
		node, ok := children[part]
		if !ok {
			node = newPreloadNode()
			children[part] = node
		}
		children = node.children
	}
	return parts[0], nil
}

// buildPreloadTraversal builds the subtraversal that fetches the related
// vertices for a gremlinEdge tagged field as a folded list of value maps.
// When node has children, each related vertex's value map is merged with the
// nested preload projections so relationships load recursively.
func buildPreloadTraversal(
	modelType reflect.Type,
	fieldName string,
	node *preloadNode,
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

	if node == nil || len(node.children) == 0 {
		return traversal.ValueMap(valueMapArgs...).By(unfoldSingleValueTraversal()).Fold(), nil
	}

	childTraversals := make(map[string]*gremlingo.GraphTraversal, len(node.children))
	for childName, childNode := range node.children {
		childTraversal, childErr := buildPreloadTraversal(relatedType, childName, childNode)
		if childErr != nil {
			return nil, fmt.Errorf("preload: %s: %w", fieldName, childErr)
		}
		childTraversals[childName] = childTraversal
	}
	return traversal.Local(mergedValueMapTraversal(childTraversals, valueMapArgs...)).Fold(), nil
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

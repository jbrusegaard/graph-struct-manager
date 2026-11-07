package driver

import (
	"errors"
	"reflect"
	"time"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

func Create[T any](db *GremlinDriver, value *T) error {
	return createVertex(db, value)
}

// tracksLastModified reports whether the driver should automatically stamp
// the value's last-modified timestamp. Models opt out (or rename the tracked
// property) via gsmtypes.LastModifiedPropertyType.
func tracksLastModified(value any) bool {
	return schemaFor(reflect.TypeOf(value)).lastModifiedProperty != ""
}

func getSlicePropertyNames(propertyMap map[string]any) []any {
	var slicePropertyNames []any
	for k, v := range propertyMap {
		if reflect.ValueOf(v).Kind() == reflect.Slice {
			slicePropertyNames = append(slicePropertyNames, k)
		}
	}
	return slicePropertyNames
}

func updateVertex[T any](db *GremlinDriver, value *T) error {
	vertex, ok := any(value).(gsmtypes.VertexType)
	if !ok {
		return errors.New("value does not implement VertexType")
	}
	if tracksLastModified(value) {
		vertex.SetVertexLastModified(time.Now().UTC())
	}
	err := runBeforeUpdateHook(db, value)
	if err != nil {
		return err
	}
	mapValue, err := structToMap(value)
	if err != nil {
		return err
	}
	if mapValue["id"] == nil {
		return errors.New(
			"invalid update operation value does not contain id field or id was not set",
		)
	}
	id := mapValue["id"]
	delete(mapValue, "id")
	label := getLabelFromValue(value)
	query := db.g.V(id).HasLabel(label)
	if slicePropertyNames := getSlicePropertyNames(mapValue); len(slicePropertyNames) > 0 {
		// Drop existing multi-valued properties in the same traversal so
		// stale elements don't survive the update. This only targets slice
		// properties: Properties() with no arguments would match (and drop)
		// every property on the vertex.
		query = query.SideEffect(anonymousTraversal.Properties(slicePropertyNames...).Drop())
	}
	query = handlePropertyUpdate(db, mapValue, query)
	db.logTraversal(query)
	_, err = query.Next()
	if err != nil {
		return err
	}
	return runAfterUpdateHook(db, value)
}

// resolveCreateID chooses the ID for a new vertex or edge and removes the
// id key from the property map so it is never written as a regular property.
// An ID set on the struct wins; otherwise the configured IDGenerator
// supplies one. hasID is false when neither provides an ID, letting the
// backend generate it.
func resolveCreateID(db *GremlinDriver, mapValue map[string]any) (any, bool) {
	id := mapValue["id"]
	delete(mapValue, "id")
	if id != nil {
		return id, true
	}
	if db.idGenerator != nil {
		if generated := db.idGenerator(); generated != nil {
			return generated, true
		}
	}
	return nil, false
}

func createVertex[T any](db *GremlinDriver, value *T) error {
	vertex, ok := any(value).(gsmtypes.VertexType)
	if !ok {
		return errors.New("value does not implement VertexType")
	}
	now := time.Now().UTC()
	vertex.SetVertexCreatedAt(now)
	if tracksLastModified(value) {
		vertex.SetVertexLastModified(now)
	}
	err := runBeforeCreateHook(db, value)
	if err != nil {
		return err
	}
	mapValue, err := structToMap(value)
	if err != nil {
		return err
	}
	id, hasID := resolveCreateID(db, mapValue)

	label := getLabelFromValue(value)
	query := db.g.AddV(label)
	query = handlePropertyUpdate(db, mapValue, query)
	if hasID {
		query = query.Property(gremlingo.T.Id, id)
	}
	query = query.Id()
	db.logTraversal(query)
	vertexID, err := query.Next()
	if err != nil {
		return err
	}
	vertex.SetVertexID(vertexID.GetInterface())
	return runAfterCreateHook(db, value)
}

func handlePropertyUpdate(
	db *GremlinDriver, properties map[string]any, query *gremlingo.GraphTraversal,
) *gremlingo.GraphTraversal {
	for k, v := range properties {
		// check if v is a slice and Neptune is being used
		if db.dbDriver == Neptune &&
			reflect.ValueOf(v).Kind() == reflect.Slice {
			for i := range reflect.ValueOf(v).Len() {
				query = query.Property(
					gremlingo.Cardinality.Set,
					k,
					reflect.ValueOf(v).Index(i).Interface(),
				)
			}
		} else {
			query = query.Property(gremlingo.Cardinality.Single, k, v)
		}
	}
	return query
}

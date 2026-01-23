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

func Update[T any](db *GremlinDriver, value *T) error {
	return updateVertex(db, value)
}

func updateVertex[T any](db *GremlinDriver, value *T) error {
	err := runBeforeUpdateHook(db, value)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
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
	mapValue[gsmtypes.LastModified] = now
	label := GetLabel[T]()
	query := db.g.V(id).HasLabel(label)
	query = handlePropertyUpdate(db, mapValue, query)
	_, err = query.Next()
	if err != nil {
		return err
	}
	vertex, ok := any(value).(gsmtypes.VertexType)
	if !ok {
		return errors.New("value does not implement VertexType")
	}
	vertex.SetVertexLastModified(now)
	return runAfterUpdateHook(db, value)
}

func createVertex[T any](db *GremlinDriver, value *T) error {
	err := runBeforeCreateHook(db, value)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	mapValue, err := structToMap(value)
	if err != nil {
		return err
	}
	delete(mapValue, "id")
	var hasID bool
	var id any
	if db.idGenerator != nil {
		id = db.idGenerator()
		if id != nil {
			hasID = true
		}
	}

	label := GetLabel[T]()
	mapValue[gsmtypes.LastModified] = now
	mapValue[gsmtypes.CreatedAt] = now
	query := db.g.AddV(label)
	query = handlePropertyUpdate(db, mapValue, query)
	if hasID {
		query = query.Property(gremlingo.T.Id, id)
	}
	vertexID, err := query.Id().Next()
	if err != nil {
		return err
	}

	vertex, ok := any(value).(gsmtypes.VertexType)
	if !ok {
		return errors.New("value does not implement VertexType")
	}
	vertex.SetVertexID(vertexID.GetInterface())
	vertex.SetVertexCreatedAt(now)
	vertex.SetVertexLastModified(now)
	return runAfterCreateHook(db, value)
}

func handlePropertyUpdate(
	db *GremlinDriver, properties map[string]any, query *gremlingo.GraphTraversal,
) *gremlingo.GraphTraversal {
	for k, v := range properties {
		// check if v is a slice and Neptune is being used
		if db.dbDriver == Neptune &&
			(reflect.ValueOf(v).Kind() == reflect.Slice || reflect.ValueOf(v).Kind() == reflect.Map) {
			if reflect.ValueOf(v).Kind() == reflect.Slice {
				for i := range reflect.ValueOf(v).Len() {
					query = query.Property(
						gremlingo.Cardinality.Set,
						k,
						reflect.ValueOf(v).Index(i).Interface(),
					)
				}
			} else {
				for _, key := range reflect.ValueOf(v).MapKeys() {
					query = query.Property(
						gremlingo.Cardinality.Set,
						k,
						reflect.ValueOf(v).MapIndex(key).Interface(),
					)
				}
			}
		} else {
			query = query.Property(gremlingo.Cardinality.Single, k, v)
		}
	}
	return query
}

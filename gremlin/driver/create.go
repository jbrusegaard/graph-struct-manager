package driver

import (
	"errors"
	"reflect"
	"time"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

func Create[T gsmtypes.VertexType](db *GremlinDriver, value *T) error {
	return createVertex(db, value)
}

func Update[T gsmtypes.VertexType](db *GremlinDriver, value *T) error {
	return updateVertex(db, value)
}

func updateVertex[T gsmtypes.VertexType](db *GremlinDriver, value *T) error {
	err := validateStructPointerWithAnonymousVertex(value)
	if err != nil {
		db.logger.Errorf("Validation failed: %v", err)
		return err
	}
	now := time.Now().UTC()
	_, mapValue, err := structToMap(value)
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
	query := db.g.V(id)
	query = handlePropertyUpdate(db, mapValue, query)
	_, err = query.Next()
	if err != nil {
		return err
	}
	reflectNow := reflect.ValueOf(now)
	reflect.ValueOf(value).Elem().FieldByName("LastModified").Set(reflectNow)
	return nil
}

func createVertex[T gsmtypes.VertexType](db *GremlinDriver, value *T) error {
	err := validateStructPointerWithAnonymousVertex(value)
	if err != nil {
		db.logger.Errorf("Validation failed: %v", err)
		return err
	}
	now := time.Now().UTC()
	label, mapValue, err := structToMap(value)
	if err != nil {
		return err
	}
	id, hasID := mapValue["id"]
	delete(mapValue, "id")
	mapValue[gsmtypes.LastModified] = now
	mapValue[gsmtypes.CreatedAt] = now
	query := db.g.AddV(label)
	query = handlePropertyUpdate(db, mapValue, query)
	if hasID && id != nil {
		query = query.Property(gremlingo.T.Id, id)
	}
	vertexID, err := query.Id().Next()
	if err != nil {
		return err
	}
	// Check if ID was pre-set or if generated add it to struct
	if !hasID {
		reflect.ValueOf(value).
			Elem().
			FieldByName("ID").
			Set(reflect.ValueOf(vertexID.GetInterface()))
	}
	reflectNow := reflect.ValueOf(now)
	reflect.ValueOf(value).Elem().FieldByName("LastModified").Set(reflectNow)
	reflect.ValueOf(value).Elem().FieldByName("CreatedAt").Set(reflectNow)
	return nil
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

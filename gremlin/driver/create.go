package driver

import (
	"reflect"
	"time"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

func Create[T gsmtypes.VertexType](db *GremlinDriver, value *T) error {
	return createVertex(db, value)
}

func Update[T gsmtypes.VertexType](db *GremlinDriver, value *T) error {
	return createOrUpdate(db, value)
}

func createOrUpdate[T gsmtypes.VertexType](db *GremlinDriver, value *T) error {
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
	id := mapValue["id"]
	delete(mapValue, "id")
	mapValue[gsmtypes.LastModified] = now
	var query *gremlingo.GraphTraversal
	newMap := make(map[any]any, len(mapValue))
	if id == nil {
		mapValue[gsmtypes.CreatedAt] = now
		for k, v := range mapValue {
			newMap[k] = v
		}
		newMap[gremlingo.T.Label] = label
		query = db.g.MergeV(newMap)
	} else {
		query = db.g.MergeV(map[any]any{gremlingo.T.Id: id})
	}
	query.Option(gremlingo.Merge.OnMatch, mapValue)
	vertexID, err := query.Id().Next()
	if err != nil {
		return err
	}
	reflect.ValueOf(value).Elem().FieldByName("ID").Set(reflect.ValueOf(vertexID.GetInterface()))
	reflectNow := reflect.ValueOf(now)
	reflect.ValueOf(value).Elem().FieldByName("LastModified").Set(reflectNow)
	reflect.ValueOf(value).Elem().FieldByName("CreatedAt").Set(reflectNow)
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
	delete(mapValue, "id")
	mapValue[gsmtypes.LastModified] = now
	mapValue[gsmtypes.CreatedAt] = now
	query := db.g.AddV(label)
	for k, v := range mapValue {
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
					query = query.Property(gremlingo.Cardinality.Set, k, reflect.ValueOf(v).MapIndex(key).Interface())
				}
			}
		} else {
			query = query.Property(gremlingo.Cardinality.Single, k, v)
		}
	}
	vertexID, err := query.Id().Next()
	if err != nil {
		return err
	}
	reflect.ValueOf(value).Elem().FieldByName("ID").Set(reflect.ValueOf(vertexID.GetInterface()))
	reflectNow := reflect.ValueOf(now)
	reflect.ValueOf(value).Elem().FieldByName("LastModified").Set(reflectNow)
	reflect.ValueOf(value).Elem().FieldByName("CreatedAt").Set(reflectNow)
	return nil
}

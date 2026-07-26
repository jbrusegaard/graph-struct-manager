package driver

import (
	"errors"
	"fmt"
	"time"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

// CreateEdge creates an edge of type E between the from and to vertices.
// The edge struct must embed gsmtypes.Edge anonymously. from and to may be
// GSM vertex structs (their IDs are used) or raw vertex IDs.
//
// The edge's created_at and last_modified properties are stamped
// automatically (models can rename or disable last-modified tracking via
// gsmtypes.LastModifiedPropertyType), BeforeCreate/AfterCreate hooks run,
// and the generated edge ID is written back to the struct.
//
//	person := ...            // existing Person vertex
//	topic := ...             // existing Topic vertex
//	sub := Subscribed{Since: 2024}
//	err := driver.CreateEdge(db, &sub, &person, &topic)
func CreateEdge[E any](db *GremlinDriver, edge *E, from any, to any) error {
	return createEdge(db, edge, from, to)
}

// SaveEdge creates the edge when its ID is unset, otherwise it updates the
// existing edge's properties. Gremlin cannot re-point an existing edge, so
// from and to are only used on create; updates match the edge by ID and
// label and leave its endpoints untouched.
func SaveEdge[E any](db *GremlinDriver, edge *E, from any, to any) error {
	edgeValue, ok := any(edge).(gsmtypes.EdgeType)
	if !ok {
		return errors.New("edge does not implement EdgeType")
	}
	if edgeValue.GetEdgeID() == nil {
		return createEdge(db, edge, from, to)
	}
	return updateEdge(db, edge)
}

func createEdge[E any](db *GremlinDriver, value *E, from any, to any) error {
	edge, ok := any(value).(gsmtypes.EdgeType)
	if !ok {
		return errors.New("value does not implement EdgeType")
	}
	fromID, err := resolveEndpointID(from)
	if err != nil {
		return fmt.Errorf("create edge from vertex: %w", err)
	}
	toID, err := resolveEndpointID(to)
	if err != nil {
		return fmt.Errorf("create edge to vertex: %w", err)
	}
	now := time.Now().UTC()
	edge.SetEdgeCreatedAt(now)
	if tracksLastModified(value) {
		edge.SetEdgeLastModified(now)
	}
	if hookErr := runBeforeCreateHook(db, value); hookErr != nil {
		return hookErr
	}
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

	label := getLabelFromValue(value)
	query := db.g.V(fromID).AddE(label).To(anonymousTraversal.V(toID))
	query = applyEdgeProperties(query, mapValue)
	if hasID {
		query = query.Property(gremlingo.T.Id, id)
	}
	edgeID, err := query.Id().Next()
	if err != nil {
		if isGremlinNotFoundErr(err) {
			return fmt.Errorf("create edge: from vertex %v not found", fromID)
		}
		return err
	}
	edge.SetEdgeID(edgeID.GetInterface())
	return runAfterCreateHook(db, value)
}

func updateEdge[E any](db *GremlinDriver, value *E) error {
	edge, ok := any(value).(gsmtypes.EdgeType)
	if !ok {
		return errors.New("value does not implement EdgeType")
	}
	if tracksLastModified(value) {
		edge.SetEdgeLastModified(time.Now().UTC())
	}
	if err := runBeforeUpdateHook(db, value); err != nil {
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
	query := db.g.E(id).HasLabel(label)
	query = applyEdgeProperties(query, mapValue)
	if _, err = query.Next(); err != nil {
		return err
	}
	return runAfterUpdateHook(db, value)
}

// resolveEndpointID resolves a CreateEdge/SaveEdge endpoint into a vertex ID.
// GSM vertex structs contribute their ID; any other non-nil value is treated
// as a raw vertex ID.
func resolveEndpointID(endpoint any) (any, error) {
	if endpoint == nil {
		return nil, errors.New("endpoint is nil")
	}
	if vertex, ok := endpoint.(gsmtypes.VertexType); ok {
		id := vertex.GetVertexID()
		if id == nil {
			return nil, errors.New("vertex has no id (was it created?)")
		}
		return id, nil
	}
	return endpoint, nil
}

// applyEdgeProperties appends Property steps for edge properties. Edge
// properties are single-valued in Gremlin, so no cardinality is passed
// (TinkerPop rejects cardinality arguments on edge properties). Slice values
// are written as a single list-valued property; support for list values on
// edges is backend-dependent.
func applyEdgeProperties(
	query *gremlingo.GraphTraversal, properties map[string]any,
) *gremlingo.GraphTraversal {
	for k, v := range properties {
		query = query.Property(k, v)
	}
	return query
}

package driver

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/comparator"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

var errGremlinNotFound = errors.New("E0903: there are no results left")

func isGremlinNotFoundErr(err error) bool {
	return err.Error() == errGremlinNotFound.Error()
}

type RangeCondition struct {
	lower int
	upper int
}

var cardinality = gremlingo.Cardinality

// Query represents a chainable query builder
type Query[T any] struct {
	conditions     []*QueryCondition
	db             *GremlinDriver
	debug          bool
	debugString    *strings.Builder
	dedup          bool
	err            error
	fromVertexID   any
	ids            []any
	isEdgeQuery    bool
	labels         []any
	toVertexID     any
	limit          *int
	offset         *int
	orderBy        *OrderCondition
	preloads       map[string]*preloadNode
	preTraversal   *gremlingo.GraphTraversal
	rangeCondition *RangeCondition
	selectedFields []any
	subTraversals  map[string]*gremlingo.GraphTraversal
}

type QueryCondition struct {
	field     string
	operator  comparator.Comparator
	value     any
	traversal *gremlingo.GraphTraversal
}

func (qc *QueryCondition) String() string {
	if qc.traversal != nil {
		return ".Where(User Passed Traversal)"
	}

	if qc.field == "id" {
		return fmt.Sprintf(".HasId(%v)", qc.value)
	}
	var sb strings.Builder
	sb.WriteString(".Has(")
	sb.WriteString(qc.field)
	sb.WriteString(", ")

	switch qc.operator {
	case comparator.EQ, "eq":
		sb.WriteString("P.Eq(")
	case comparator.NEQ, "neq":
		sb.WriteString("P.Neq(")
	case comparator.GT, "gt":
		sb.WriteString("P.Gt(")
	case comparator.GTE, "gte":
		sb.WriteString("P.Gte(")
	case comparator.LT, "lt":
		sb.WriteString("P.Lt(")
	case comparator.LTE, "lte":
		sb.WriteString("P.Lte(")
	case comparator.IN:
		sb.WriteString("P.Within(")
	case comparator.CONTAINS:
		sb.WriteString("TextP.Containing(")
	case comparator.WITHOUT:
		sb.WriteString("P.Without(")
	}
	value := reflect.ValueOf(qc.value)
	// Check if qc.value is a slice
	if value.IsValid() && value.Kind() == reflect.Slice {
		// iterate over the slice and append the value to the sb
		for i := range value.Len() {
			fmt.Fprintf(&sb, "%v ", value.Index(i).Interface())
			if i != value.Len()-1 {
				sb.WriteString(", ")
			}
		}
	} else {
		fmt.Fprintf(&sb, "%v", qc.value)
	}
	sb.WriteString(")")
	return sb.String()
}

type OrderCondition struct {
	field string
	desc  bool
}

func GetLabel[T any]() string {
	var v T
	// Use getLabelFromValue to support both pointer and value receivers
	label := getLabelFromValue(v)
	return label
}

// NewQuery creates a new query builder for type T.
// Types embedding gsmtypes.Edge query edges (g.E()) instead of vertices.
func NewQuery[T any](db *GremlinDriver) *Query[T] {
	label := GetLabel[T]()
	schema := schemaFor(reflect.TypeFor[T]())
	queryAsString := strings.Builder{}
	if schema.isEdge {
		queryAsString.WriteString("E()")
	} else {
		queryAsString.WriteString("V()")
	}
	if label != "" {
		queryAsString.WriteString(".HasLabel(")
		queryAsString.WriteString(label)
		queryAsString.WriteString(")")
	}
	ids := make([]any, 0)
	labels := []any{label}
	return &Query[T]{
		conditions:     make([]*QueryCondition, 0),
		db:             db,
		debug:          os.Getenv("GSM_DEBUG") == "true",
		debugString:    &queryAsString,
		ids:            ids,
		isEdgeQuery:    schema.isEdge,
		labels:         labels,
		orderBy:        nil,
		selectedFields: schema.selectedFields,
		subTraversals:  make(map[string]*gremlingo.GraphTraversal),
	}
}

// AddSubTraversals adds multiple subtraversals to the query
// This is useful when you need to fetch related data or perform complex traversals that should populate specific fields in your struct.
// You will need to signal this in your struct tags with the gremlinSubTraversal tag.
func (q *Query[T]) AddSubTraversals(subTraversals map[string]*gremlingo.GraphTraversal) *Query[T] {
	maps.Copy(q.subTraversals, subTraversals)
	return q
}

// AddSubTraversal adds a single subtraversal to the query
// This is useful when you need to fetch related data or perform complex traversals that should populate a specific field in your struct.
// You will need to signal this in your struct tags with the gremlinSubTraversal tag.
func (q *Query[T]) AddSubTraversal(
	gremlinTag string,
	traversal *gremlingo.GraphTraversal,
) *Query[T] {
	q.subTraversals[gremlinTag] = traversal
	return q
}

// Where adds a condition to the query
func (q *Query[T]) Where(field string, operator comparator.Comparator, value any) *Query[T] {
	queryCondition := QueryCondition{
		field:    field,
		operator: operator,
		value:    value,
	}
	q.writeDebugString(queryCondition.String())

	q.conditions = append(
		q.conditions, &queryCondition,
	)
	return q
}

// WhereTraversal adds a custom Gremlin traversal condition
func (q *Query[T]) WhereTraversal(traversal *gremlingo.GraphTraversal) *Query[T] {
	queryCondition := QueryCondition{
		traversal: traversal,
	}
	q.writeDebugString(queryCondition.String())
	q.conditions = append(
		q.conditions, &queryCondition,
	)
	return q
}

// QueryScope is a reusable piece of query logic, inspired by GORM scopes.
// A scope receives the query, applies one or more chainable steps, and
// returns the (mutated) query so scopes can be composed and reused across
// queries.
type QueryScope[T any] func(*Query[T]) *Query[T]

// Scopes applies one or more reusable QueryScope functions to the query, in
// order. This lets you package commonly used logic (filters, ordering,
// pagination) and reuse it across queries, similar to GORM's Scopes.
//
// Example:
//
//	func ActiveUsers(q *driver.Query[User]) *driver.Query[User] {
//	    return q.Where("status", comparator.EQ, "active")
//	}
//
//	func OlderThan(age int) driver.QueryScope[User] {
//	    return func(q *driver.Query[User]) *driver.Query[User] {
//	        return q.Where("age", comparator.GT, age)
//	    }
//	}
//
//	users, err := driver.Model[User](db).
//	    Scopes(ActiveUsers, OlderThan(21)).
//	    Find()
//
// Nil scopes and scopes that return nil are ignored so a single bad scope
// can't drop the rest of the chain.
func (q *Query[T]) Scopes(scopes ...QueryScope[T]) *Query[T] {
	for _, scope := range scopes {
		if scope == nil {
			continue
		}
		if next := scope(q); next != nil {
			q = next
		}
	}
	return q
}

// Dedup removes duplicate results from the query
func (q *Query[T]) Dedup() *Query[T] {
	q.writeDebugString(".Dedup()")
	q.dedup = true
	return q
}

// PreQuery sets a traversal to run before applying query conditions.
// When set, it replaces the default V() start for the query.
func (q *Query[T]) PreQuery(traversal *gremlingo.GraphTraversal) *Query[T] {
	if traversal == nil {
		return q
	}
	if q.fromVertexID != nil || q.toVertexID != nil {
		q.err = errors.New("prequery: cannot be combined with From/To")
		return q
	}
	q.preTraversal = traversal
	q.resetDebugStringForPreQuery()
	return q
}

// IDs adds the ids to the query
// You can use this to speed up the query by using the graph index
func (q *Query[T]) IDs(id ...any) *Query[T] {
	if q.debug {
		switch {
		case q.preTraversal != nil:
			q.writeDebugString(".HasId(")
		case q.isEdgeQuery:
			q.writeDebugString(".E(")
		default:
			q.writeDebugString(".V(")
		}
		for _, id := range id {
			q.writeDebugString(fmt.Sprintf("%v, ", id))
		}
		q.writeDebugString(")")
	}
	q.ids = append(q.ids, id...)
	return q
}

// From constrains an edge query to edges leaving the given vertex. The
// vertex may be a GSM vertex struct (its ID is used) or a raw vertex ID.
// The traversal starts at the vertex (g.V(id).OutE()) instead of scanning
// all edges, so this is also the fast way to query a vertex's edges.
// Only supported on edge queries and cannot be combined with PreQuery.
//
//	subs, err := driver.Edge[SubscribesTo](db).From(&person).Find()
func (q *Query[T]) From(vertex any) *Query[T] {
	return q.setEndpoint(vertex, &q.fromVertexID, "From")
}

// To constrains an edge query to edges arriving at the given vertex. The
// vertex may be a GSM vertex struct (its ID is used) or a raw vertex ID.
// The traversal starts at the vertex (g.V(id).InE()) instead of scanning
// all edges, so this is also the fast way to query a vertex's edges.
// Combine with From to match edges between a specific pair of vertices.
// Only supported on edge queries and cannot be combined with PreQuery.
//
//	subs, err := driver.Edge[SubscribesTo](db).From(&person).To(&topic).Find()
func (q *Query[T]) To(vertex any) *Query[T] {
	return q.setEndpoint(vertex, &q.toVertexID, "To")
}

// setEndpoint validates and stores a From/To endpoint constraint.
func (q *Query[T]) setEndpoint(vertex any, target *any, step string) *Query[T] {
	if !q.isEdgeQuery {
		q.err = fmt.Errorf("%s: only supported on edge queries", strings.ToLower(step))
		return q
	}
	if q.preTraversal != nil {
		q.err = fmt.Errorf("%s: cannot be combined with PreQuery", strings.ToLower(step))
		return q
	}
	id, err := resolveEndpointID(vertex)
	if err != nil {
		q.err = fmt.Errorf("%s vertex: %w", strings.ToLower(step), err)
		return q
	}
	q.writeDebugString(fmt.Sprintf(".%s(%v)", step, id))
	*target = id
	return q
}

// Limit sets the maximum number of results
func (q *Query[T]) Limit(limit int) *Query[T] {
	q.writeDebugString(".Limit(")
	q.writeDebugString(strconv.Itoa(limit))
	q.writeDebugString(")")
	q.limit = &limit
	return q
}

// Offset sets the number of results to skip
func (q *Query[T]) Offset(offset int) *Query[T] {
	q.writeDebugString(".Skip(")
	q.writeDebugString(strconv.Itoa(offset))
	q.writeDebugString(")")
	q.offset = &offset
	return q
}

// Labels adds labels to the query
// This is useful when you need to filter by multiple labels
// You can use this to speed up the query by using the graph index
// Note this will override any label set via the Label() method or pre computed label
func (q *Query[T]) Labels(labels ...string) *Query[T] {
	q.labels = SliceToAnySlice(labels)
	return q
}

// Range sets the range of the query
// This is useful when you need to get a range of results
// It will be ignored if offset is set
// Note the range is inclusive of lower bound and exclusive of upper bound
// Examples:
//   - Range(0, 10) will return results 0-9
//   - Range(10, 20) will return results 10-19
//   - Range(0, 20) will return results 0-19
func (q *Query[T]) Range(lower int, upper int) *Query[T] {
	if q.offset != nil {
		q.db.logger.Warnf(
			"Range should not be used with offset! It will be ignored.",
		)
		return q
	}
	q.writeDebugString(".Range(")
	q.writeDebugString(strconv.Itoa(lower))
	q.writeDebugString(", ")
	q.writeDebugString(strconv.Itoa(upper))
	q.writeDebugString(")")
	q.rangeCondition = &RangeCondition{lower: lower, upper: upper}
	return q
}

// Select adds selected fields to the query
func (q *Query[T]) Select(fields ...string) *Query[T] {
	if len(q.selectedFields) == 0 {
		q.db.logger.Warnf(
			"Select was already defined secondary select will override original select!",
		)
	}
	q.selectedFields = []any{true}
	q.writeDebugString(".GSMFieldsSelect(")
	q.writeDebugString(strings.Join(fields, ", "))
	q.writeDebugString(")")
	for _, field := range fields {
		q.selectedFields = append(q.selectedFields, field)
	}
	return q
}

// OrderBy adds ordering to the query
func (q *Query[T]) OrderBy(field string, order GremlinOrder) *Query[T] {
	if q.orderBy != nil {
		q.db.logger.Warnf(
			"Order by was already defined secondary order by will override original order",
		)
	}
	q.writeDebugString(".OrderBy(")
	q.writeDebugString(field)
	q.writeDebugString(", ")
	if order == Desc {
		q.writeDebugString("Order.Desc")
	} else {
		q.writeDebugString("Order.Asc")
	}
	q.writeDebugString(")")
	desc := order != 0
	q.orderBy = &OrderCondition{field: field, desc: desc}
	return q
}

// Find executes the query and returns all matching results
func (q *Query[T]) Find() ([]T, error) {
	if q.err != nil {
		return nil, q.err
	}
	q.writeDebugString(".ToList()")
	query := q.buildBaseQuery()
	if len(q.selectedFields) > 0 {
		query = ToMapTraversal(query, q.subTraversals, q.selectedFields...)
	} else {
		query = ToMapTraversal(query, q.subTraversals, true)
	}
	query = q.doOrderSkipRange(query)
	queryResults, err := query.ToList()
	if err != nil {
		return nil, err
	}

	results := make([]T, 0, len(queryResults))
	for _, result := range queryResults {
		var v T
		err = UnloadGremlinResultIntoStruct(&v, result)
		if err != nil {
			return nil, err
		}
		if findHookErr := runAfterFindHook(q.db, &v); findHookErr != nil {
			return nil, findHookErr
		}
		results = append(results, v)
	}
	return results, nil
}

// Take executes the query and returns the first result
func (q *Query[T]) Take() (T, error) {
	var v T
	if q.err != nil {
		return v, q.err
	}
	q.writeDebugString(".Next()")
	query := q.buildBaseQuery()
	if len(q.selectedFields) > 0 {
		query = ToMapTraversal(query, q.subTraversals, q.selectedFields...)
	} else {
		query = ToMapTraversal(query, q.subTraversals, true)
	}
	query = q.doOrderSkipRange(query)
	result, err := query.Next()
	if err != nil {
		if isGremlinNotFoundErr(err) {
			return v, gsmtypes.ErrNotFound
		}
		return v, err
	}

	err = UnloadGremlinResultIntoStruct(&v, result)
	if err != nil {
		return v, err
	}

	if findHookErr := runAfterFindHook(q.db, &v); findHookErr != nil {
		return v, findHookErr
	}
	return v, nil
}

// Count returns the number of matching results
func (q *Query[T]) Count() (int, error) {
	if q.err != nil {
		return 0, q.err
	}
	q.writeDebugString(".Count()")
	query := q.BuildQuery().Count()
	result, defaultVal, err := nextWithDefaultValue(query, 0)
	if err != nil {
		return 0, err
	}
	if result == nil {
		return defaultVal, nil
	}
	num, err := result.GetInt()
	if err != nil {
		return 0, err
	}
	return num, nil
}

// Delete deletes all matching results
func (q *Query[T]) Delete() error {
	if q.err != nil {
		return q.err
	}
	q.writeDebugString(".Drop().Iterate()")
	query := q.BuildQuery()
	err := query.Drop().Iterate()
	return <-err
}

// ID finds a vertex (or edge, for edge models) by id in a more optimized way
// than using where
func (q *Query[T]) ID(id any) (T, error) {
	var v T
	if q.err != nil {
		return v, q.err
	}
	query := q.startTraversal(id)
	if len(q.labels) > 0 {
		query = query.HasLabel(q.labels...)
	}
	result, err := ToMapTraversal(query, q.subTraversals, true).Next()
	if err != nil {
		if isGremlinNotFoundErr(err) {
			return v, gsmtypes.ErrNotFound
		}
		return v, err
	}
	err = UnloadGremlinResultIntoStruct(&v, result)
	if err != nil {
		return v, err
	}
	if findHookErr := runAfterFindHook(q.db, &v); findHookErr != nil {
		return v, findHookErr
	}
	return v, nil
}

// Update updates a property of the struct
// NOTE: Slices will be updated as Cardinality.Set
// NOTE: Maps will be updated as Cardinality.Set with keys as the value of the property
func (q *Query[T]) Update(propertyName string, value any) error {
	return q.Updates(map[string]any{propertyName: value})
}

// Updates performs a targeted update of multiple properties on all matching
// vertices in a single traversal. Only the supplied properties are written;
// every other property on the vertex is left untouched. Map keys must match
// the gremlin struct tags on T.
// NOTE: Slices will be updated as Cardinality.Set
// NOTE: the model's last-modified property (last_modified by default) is
// refreshed as part of the update; models can rename or disable this via
// gsmtypes.LastModifiedPropertyType
func (q *Query[T]) Updates(properties map[string]any) error {
	if q.err != nil {
		return q.err
	}
	if len(properties) == 0 {
		return nil
	}
	rt := reflect.TypeFor[T]()
	if rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	schema := schemaFor(rt)

	// Sort keys so the generated traversal and debug output are deterministic.
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	// Validate every property before touching the database so a bad key
	// can't leave a partial update behind.
	fieldTypes := make(map[string]reflect.Type, len(properties))
	for _, key := range keys {
		if key == "id" {
			return errors.New("cannot update vertex id")
		}
		field, ok := schema.mapFieldByTag(key)
		if !ok {
			return fmt.Errorf("propertyName not found in gremlin struct tags: %s", key)
		}
		fieldTypes[key] = rt.FieldByIndex(field.index).Type
	}

	query := q.BuildQuery()
	if lastModifiedProperty := schema.lastModifiedProperty; lastModifiedProperty != "" {
		query = q.stampLastModified(query, lastModifiedProperty)
	}
	for _, key := range keys {
		query = q.applyPropertyUpdate(query, key, fieldTypes[key], properties[key])
	}
	errChan := query.Iterate()
	return <-errChan
}

// RemoveProperty removes a single property from all matching vertices.
// NOTE: this differs from Update(propertyName, nil), which would write a
// null/zero value: RemoveProperty drops the property key entirely.
func (q *Query[T]) RemoveProperty(propertyName string) error {
	return q.RemoveProperties(propertyName)
}

// RemoveProperties removes one or more properties from all matching vertices
// in a single traversal. Every other property on the vertex is left
// untouched. Names must match the gremlin struct tags on T.
// NOTE: the model's last-modified property (last_modified by default) is
// refreshed as part of the removal; models can rename or disable this via
// gsmtypes.LastModifiedPropertyType
func (q *Query[T]) RemoveProperties(propertyNames ...string) error {
	if q.err != nil {
		return q.err
	}
	if len(propertyNames) == 0 {
		return nil
	}
	rt := reflect.TypeFor[T]()
	if rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	schema := schemaFor(rt)

	// De-duplicate and sort so the generated traversal and debug output are
	// deterministic regardless of caller-supplied ordering/duplicates.
	keySet := make(map[string]struct{}, len(propertyNames))
	for _, name := range propertyNames {
		keySet[name] = struct{}{}
	}
	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	// Validate every property before touching the database so a bad key
	// can't leave a partial removal behind.
	for _, key := range keys {
		if key == "id" {
			return errors.New("cannot remove vertex id")
		}
		if _, ok := schema.mapFieldByTag(key); !ok {
			return fmt.Errorf("propertyName not found in gremlin struct tags: %s", key)
		}
	}

	query := q.BuildQuery()
	if lastModifiedProperty := schema.lastModifiedProperty; lastModifiedProperty != "" {
		query = q.stampLastModified(query, lastModifiedProperty)
	}
	q.writeDebugString(".SideEffect(Properties(")
	q.writeDebugString(strings.Join(keys, ", "))
	q.writeDebugString(").Drop())")
	keyArgs := make([]any, len(keys))
	for i, key := range keys {
		keyArgs[i] = key
	}
	query = query.SideEffect(anonymousTraversal.Properties(keyArgs...).Drop())
	errChan := query.Iterate()
	return <-errChan
}

// stampLastModified refreshes the model's last-modified property as part of
// the traversal. Edge properties are single-valued and reject cardinality
// arguments, so edge queries write the property without one.
func (q *Query[T]) stampLastModified(
	query *gremlingo.GraphTraversal, lastModifiedProperty string,
) *gremlingo.GraphTraversal {
	if q.isEdgeQuery {
		q.writeDebugString(".Property(")
		q.writeDebugString(lastModifiedProperty)
		q.writeDebugString(", <now>)")
		return query.Property(lastModifiedProperty, time.Now().UTC())
	}
	q.writeDebugString(".Property(Cardinality.Single, ")
	q.writeDebugString(lastModifiedProperty)
	q.writeDebugString(", <now>)")
	return query.Property(cardinality.Single, lastModifiedProperty, time.Now().UTC())
}

// applyPropertyUpdate appends the Property steps for a single property to the
// traversal. Multi-valued (slice) properties are dropped first so stale
// elements don't survive the update.
// Edge properties are single-valued in Gremlin, so edge queries write every
// property (slices included) without a cardinality argument; slice values
// become a single list-valued property, which is backend-dependent.
func (q *Query[T]) applyPropertyUpdate(
	query *gremlingo.GraphTraversal,
	propertyName string,
	fieldType reflect.Type,
	value any,
) *gremlingo.GraphTraversal {
	if q.isEdgeQuery {
		q.writeDebugString(".Property(")
		q.writeDebugString(propertyName)
		q.writeDebugString(", ")
		q.writeDebugString(fmt.Sprintf("%v", value))
		q.writeDebugString(")")
		return query.Property(propertyName, value)
	}
	switch fieldType.Kind() { //nolint: exhaustive // We are only handling slices and maps otherwise regular cardinality
	case reflect.Slice:
		// Drop the existing property in the same traversal so stale slice
		// elements don't survive the update.
		q.writeDebugString(".SideEffect(Properties(")
		q.writeDebugString(propertyName)
		q.writeDebugString(").Drop())")
		query = query.SideEffect(anonymousTraversal.Properties(propertyName).Drop())
		cardinality := gremlingo.Cardinality.List
		cardinalityString := "Cardinality.List"
		if q.db.dbDriver == Neptune {
			cardinalityString = "Cardinality.Set"
			cardinality = gremlingo.Cardinality.Set
		}
		rv := reflect.ValueOf(value)
		sliceValue := make([]any, rv.Len())
		for i := range rv.Len() {
			sliceValue[i] = rv.Index(i).Interface()
		}
		for _, v := range sliceValue {
			q.writeDebugString(".Property(")
			q.writeDebugString(cardinalityString)
			q.writeDebugString(", ")
			q.writeDebugString(propertyName)
			q.writeDebugString(", ")
			q.writeDebugString(fmt.Sprintf("%v", v))
			q.writeDebugString(")")
			query = query.Property(cardinality, propertyName, v)
		}
	default:
		q.writeDebugString(".Property(Cardinality.Single, ")
		q.writeDebugString(propertyName)
		q.writeDebugString(", ")
		q.writeDebugString(fmt.Sprintf("%v", value))
		q.writeDebugString(")")
		query = query.Property(gremlingo.Cardinality.Single, propertyName, value)
	}
	return query
}

// writeDebugString writes a string to the debug string if GSM_DEBUG is set to true
func (q *Query[T]) writeDebugString(s string) {
	if q.debug {
		q.debugString.WriteString(s)
	}
}

// BuildQuery constructs the Gremlin traversal from the query conditions
func (q *Query[T]) BuildQuery() *gremlingo.GraphTraversal {
	query := q.buildBaseQuery()
	return q.doOrderSkipRange(query)
}

func (q *Query[T]) buildBaseQuery() *gremlingo.GraphTraversal {
	if q.debug {
		q.db.logger.Infof("Running Query: %s", q.debugString.String())
		q.debugString.Reset()
	}
	var query *gremlingo.GraphTraversal

	switch {
	case q.preTraversal != nil:
		query = q.preTraversal.Clone()
		if len(q.ids) > 0 {
			query = query.HasId(q.ids...)
		}
	case q.fromVertexID != nil || q.toVertexID != nil:
		query = q.startEndpointTraversal()
		if len(q.ids) > 0 {
			query = query.HasId(q.ids...)
		}
	case len(q.ids) > 0:
		query = q.startTraversal(q.ids...)
	default:
		query = q.startTraversal()
	}

	if len(q.labels) > 0 {
		query = query.HasLabel(q.labels...)
	}

	q.addQueryConditions(query)

	if q.dedup {
		query = query.Dedup()
	}
	return query
}

// startTraversal begins a traversal at the query's element source: g.E() for
// edge models, g.V() otherwise.
func (q *Query[T]) startTraversal(ids ...any) *gremlingo.GraphTraversal {
	if q.isEdgeQuery {
		return q.db.g.E(ids...)
	}
	return q.db.g.V(ids...)
}

// startEndpointTraversal begins an edge traversal at a From/To endpoint
// vertex so the query walks the vertex's adjacency list instead of scanning
// every edge. With both endpoints set, edges leaving the From vertex are
// filtered to those arriving at the To vertex.
func (q *Query[T]) startEndpointTraversal() *gremlingo.GraphTraversal {
	switch {
	case q.fromVertexID != nil && q.toVertexID != nil:
		return q.db.g.V(q.fromVertexID).
			OutE().
			Where(anonymousTraversal.InV().HasId(q.toVertexID))
	case q.fromVertexID != nil:
		return q.db.g.V(q.fromVertexID).OutE()
	default:
		return q.db.g.V(q.toVertexID).InE()
	}
}

func (q *Query[T]) doOrderSkipRange(query *gremlingo.GraphTraversal) *gremlingo.GraphTraversal {
	if q.orderBy != nil {
		if q.orderBy.desc {
			query.Order().By(q.orderBy.field, Order.Desc)
		} else {
			query.Order().By(q.orderBy.field, Order.Asc)
		}
	}

	// Apply offset
	if q.offset != nil {
		query = query.Skip(*q.offset)
	}

	// Apply limit
	if q.limit != nil {
		query = query.Limit(*q.limit)
	}

	// Apply range
	if q.rangeCondition != nil {
		query = query.Range(q.rangeCondition.lower, q.rangeCondition.upper)
	}
	return query
}

func (q *Query[T]) addQueryConditions(query *gremlingo.GraphTraversal) { //nolint:gocognit
	// Apply conditions
	for _, condition := range q.conditions {
		if condition.traversal != nil {
			query = query.Where(condition.traversal)
			continue
		}
		switch condition.operator {
		case comparator.EQ, "eq":
			if condition.field == "id" {
				query = query.HasId(condition.value)
			} else {
				query = query.Has(condition.field, condition.value)
			}
		case comparator.NEQ, "neq":
			query = query.Has(condition.field, gremlingo.P.Neq(condition.value))
		case comparator.GT, "gt":
			query = query.Has(condition.field, gremlingo.P.Gt(condition.value))
		case comparator.GTE, "gte":
			query = query.Has(condition.field, gremlingo.P.Gte(condition.value))
		case comparator.LT, "lt":
			query = query.Has(condition.field, gremlingo.P.Lt(condition.value))
		case comparator.LTE, "lte":
			query = query.Has(condition.field, gremlingo.P.Lte(condition.value))
		case comparator.CONTAINS:
			if strVal, ok := condition.value.(string); ok {
				query = query.Has(condition.field, gremlingo.TextP.Containing(strVal))
			}
		case comparator.IN, comparator.WITHOUT:
			var sliceValue []any
			value := reflect.ValueOf(condition.value)
			if value.IsValid() && value.Kind() == reflect.Slice {
				for i := range value.Len() {
					sliceValue = append(sliceValue, value.Index(i).Interface())
				}
			} else {
				sliceValue = append(sliceValue, condition.value)
			}
			if comparator.WITHOUT == condition.operator {
				query = query.Has(condition.field, gremlingo.P.Without(sliceValue))
			} else {
				query = query.Has(condition.field, gremlingo.P.Within(sliceValue))
			}
		}
	}
}

func (q *Query[T]) resetDebugStringForPreQuery() {
	if !q.debug {
		return
	}
	queryAsString := strings.Builder{}
	queryAsString.WriteString("PreQuery()")
	if len(q.labels) > 0 {
		labelStrings := make([]string, len(q.labels))
		for i, label := range q.labels {
			labelStrings[i], _ = label.(string)
		}
		queryAsString.WriteString(".HasLabel(")
		queryAsString.WriteString(strings.Join(labelStrings, ","))
		queryAsString.WriteString(")")
	}
	q.debugString = &queryAsString
}

// ToMapTraversal converts a Gremlin traversal to a map traversal using valuemap and projecting the subtraversals
// if there are no subtraversals, it will return the query.ValueMap(args...).By(
//
//		anonymousTraversal.Choose(
//			anonymousTraversal.Count(Scope.Local).Is(P.Eq(1)),
//			anonymousTraversal.Unfold(),
//			anonymousTraversal.Identity(),
//		),
//	)
func ToMapTraversal(
	query *gremlingo.GraphTraversal,
	subtraversals map[string]*gremlingo.GraphTraversal,
	args ...any,
) *gremlingo.GraphTraversal {
	if len(subtraversals) == 0 {
		return query.ValueMap(args...).By(unfoldSingleValueTraversal())
	}
	return query.Local(mergedValueMapTraversal(subtraversals, args...))
}

// unfoldSingleValueTraversal is the ValueMap by-modulator that unfolds
// single-cardinality property lists into plain values.
func unfoldSingleValueTraversal() *gremlingo.GraphTraversal {
	return anonymousTraversal.Choose(
		anonymousTraversal.Count(Scope.Local).Is(P.Eq(1)),
		anonymousTraversal.Unfold(),
		anonymousTraversal.Identity(),
	)
}

// mergedValueMapTraversal builds an anonymous traversal that merges a vertex's
// value map with projected subtraversal results into a single flat map keyed
// by property names and subtraversal aliases. It is used for the root query
// and recursively for nested preloads.
func mergedValueMapTraversal(
	subtraversals map[string]*gremlingo.GraphTraversal,
	args ...any,
) *gremlingo.GraphTraversal {
	subtraversalsKeys := make([]any, 0, len(subtraversals))
	for key := range subtraversals {
		subtraversalsKeys = append(subtraversalsKeys, key)
	}
	projectQuery := anonymousTraversal.Project(subtraversalsKeys...)
	for _, key := range subtraversalsKeys {
		keyString := key.(string) //nolint:errcheck //we already know this is a string
		projectQuery = projectQuery.By(subtraversals[keyString])
	}
	return anonymousTraversal.Union(
		anonymousTraversal.ValueMap(args...).By(unfoldSingleValueTraversal()),
		projectQuery,
	).Unfold().Group().By(gremlingo.Column.Keys).By(anonymousTraversal.Select(gremlingo.Column.Values))
}

package driver

import (
	"fmt"
	"maps"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/comparator"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

type RangeCondition struct {
	lower int
	upper int
}

var cardinality = gremlingo.Cardinality

// Query represents a chainable query builder
type Query[T any] struct {
	db             *GremlinDriver
	conditions     []*QueryCondition
	ids            []any
	label          string
	limit          *int
	offset         *int
	rangeCondition *RangeCondition
	preTraversal   *gremlingo.GraphTraversal
	subTraversals  map[string]*gremlingo.GraphTraversal
	orderBy        *OrderCondition
	dedup          bool
	debug          bool
	debugString    *strings.Builder
}

type QueryCondition struct {
	field     string
	operator  comparator.Comparator
	value     any
	traversal *gremlingo.GraphTraversal
}

func (qc *QueryCondition) String() string {
	if qc.traversal != nil {
		return ""
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

	sb.WriteString(fmt.Sprintf("%v))", qc.value))
	return sb.String()
}

type OrderCondition struct {
	field string
	desc  bool
}

func GetLabel[T any]() string {
	var v T
	// Use getLabelFromValue to support both pointer and value receivers
	label := getLabelFromVertex(v)
	return label
}

// NewQuery creates a new query builder for type T
func NewQuery[T any](db *GremlinDriver) *Query[T] {
	label := GetLabel[T]()
	queryAsString := strings.Builder{}
	queryAsString.WriteString("V()")
	if label != "" {
		queryAsString.WriteString(".HasLabel(")
		queryAsString.WriteString(label)
		queryAsString.WriteString(")")
	}
	ids := make([]any, 0)
	return &Query[T]{
		db:            db,
		debugString:   &queryAsString,
		ids:           ids,
		conditions:    make([]*QueryCondition, 0),
		label:         label,
		orderBy:       nil,
		subTraversals: make(map[string]*gremlingo.GraphTraversal),
		debug:         os.Getenv("GSM_DEBUG") == "true",
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
	q.preTraversal = traversal
	q.resetDebugStringForPreQuery()
	return q
}

// IDs adds the ids to the query
// You can use this to speed up the query by using the graph index
func (q *Query[T]) IDs(id ...any) *Query[T] {
	if q.debug {
		if q.preTraversal != nil {
			q.writeDebugString(".HasId(")
		} else {
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
		q.db.logger.Warn(
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

// OrderBy adds ordering to the query
func (q *Query[T]) OrderBy(field string, order GremlinOrder) *Query[T] {
	if q.orderBy != nil {
		q.db.logger.Warn(
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
	q.writeDebugString(".ToList()")
	query := q.buildBaseQuery()
	query = ToMapTraversal(query, q.subTraversals, true)
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
	q.writeDebugString(".Next()")
	var v T
	query := q.buildBaseQuery()
	query = ToMapTraversal(query, q.subTraversals, true)
	query = q.doOrderSkipRange(query)
	result, err := query.Next()
	if err != nil {
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
	q.writeDebugString(".Count()")
	query := q.BuildQuery()
	result, err := query.Count().Next()
	if err != nil {
		return 0, err
	}
	num, err := result.GetInt()
	if err != nil {
		return 0, err
	}
	return num, nil
}

// Delete deletes all matching results
func (q *Query[T]) Delete() error {
	q.writeDebugString(".Drop().Iterate()")
	query := q.BuildQuery()
	err := query.Drop().Iterate()
	return <-err
}

// ID finds vertex by id in a more optimized way than using where
func (q *Query[T]) ID(id any) (T, error) {
	var v T
	query := q.db.g.V(id)
	label := GetLabel[T]()
	query = query.HasLabel(label)
	result, err := ToMapTraversal(query, q.subTraversals, true).Next()
	if err != nil {
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
	// figure out if propertyName is in the struct
	_, fieldType, err := getStructFieldNameAndType[T](propertyName)
	if err != nil {
		return fmt.Errorf("propertyName not found in gremlin struct tags: %s", propertyName)
	}
	query := q.BuildQuery()
	query.Property(cardinality.Single, gsmtypes.LastModified, time.Now().UTC())
	switch fieldType.Kind() { //nolint: exhaustive // We are only handling slices and maps otherwise regular cardinality
	case reflect.Slice:
		cardinality := gremlingo.Cardinality.List
		cardinalityString := "Cardinality.List"
		if q.db.dbDriver == Neptune {
			cardinalityString = "Cardinality.Set"
			cardinality = gremlingo.Cardinality.Set
		}
		sliceValue, _ := value.([]any)
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
	case reflect.Map:
		mapValue, _ := value.(map[any]any)
		for k := range mapValue {
			q.writeDebugString(".Property(Cardinality.Set, ")
			q.writeDebugString(propertyName)
			q.writeDebugString(", ")
			q.writeDebugString(fmt.Sprintf("%v", k))
			q.writeDebugString(")")
			query = query.Property(gremlingo.Cardinality.Set, propertyName, k)
		}
	default:
		q.writeDebugString(".Property(Cardinality.Single, ")
		q.writeDebugString(propertyName)
		q.writeDebugString(", ")
		q.writeDebugString(fmt.Sprintf("%v", value))
		q.writeDebugString(")")
		query = query.Property(gremlingo.Cardinality.Single, propertyName, value)
	}
	errChan := query.Iterate()
	return <-errChan
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
	case len(q.ids) > 0:
		query = q.db.g.V(q.ids...)
	default:
		query = q.db.g.V()
	}

	if q.label != "" {
		query = query.HasLabel(q.label)
	}

	q.addQueryConditions(query)

	if q.dedup {
		query = query.Dedup()
	}
	return query
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

func (q *Query[T]) addQueryConditions(query *gremlingo.GraphTraversal) {
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
		case comparator.IN:
			if slice, ok := condition.value.([]any); ok {
				query = query.Has(condition.field, gremlingo.P.Within(slice...))
			}
		case comparator.CONTAINS:
			if strVal, ok := condition.value.(string); ok {
				query = query.Has(condition.field, gremlingo.TextP.Containing(strVal))
			}
		case comparator.WITHOUT:
			if slice, ok := condition.value.([]any); ok {
				query = query.Has(condition.field, gremlingo.P.Without(slice...))
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
	if q.label != "" {
		queryAsString.WriteString(".HasLabel(")
		queryAsString.WriteString(q.label)
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
		return query.ValueMap(args...).By(
			anonymousTraversal.Choose(
				anonymousTraversal.Count(Scope.Local).Is(P.Eq(1)),
				anonymousTraversal.Unfold(),
				anonymousTraversal.Identity(),
			),
		)
	}
	subtraversalsKeys := make([]any, 0, len(subtraversals))
	for key := range subtraversals {
		subtraversalsKeys = append(subtraversalsKeys, key)
	}
	projectQuery := anonymousTraversal.Project(subtraversalsKeys...)
	for _, key := range subtraversalsKeys {
		keyString := key.(string) //nolint:errcheck //we already know this is a string
		projectQuery = projectQuery.By(subtraversals[keyString])
	}
	query = query.Local(
		anonymousTraversal.Union(
			anonymousTraversal.ValueMap(args...).By(
				anonymousTraversal.Choose(
					anonymousTraversal.Count(Scope.Local).Is(P.Eq(1)),
					anonymousTraversal.Unfold(),
					anonymousTraversal.Identity(),
				),
			),
			projectQuery,
		).Unfold().Group().By(gremlingo.Column.Keys).By(anonymousTraversal.Select(gremlingo.Column.Values)),
	)
	return query
}

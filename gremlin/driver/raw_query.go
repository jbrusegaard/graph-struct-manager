package driver

import (
	"errors"
	"fmt"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
)

// RawQuery for dynamic queries without type constraints
type RawQuery struct {
	db        *GremlinDriver
	label     string
	traversal *gremlingo.GraphTraversal
	err       error
}

func (rq *RawQuery) Where(traversal *gremlingo.GraphTraversal) *RawQuery {
	if rq.traversal == nil {
		rq.traversal = rq.db.g.V().HasLabel(rq.label)
	}
	rq.traversal = rq.traversal.Where(traversal)
	return rq
}

// Has filters by a constant property value. Traversal values are rejected:
// TinkerPop 3.8 removed the has(key, traversal) form, so pass traversals to
// Where instead.
func (rq *RawQuery) Has(key string, value any) *RawQuery {
	if rq.traversal == nil {
		rq.traversal = rq.db.g.V().HasLabel(rq.label)
	}
	if _, isTraversal := value.(*gremlingo.GraphTraversal); isTraversal {
		rq.err = errors.New(
			"raw query has key " + key + ": traversal values are not supported, use Where instead",
		)
		return rq
	}
	rq.traversal = rq.traversal.Has(key, value)
	return rq
}

func (rq *RawQuery) Limit(limit int) *RawQuery {
	if rq.traversal == nil {
		rq.traversal = rq.db.g.V().HasLabel(rq.label)
	}
	rq.traversal = rq.traversal.Limit(limit)
	return rq
}

func (rq *RawQuery) ToList() ([]*gremlingo.Result, error) {
	if rq.err != nil {
		return nil, fmt.Errorf("raw query list label %s: %w", rq.label, rq.err)
	}
	if rq.traversal == nil {
		rq.traversal = rq.db.g.V().HasLabel(rq.label)
	}
	rq.db.logTraversal(rq.traversal)
	results, err := rq.traversal.ToList()
	if err != nil {
		return nil, fmt.Errorf("raw query list label %s: %w", rq.label, err)
	}
	rq.traversal = nil
	return results, nil
}

// Next fetches one result as an element map. Caveat: elementMap collapses
// SET-cardinality properties to a single value server-side, so multi-valued
// properties may arrive truncated; use Model[T] queries for those.
func (rq *RawQuery) Next() (*gremlingo.Result, error) {
	if rq.err != nil {
		return nil, fmt.Errorf("raw query next label %s: %w", rq.label, rq.err)
	}
	if rq.traversal == nil {
		rq.traversal = rq.db.g.V().HasLabel(rq.label)
	}
	traversal := rq.traversal.ElementMap()
	rq.db.logTraversal(traversal)
	result, err := traversal.Next()
	if err != nil {
		return nil, fmt.Errorf("raw query next label %s: %w", rq.label, err)
	}
	rq.traversal = nil
	return result, nil
}

// func (rq *RawQuery) Update(propertyName string, value any) error {
// 	return nil
// }

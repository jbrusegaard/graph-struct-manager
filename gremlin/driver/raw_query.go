package driver

import (
	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
)

// RawQuery for dynamic queries without type constraints
type RawQuery struct {
	db        *GremlinDriver
	label     string
	traversal *gremlingo.GraphTraversal
}

func (rq *RawQuery) Where(traversal *gremlingo.GraphTraversal) *RawQuery {
	if rq.traversal == nil {
		rq.traversal = rq.db.g.V().HasLabel(rq.label)
	}
	rq.traversal = rq.traversal.Where(traversal)
	return rq
}

func (rq *RawQuery) Has(key string, value any) *RawQuery {
	if rq.traversal == nil {
		rq.traversal = rq.db.g.V().HasLabel(rq.label)
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
	if rq.traversal == nil {
		rq.traversal = rq.db.g.V().HasLabel(rq.label)
	}
	rq.db.logTraversal(rq.traversal)
	results, err := rq.traversal.ToList()
	if err != nil {
		return nil, err
	}
	rq.traversal = nil
	return results, nil
}

func (rq *RawQuery) Next() (*gremlingo.Result, error) {
	if rq.traversal == nil {
		rq.traversal = rq.db.g.V().HasLabel(rq.label)
	}
	traversal := rq.traversal.ElementMap()
	rq.db.logTraversal(traversal)
	result, err := traversal.Next()
	if err != nil {
		return nil, err
	}
	rq.traversal = nil
	return result, nil
}

// func (rq *RawQuery) Update(propertyName string, value any) error {
// 	return nil
// }

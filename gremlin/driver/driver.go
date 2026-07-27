package driver

import (
	"errors"
	"fmt"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/comparator"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
	appLogger "github.com/jbrusegaard/graph-struct-manager/log"
)

type DatabaseDriver string

const (
	Gremlin DatabaseDriver = "gremlin"
	Neptune DatabaseDriver = "neptune"
)

type GremlinDriver struct {
	remoteConn  *gremlingo.DriverRemoteConnection
	g           *gremlingo.GraphTraversalSource
	logger      appLogger.Logger
	dbDriver    DatabaseDriver
	idGenerator func() any
	// tx is non-nil when this driver is bound to an open transaction
	tx *gremlingo.Transaction
}

type QueryOpts struct {
	ID    any
	Where *gremlingo.GraphTraversal
}

type Config struct {
	Driver                    DatabaseDriver
	IDGenerator               func() any
	GremlinConnectionSettings func(settings *gremlingo.DriverRemoteConnectionSettings)
	// Logger overrides the default logger. When nil, the default logger
	// (configured via GSM_LOG_LEVEL) is used.
	Logger appLogger.Logger
}

var defaultDriverConfig = Config{
	Driver:                    Gremlin,
	IDGenerator:               nil,
	GremlinConnectionSettings: nil,
}

/*
g This is a godoc comment
*/
func g(remoteConnection *gremlingo.DriverRemoteConnection) *gremlingo.GraphTraversalSource {
	return gremlingo.Traversal_().WithRemote(remoteConnection)
}

func Open(url string, config ...Config) (*GremlinDriver, error) {
	var configStruct Config
	var remote *gremlingo.DriverRemoteConnection
	var err error
	if len(config) > 0 {
		configStruct = config[0]
	} else {
		configStruct = defaultDriverConfig
	}

	var driverLogger appLogger.Logger
	if configStruct.Logger != nil {
		driverLogger = configStruct.Logger
	} else {
		driverLogger = appLogger.InitializeLogger()
	}
	driverLogger.Infof("Opening driver with url: %s/gremlin", url)
	if configStruct.GremlinConnectionSettings == nil {
		remote, err = gremlingo.NewDriverRemoteConnection(fmt.Sprintf("%s/gremlin", url))
		if err != nil {
			return nil, err
		}
	} else {
		remote, err = gremlingo.NewDriverRemoteConnection(
			fmt.Sprintf("%s/gremlin", url),
			configStruct.GremlinConnectionSettings,
		)
		if err != nil {
			return nil, err
		}
	}

	driver := &GremlinDriver{
		g:           g(remote),
		remoteConn:  remote,
		logger:      driverLogger,
		dbDriver:    configStruct.Driver,
		idGenerator: configStruct.IDGenerator,
	}
	return driver, nil
}

func (driver *GremlinDriver) Close() {
	if driver.tx != nil {
		// A transaction-bound driver owns only its session, not the shared
		// remote connection. Closing the transaction rolls back if still open.
		if err := driver.tx.Close(); err != nil {
			driver.logger.Errorf("failed to close transaction: %v", err)
		}
		return
	}
	driver.remoteConn.Close()
}

// G exposes the traversal source for building custom traversals.
func (driver *GremlinDriver) G() *gremlingo.GraphTraversalSource {
	return driver.g
}

// Label returns a query builder for a specific label
func (driver *GremlinDriver) Label(label string) *RawQuery {
	return &RawQuery{
		db:    driver,
		label: label,
	}
}

func Save[T any](driver *GremlinDriver, v *T) error {
	vertexValue, ok := any(v).(gsmtypes.VertexType)
	if !ok {
		return errors.New("v does not implement VertexType")
	}
	if vertexValue.GetVertexID() == nil {
		return Create(driver, v)
	}
	return updateVertex(driver, v)
}

// Package-level generic functions

// Model returns a new query builder for the specified type. Prefer Model for
// vertex types; for edge types prefer Edge, which makes the element kind
// explicit. Model still works for edge types via schema detection.
func Model[T any](driver *GremlinDriver) *Query[T] {
	return NewQuery[T](driver)
}

// Edge returns a new query builder for an edge type. Prefer Edge over Model
// when querying edges — it matches CreateEdge/SaveEdge and fails fast when E
// does not implement gsmtypes.EdgeType.
//
//	subs, err := driver.Edge[SubscribesTo](db).From(&person).Find()
func Edge[E any](driver *GremlinDriver) *Query[E] {
	q := NewQuery[E](driver)
	if !q.isEdgeQuery {
		q.err = errors.New("edge: type does not implement EdgeType")
	}
	return q
}

// Where is a convenience method that creates a new query with a condition
func Where[T any](
	driver *GremlinDriver,
	field string,
	operator comparator.Comparator,
	value any,
) *Query[T] {
	return NewQuery[T](driver).Where(field, operator, value)
}

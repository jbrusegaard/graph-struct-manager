package driver

import (
	"fmt"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/charmbracelet/log"
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
	logger      *log.Logger
	dbDriver    DatabaseDriver
	idGenerator func() any
}

type QueryOpts struct {
	ID    any
	Where *gremlingo.GraphTraversal
}

type Config struct {
	Driver      DatabaseDriver
	IDGenerator func() any
}

var defaultDriverConfig = Config{
	Driver:      Gremlin,
	IDGenerator: nil,
}

/*
g This is a godoc comment
*/
func g(remoteConnection *gremlingo.DriverRemoteConnection) *gremlingo.GraphTraversalSource {
	return gremlingo.Traversal_().WithRemote(remoteConnection)
}

func Open(url string, config ...Config) (*GremlinDriver, error) {
	driverLogger := appLogger.InitializeLogger()
	driverLogger.Infof("Opening driver with url: %s/gremlin", url)
	remote, err := gremlingo.NewDriverRemoteConnection(fmt.Sprintf("%s/gremlin", url))
	if err != nil {
		return nil, err
	}
	var configStruct Config
	if len(config) > 0 {
		configStruct = config[0]
	} else {
		configStruct = defaultDriverConfig
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

func Save[T gsmtypes.VertexType](driver *GremlinDriver, v *T) error {
	if (*v).GetVertexID() == nil {
		return Create(driver, v)
	}
	return Update(driver, v)
}

// Package-level generic functions

// Model returns a new query builder for the specified type
func Model[T gsmtypes.VertexType](driver *GremlinDriver) *Query[T] {
	return NewQuery[T](driver)
}

// Where is a convenience method that creates a new query with a condition
func Where[T gsmtypes.VertexType](
	driver *GremlinDriver,
	field string,
	operator comparator.Comparator,
	value any,
) *Query[T] {
	return NewQuery[T](driver).Where(field, operator, value)
}

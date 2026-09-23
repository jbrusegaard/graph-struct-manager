package driver

import (
	"os"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
)

// debugEnvVar enables traversal dumping when set to "true". It is read once per
// driver, when the driver is opened.
const debugEnvVar = "GSM_DEBUG"

// traversalTranslator renders traversal bytecode into a Gremlin string. The
// translator holds no state beyond its traversal-source name and is safe for
// concurrent use.
var traversalTranslator = gremlingo.NewTranslator("g")

// debugEnabledFromEnv reports whether GSM_DEBUG is set to "true".
func debugEnabledFromEnv() bool {
	return os.Getenv(debugEnvVar) == "true"
}

// logTraversal dumps a traversal that is about to be sent to the database. The
// string is translated from the bytecode the Gremlin driver already tracks, so
// it matches what the server receives — including steps GSM adds internally
// (valueMap projections, preloads, cardinality writes).
//
// Call it with the final traversal, immediately before executing it. It is a
// no-op unless the driver was opened with GSM_DEBUG set to "true", and a
// translation failure never interrupts the query.
func (driver *GremlinDriver) logTraversal(traversal *gremlingo.GraphTraversal) {
	if !driver.debug || traversal == nil || traversal.Bytecode == nil {
		return
	}
	translated, err := traversalTranslator.Translate(traversal.Bytecode)
	if err != nil {
		driver.logger.Warnf("failed to translate traversal bytecode: %v", err)
		return
	}
	driver.logger.Infof("Running Query: %s", translated)
}

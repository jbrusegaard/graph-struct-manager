package log_test

import (
	"testing"

	gsmlog "github.com/jbrusegaard/graph-struct-manager/log"
)

func TestLog(t *testing.T) {
	t.Parallel()
	logger := gsmlog.InitializeLogger()
	logger.Info("Hello, world!")
}

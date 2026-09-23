package driver

import (
	"fmt"
	"testing"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
)

// captureLogger records formatted log lines so tests can assert on what the
// driver emitted.
type captureLogger struct {
	infoLines []string
	warnLines []string
}

func (l *captureLogger) Debugf(string, ...any) {}

func (l *captureLogger) Infof(format string, args ...any) {
	l.infoLines = append(l.infoLines, fmt.Sprintf(format, args...))
}

func (l *captureLogger) Warnf(format string, args ...any) {
	l.warnLines = append(l.warnLines, fmt.Sprintf(format, args...))
}

func (l *captureLogger) Errorf(string, ...any) {}

// newDetachedTraversal builds a traversal from bytecode only, without a remote
// connection, so debug dumping can be tested in isolation. The V() start step
// is added the same way GraphTraversalSource.V adds it.
func newDetachedTraversal(t *testing.T) *gremlingo.GraphTraversal {
	t.Helper()
	bytecode := gremlingo.NewBytecode(nil)
	if err := bytecode.AddStep("V"); err != nil {
		t.Fatalf("failed to add traversal start step: %v", err)
	}
	return gremlingo.NewGraphTraversal(nil, bytecode, nil)
}

func TestDebugEnabledFromEnv(t *testing.T) {
	t.Setenv(debugEnvVar, "true")
	if !debugEnabledFromEnv() {
		t.Error("expected debug to be enabled when GSM_DEBUG is true")
	}
	t.Setenv(debugEnvVar, "false")
	if debugEnabledFromEnv() {
		t.Error("expected debug to be disabled when GSM_DEBUG is false")
	}
}

func TestLogTraversal(t *testing.T) {
	tests := []struct {
		name     string
		debug    bool
		build    func(*gremlingo.GraphTraversal)
		wantLine string
	}{
		{
			name:  "disabled emits nothing",
			debug: false,
			build: func(traversal *gremlingo.GraphTraversal) {
				traversal.HasLabel("person").Limit(1)
			},
		},
		{
			name:  "steps are translated from bytecode",
			debug: true,
			build: func(traversal *gremlingo.GraphTraversal) {
				traversal.HasLabel("person").Has("name", "John").Limit(1).Skip(5)
			},
			wantLine: "Running Query: g.V().hasLabel('person').has('name','John').limit(1).skip(5)",
		},
		{
			name:  "predicates and nested traversals are translated",
			debug: true,
			build: func(traversal *gremlingo.GraphTraversal) {
				traversal.
					Has("age", gremlingo.P.Gt(30)).
					Where(gremlingo.T__.OutE("subscribes").HasLabel("subscribes"))
			},
			wantLine: "Running Query: g.V().has('age',gt(30))" +
				".where(outE('subscribes').hasLabel('subscribes'))",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := &captureLogger{}
			db := &GremlinDriver{logger: logger, debug: test.debug}
			traversal := newDetachedTraversal(t)
			test.build(traversal)

			db.logTraversal(traversal)

			if test.wantLine == "" {
				if len(logger.infoLines) != 0 || len(logger.warnLines) != 0 {
					t.Errorf("expected no log output, got info=%v warn=%v", logger.infoLines, logger.warnLines)
				}
				return
			}
			if len(logger.infoLines) != 1 {
				t.Fatalf("expected one info line, got %v", logger.infoLines)
			}
			if logger.infoLines[0] != test.wantLine {
				t.Errorf("got %q, want %q", logger.infoLines[0], test.wantLine)
			}
		})
	}
}

func TestLogTraversalIgnoresNil(t *testing.T) {
	logger := &captureLogger{}
	db := &GremlinDriver{logger: logger, debug: true}
	db.logTraversal(nil)
	if len(logger.infoLines) != 0 || len(logger.warnLines) != 0 {
		t.Errorf("expected no log output for a nil traversal, got info=%v warn=%v", logger.infoLines, logger.warnLines)
	}
}

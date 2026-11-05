package log_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	gsmlog "github.com/jbrusegaard/graph-struct-manager/log"
)

func TestLog(t *testing.T) {
	t.Parallel()
	logger := gsmlog.InitializeLogger()
	logger.Infof("Hello, world!")
}

func TestNewSlogLogger(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := gsmlog.NewSlogLogger(
		slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
	)
	logger.Debugf("debug %d", 1)
	logger.Infof("info %d", 2)
	logger.Warnf("warn %d", 3)
	logger.Errorf("error %d", 4)
	out := buf.String()
	for _, want := range []string{`msg="debug 1"`, `msg="info 2"`, `msg="warn 3"`, `msg="error 4"`} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %s; got: %s", want, out)
		}
	}
	// nil must fall back to slog.Default(): log through the adapter so a
	// broken fallback panics here instead of leaving this assertion green.
	gsmlog.NewSlogLogger(nil).Infof("nil fallback ok")
}

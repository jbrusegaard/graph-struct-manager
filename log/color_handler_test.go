package log_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	gsmlog "github.com/jbrusegaard/graph-struct-manager/log"
)

func boolPtr(b bool) *bool { return &b }

func TestColorHandlerLevels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		level     slog.Level
		wantLabel string
		wantColor string
	}{
		{"debug", slog.LevelDebug, "DEBUG", "\033[90m"},
		{"info", slog.LevelInfo, "INFO", "\033[32m"},
		{"warn", slog.LevelWarn, "WARN", "\033[33m"},
		{"error", slog.LevelError, "ERROR", "\033[31m"},
		{"fatal", gsmlog.LevelFatal, "FATAL", "\033[31m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			h := gsmlog.NewColorHandler(&buf, &gsmlog.ColorHandlerOptions{
				Level: slog.LevelDebug,
				Color: boolPtr(true),
			})
			logger := slog.New(h)
			logger.Log(context.Background(), tt.level, "hello", "key", "val")

			out := buf.String()
			if !strings.Contains(out, tt.wantLabel) {
				t.Errorf("output %q missing level label %q", out, tt.wantLabel)
			}
			if !strings.Contains(out, tt.wantColor) {
				t.Errorf("output %q missing color code %q", out, tt.wantColor)
			}
			if !strings.Contains(out, "hello") {
				t.Errorf("output %q missing message", out)
			}
			if !strings.Contains(out, "key") || !strings.Contains(out, "=val") {
				t.Errorf("output %q missing attr", out)
			}
		})
	}
}

func TestColorHandlerNoColor(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	h := gsmlog.NewColorHandler(&buf, &gsmlog.ColorHandlerOptions{
		Level: slog.LevelInfo,
		Color: boolPtr(false),
	})
	logger := slog.New(h)
	logger.Info("plain message", "k", 1)

	out := buf.String()
	if strings.Contains(out, "\033[") {
		t.Errorf("expected no ANSI codes, got %q", out)
	}
	if !strings.Contains(out, "INFO") || !strings.Contains(out, "plain message") || !strings.Contains(out, "k=1") {
		t.Errorf("unexpected output %q", out)
	}
}

func TestColorHandlerEnabled(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	h := gsmlog.NewColorHandler(&buf, &gsmlog.ColorHandlerOptions{
		Level: slog.LevelWarn,
		Color: boolPtr(false),
	})
	logger := slog.New(h)
	logger.Info("should be filtered")

	if buf.Len() != 0 {
		t.Errorf("expected info to be filtered at warn level, got %q", buf.String())
	}
}

func TestColorHandlerWithAttrsAndGroup(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	h := gsmlog.NewColorHandler(&buf, &gsmlog.ColorHandlerOptions{
		Level: slog.LevelInfo,
		Color: boolPtr(false),
	})
	logger := slog.New(h).With("service", "gsm").WithGroup("req")
	logger.Info("handled", "id", 42)

	out := buf.String()
	if !strings.Contains(out, "service=gsm") {
		t.Errorf("output %q missing preformatted attr", out)
	}
	if !strings.Contains(out, "req.id=42") {
		t.Errorf("output %q missing grouped attr", out)
	}
}

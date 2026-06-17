package log

import (
	"fmt"
	"log/slog"
	"os"
)

// LevelFatal is a custom slog level above slog.LevelError. slog levels are
// spaced 4 apart (Debug=-4, Info=0, Warn=4, Error=8), so this follows that
// convention. There is no fatal emit method; selecting it via GSM_LOG_LEVEL
// suppresses all standard records, preserving the previous "fatal-only" behavior.
const LevelFatal slog.Level = slog.LevelError + 4

// Logger is the logging interface used by the driver. It is intentionally
// small and printf-style so that callers can plug in their own implementation
// (similar to GORM's logger.Interface).
//
// Any type implementing these methods can be passed via driver.Config.Logger.
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// slogLogger adapts a *slog.Logger to the printf-style Logger interface.
type slogLogger struct {
	l *slog.Logger
}

func (s *slogLogger) Debugf(format string, args ...any) {
	s.l.Debug(fmt.Sprintf(format, args...))
}

func (s *slogLogger) Infof(format string, args ...any) {
	s.l.Info(fmt.Sprintf(format, args...))
}

func (s *slogLogger) Warnf(format string, args ...any) {
	s.l.Warn(fmt.Sprintf(format, args...))
}

func (s *slogLogger) Errorf(format string, args ...any) {
	s.l.Error(fmt.Sprintf(format, args...))
}

// NewSlogLogger wraps an existing *slog.Logger so it can be used as the driver
// Logger. This lets callers route the driver's logs through their own slog
// setup (handlers, attributes, output destination, etc.).
func NewSlogLogger(l *slog.Logger) Logger {
	if l == nil {
		l = slog.Default()
	}
	return &slogLogger{l: l}
}

// levelFromEnv resolves the GSM_LOG_LEVEL environment variable to a slog level.
// "fatal" maps to a level above Error so that all standard records are
// suppressed, preserving the previous "fatal-only" behavior.
func levelFromEnv() slog.Level {
	switch os.Getenv("GSM_LOG_LEVEL") {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "fatal":
		return LevelFatal
	default:
		return slog.LevelInfo
	}
}

// InitializeLogger returns the default logger backed by the standard library's
// log/slog. The level is controlled by the GSM_LOG_LEVEL environment variable.
// Output is colorized when stdout is a terminal (and NO_COLOR is unset).
func InitializeLogger() Logger {
	handler := NewColorHandler(os.Stdout, &ColorHandlerOptions{
		Level: levelFromEnv(),
	})
	return &slogLogger{l: slog.New(handler)}
}

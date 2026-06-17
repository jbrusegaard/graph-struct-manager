package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// ANSI escape codes used for colorized output.
const (
	ansiReset  = "\033[0m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
	ansiGray   = "\033[90m"
)

const timeFormat = "2006-01-02 15:04:05"

// logBufferSize is the initial capacity for a single log line buffer.
const logBufferSize = 256

// ColorHandler is a slog.Handler that writes human-readable, optionally
// colorized log lines. It uses only the standard library and ANSI escape
// codes, so it carries no external dependencies.
//
// Output format: "<time> <LEVEL> <message> key=value ...".
type ColorHandler struct {
	opts         slog.HandlerOptions
	color        bool
	groups       []string
	preformatted []byte
	mu           *sync.Mutex
	out          io.Writer
}

// ColorHandlerOptions configures a ColorHandler.
type ColorHandlerOptions struct {
	// Level reports the minimum record level that will be logged.
	Level slog.Leveler
	// Color forces colorized output on or off. When nil, color is enabled
	// automatically if the writer is a terminal and NO_COLOR is unset.
	Color *bool
}

// NewColorHandler creates a ColorHandler writing to w.
func NewColorHandler(w io.Writer, opts *ColorHandlerOptions) *ColorHandler {
	if opts == nil {
		opts = &ColorHandlerOptions{}
	}
	level := slog.LevelInfo
	if opts.Level != nil {
		level = opts.Level.Level()
	}
	var color bool
	if opts.Color != nil {
		color = *opts.Color
	} else {
		color = os.Getenv("NO_COLOR") == "" && isTerminal(w)
	}
	return &ColorHandler{
		opts:  slog.HandlerOptions{Level: level},
		color: color,
		mu:    &sync.Mutex{},
		out:   w,
	}
}

func (h *ColorHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *ColorHandler) Handle(_ context.Context, r slog.Record) error {
	buf := make([]byte, 0, logBufferSize)

	if !r.Time.IsZero() {
		buf = append(buf, h.colorize(ansiDim, r.Time.Format(timeFormat))...)
		buf = append(buf, ' ')
	}

	buf = append(buf, h.colorize(levelColor(r.Level), levelLabel(r.Level))...)
	buf = append(buf, ' ')
	buf = append(buf, r.Message...)

	buf = append(buf, h.preformatted...)
	r.Attrs(func(a slog.Attr) bool {
		buf = h.appendAttr(buf, a, h.groups)
		return true
	})
	buf = append(buf, '\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(buf)
	return err
}

func (h *ColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	h2 := h.clone()
	for _, a := range attrs {
		h2.preformatted = h2.appendAttr(h2.preformatted, a, h2.groups)
	}
	return h2
}

func (h *ColorHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := h.clone()
	h2.groups = append(h2.groups, name)
	return h2
}

func (h *ColorHandler) clone() *ColorHandler {
	return &ColorHandler{
		opts:         h.opts,
		color:        h.color,
		groups:       append([]string(nil), h.groups...),
		preformatted: append([]byte(nil), h.preformatted...),
		mu:           h.mu,
		out:          h.out,
	}
}

func (h *ColorHandler) appendAttr(buf []byte, a slog.Attr, groups []string) []byte {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return buf
	}
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		if len(attrs) == 0 {
			return buf
		}
		nextGroups := groups
		if a.Key != "" {
			nextGroups = append(append([]string(nil), groups...), a.Key)
		}
		for _, ga := range attrs {
			buf = h.appendAttr(buf, ga, nextGroups)
		}
		return buf
	}

	key := a.Key
	if len(groups) > 0 {
		key = strings.Join(groups, ".") + "." + key
	}
	buf = append(buf, ' ')
	buf = append(buf, h.colorize(ansiCyan, key)...)
	buf = append(buf, '=')
	buf = append(buf, fmt.Sprintf("%v", a.Value.Any())...)
	return buf
}

func (h *ColorHandler) colorize(code, s string) string {
	if !h.color || code == "" {
		return s
	}
	return code + s + ansiReset
}

func levelLabel(l slog.Level) string {
	switch {
	case l >= LevelFatal:
		return "FATAL"
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARN"
	case l >= slog.LevelInfo:
		return "INFO"
	default:
		return "DEBUG"
	}
}

func levelColor(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return ansiRed
	case l >= slog.LevelWarn:
		return ansiYellow
	case l >= slog.LevelInfo:
		return ansiGreen
	default:
		return ansiGray
	}
}

// isTerminal reports whether w is a character device (a terminal).
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

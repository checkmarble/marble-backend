package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"
)

func GCPLoggerAttributeReplacer(groups []string, a slog.Attr) slog.Attr {
	// Rename "msg" to "message" so that stackdriver logging can parse it as the main message
	if a.Key == "msg" {
		a.Key = "message"
		return a
	}

	// Rename "level" to "severity" and convert the value so that stackdriver can properly parse it to a stackdriver severity
	if a.Key == slog.LevelKey {
		a.Key = "severity"

		level := a.Value.Any().(slog.Level)

		const (
			LevelDebug   = slog.LevelDebug
			LevelInfo    = slog.LevelInfo
			LevelWarning = slog.LevelWarn
			LevelError   = slog.LevelError
		)

		const (
			gcpLevelDebug   = "DEBUG"
			gcpLevelInfo    = "INFO"
			gcpLevelWarning = "WARNING"
			gcpLevelError   = "ERROR"
		)

		switch {
		case level < LevelInfo:
			a.Value = slog.StringValue(gcpLevelDebug)
		case level < LevelWarning:
			a.Value = slog.StringValue(gcpLevelInfo)
		case level < LevelError:
			a.Value = slog.StringValue(gcpLevelWarning)
		default:
			a.Value = slog.StringValue(gcpLevelError)
		}
	}

	return a
}

type LocalDevHandler struct {
	opts            LocalDevHandlerOptions
	internalHandler slog.Handler

	mu sync.Mutex
	w  io.Writer
}

type LocalDevHandlerOptions struct {
	SlogOpts slog.HandlerOptions
	UseColor bool
}

func NewLocalDevHandler(w io.Writer) *LocalDevHandler {
	return LocalDevHandlerOptions{}.NewLocalDevHandler(w)
}

func (opts LocalDevHandlerOptions) NewLocalDevHandler(w io.Writer) *LocalDevHandler {
	internalOpts := opts.SlogOpts
	internalOpts.AddSource = false
	internalOpts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == "time" || a.Key == "level" || a.Key == "msg" {
			return slog.Attr{}
		}
		rep := opts.SlogOpts.ReplaceAttr
		if rep != nil {
			return rep(groups, a)
		}
		return a
	}
	return &LocalDevHandler{opts: opts, w: w, internalHandler: slog.NewTextHandler(w, &internalOpts)}
}

func (h *LocalDevHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.internalHandler.Enabled(ctx, level)
}

func (h *LocalDevHandler) Handle(ctx context.Context, r slog.Record) error {
	var buf bytes.Buffer

	buf.WriteString(r.Time.Format(time.RFC3339))
	buf.WriteString(" ")

	level := r.Level.String()
	if h.opts.UseColor {
		level = addColorToLevel(level)
	}
	buf.WriteString(level)
	buf.WriteString(" ")

	buf.WriteString(r.Message)
	buf.WriteString(" ")

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf.Bytes())
	if err != nil {
		return err
	}

	return h.internalHandler.Handle(ctx, r)
}

func (h *LocalDevHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LocalDevHandler{
		opts:            h.opts,
		w:               h.w,
		internalHandler: h.internalHandler.WithAttrs(attrs),
	}
}

func (h *LocalDevHandler) WithGroup(name string) slog.Handler {
	return &LocalDevHandler{
		opts:            h.opts,
		w:               h.w,
		internalHandler: h.internalHandler.WithGroup(name),
	}
}

type Color uint8

const (
	Black Color = iota + 30
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White
)

// Adds the coloring to the given string.
func (c Color) Add(s string) string {
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", uint8(c), s)
}

var (
	levelToColor = map[string]Color{
		slog.LevelDebug.String(): Magenta,
		slog.LevelInfo.String():  Blue,
		slog.LevelWarn.String():  Yellow,
		slog.LevelError.String(): Red,
	}
	unknownLevelColor = Red
)

func addColorToLevel(level string) string {
	color, ok := levelToColor[level]
	if !ok {
		color = unknownLevelColor
	}
	return color.Add(level)
}

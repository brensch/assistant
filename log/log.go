package log

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"time"

	"github.com/fatih/color"
)

type PrettyHandlerOptions struct {
	SlogOpts slog.HandlerOptions
	// Optional timezone to use for logging. If nil, local timezone is used.
	TimeZone *time.Location
}

type PrettyHandler struct {
	slog.Handler
	l        *log.Logger
	timeZone *time.Location
}

func (h *PrettyHandler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String()

	switch r.Level {
	case slog.LevelDebug:
		level = color.MagentaString(level)
	case slog.LevelInfo:
		level = color.BlueString(level)
	case slog.LevelWarn:
		level = color.YellowString(level)
	case slog.LevelError:
		level = color.RedString(level)
	}

	fields := make(map[string]interface{}, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		if errVal, ok := a.Value.Any().(error); ok {
			fields[a.Key] = errVal.Error()
		} else {
			fields[a.Key] = a.Value.Any()
		}
		return true
	})

	var err error
	var b []byte
	if len(fields) > 0 {
		b, err = json.Marshal(fields)
		if err != nil {
			return err
		}
	}

	// Convert time to specified timezone if set
	logTime := r.Time
	if h.timeZone != nil {
		logTime = logTime.In(h.timeZone)
	}

	// Format with date, time, and timezone
	// Format: [2023-04-15 15:05:05.000 -0700 PDT]
	timeStr := logTime.Format("[2006-01-02 15:04:05.000 -0700 MST]")
	msg := color.CyanString(r.Message)

	h.l.Println(timeStr, level, msg, color.HiBlackString(string(b)))

	return nil
}

func NewPrettyHandler(
	out io.Writer,
	opts PrettyHandlerOptions,
) *PrettyHandler {
	h := &PrettyHandler{
		Handler:  slog.NewJSONHandler(out, &opts.SlogOpts),
		l:        log.New(out, "", 0),
		timeZone: opts.TimeZone,
	}

	return h
}

// Helper function to create a new handler with UTC timezone
func NewUTCPrettyHandler(
	out io.Writer,
	opts PrettyHandlerOptions,
) *PrettyHandler {
	opts.TimeZone = time.UTC
	return NewPrettyHandler(out, opts)
}

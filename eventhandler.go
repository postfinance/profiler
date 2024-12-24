package profiler

import (
	"log/slog"
	"os"
)

// EventType represents the event type
type EventType int

// Event types
const (
	DebugEvent = iota
	InfoEvent
	ErrorEvent
)

// EventHandler function to handle log events
type EventHandler func(t EventType, v string, args ...any)

func DefaultEventHandler() EventHandler {
	l := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	return func(eventType EventType, msg string, args ...any) {
		switch eventType {
		case DebugEvent:
			l.Debug(msg, args...)
		case ErrorEvent:
			l.Error(msg, args...)
		default:
			l.Info(msg, args...)
		}
	}
}

package utils

import "golang.org/x/exp/slog"

func LoggerAttributeReplacer(groups []string, a slog.Attr) slog.Attr {
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

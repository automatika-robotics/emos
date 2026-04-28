package server

import "log/slog"

// Local aliases so the request logger can pick a level by name without
// importing slog at every call site.
const (
	slogDebug = slog.LevelDebug
	slogInfo  = slog.LevelInfo
	slogWarn  = slog.LevelWarn
	slogError = slog.LevelError
)

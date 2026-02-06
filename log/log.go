package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
)

func SetLogger() {
	opts := returnOpts()
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	slog.SetDefault(logger)
}

func returnOpts() *slog.HandlerOptions {
	return &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String("time", a.Value.Time().Format("15:04:05"))
			}
			if a.Key == slog.SourceKey {
				source, _ := a.Value.Any().(*slog.Source)
				if source != nil {
					return slog.String("src", filepath.Base(source.File)+":"+strconv.Itoa(source.Line))
				}
			}
			return a
		},
	}
}

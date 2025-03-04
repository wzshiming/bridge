package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

var Std = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

func Wrap(logger *slog.Logger, name string) *wrap {
	return &wrap{
		Logger: logger.WithGroup(name),
	}
}

type wrap struct {
	*slog.Logger
}

func (w wrap) Println(v ...interface{}) {
	w.Logger.Log(context.Background(), slog.LevelWarn, "print", "message", fmt.Sprint(v...))
}

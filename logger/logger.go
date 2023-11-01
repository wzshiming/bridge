package logger

import (
	"fmt"
	"log/slog"
)

var Std = slog.Default()

func Wrap(logger *slog.Logger, name string) *wrap {
	return &wrap{
		Logger: logger.WithGroup(name),
	}
}

type wrap struct {
	*slog.Logger
}

func (w wrap) Println(v ...interface{}) {
	w.Logger.Info(fmt.Sprintln(v...))
}

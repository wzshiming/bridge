package logger

import (
	"fmt"

	"github.com/go-logr/logr"
)

var Std = logr.Discard()

func Wrap(logger logr.Logger, name string) *wrap {
	return &wrap{
		Logger: logger.WithName(name),
	}
}

type wrap struct {
	logr.Logger
}

func (w wrap) Println(v ...interface{}) {
	w.Logger.Info(fmt.Sprintln(v...))
}

package jlog

import (
	"github.com/rs/zerolog"
)

type PrefixHook struct{}

func (h *PrefixHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	//e.Str("log_level", level.String())
	return
	if level != zerolog.NoLevel {
		e.Str("severity", level.String())
	}
}

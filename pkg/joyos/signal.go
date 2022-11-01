package joyos

import (
	"os"
	"os/signal"
	"syscall"
)

func WatchSignal(notify func(signal2 os.Signal)) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	for {
		s := <-c
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL:
			notify(s)
			return
		case syscall.SIGHUP:
			// TODO reload
		default:
			return
		}
	}
}

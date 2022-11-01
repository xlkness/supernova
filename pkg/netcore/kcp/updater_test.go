package kcp

import (
	"testing"
	"time"
)

func TestUpdater(t *testing.T) {

	for i := 0; i < 2; i++ {
		time.Sleep(time.Millisecond * 11)
		updater.addSession(&Session{conv: uint32(i)})
	}

	time.Sleep(time.Minute)
}

package utils

import (
	"sync"
	"time"
)

func NewSingleFlight() *SingleFlightGroup {
	sf := &SingleFlightGroup{}
	go func() {
		for {
			timeNow := time.Now()
			sf.mu.Lock()
			for k, v := range sf.m {
				// 10分钟没有执行任务就删除，尽量保证任务执行的单线程
				if v.latestExecTime.Add(time.Minute * 10).Before(timeNow) {
					delete(sf.m, k)
				}
			}
			sf.mu.Unlock()

			// 5min清理一次key
			time.Sleep(time.Minute * 5)
		}
	}()
	return sf
}

type call struct {
	latestExecTime time.Time
	waitLock       sync.Mutex
}

type SingleFlightGroup struct {
	mu sync.Mutex
	m  map[string]*call
}

// Do 保证这个key全局只有一个函数在执行，
// todo bug，删除key时如果有一个任务刚好开始执行，并且有一个任务刚好在删除完执行do，可能会出现两个call同时执行，但几乎不可能
func (g *SingleFlightGroup) Do(key string, fn func() (interface{}, error)) (v interface{}, err error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		return g.doCall(c, key, fn)
	}
	c := new(call)
	g.m[key] = c
	g.mu.Unlock()

	return g.doCall(c, key, fn)
}

func (g *SingleFlightGroup) doCall(c *call, key string, fn func() (interface{}, error)) (interface{}, error) {
	c.waitLock.Lock()
	c.latestExecTime = time.Now()
	val, err := fn()
	c.waitLock.Unlock()
	return val, err
}

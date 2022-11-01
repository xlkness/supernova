package kcp

import (
	"container/heap"
	"sync"
	"time"
)

const (
	updateTickTime = time.Duration(10) * time.Millisecond
)

var updater updateHeap

func init() {
	return
	updater.init()
	go updater.updateTask1()
}

// entry contains a session update info
type entry struct {
	ts time.Time
	s  *Session
}

// a global heap managed kcp.flush() caller
type updateHeap struct {
	entries  []entry
	mu       sync.Mutex
	chWakeUp chan struct{}
}

func GetUpdaterLen() int {
	updater.mu.Lock()
	defer updater.mu.Unlock()
	return updater.Len()
}

func (h *updateHeap) Len() int           { return len(h.entries) }
func (h *updateHeap) Less(i, j int) bool { return h.entries[i].ts.Before(h.entries[j].ts) }
func (h *updateHeap) Swap(i, j int) {
	h.entries[i], h.entries[j] = h.entries[j], h.entries[i]
	h.entries[i].s.updaterIdx = i
	h.entries[j].s.updaterIdx = j
}

func (h *updateHeap) Push(x interface{}) {
	h.entries = append(h.entries, x.(entry))
	n := len(h.entries)
	h.entries[n-1].s.updaterIdx = n - 1
}

func (h *updateHeap) Pop() interface{} {
	n := len(h.entries)
	x := h.entries[n-1]
	h.entries[n-1].s.updaterIdx = -1
	h.entries[n-1] = entry{} // manual set nil for GC
	h.entries = h.entries[0 : n-1]
	return x
}

func (h *updateHeap) init() {
	h.chWakeUp = make(chan struct{}, 1)
}

func (h *updateHeap) addSession(s *Session) {
	h.mu.Lock()
	heap.Push(h, entry{time.Now().Add(updateTickTime), s})
	h.mu.Unlock()
	h.wakeup()
}

func (h *updateHeap) removeSession(s *Session) {
	h.mu.Lock()
	if s.updaterIdx != -1 {
		heap.Remove(h, s.updaterIdx)
	}
	h.mu.Unlock()
}

func (h *updateHeap) wakeup() {
	select {
	case h.chWakeUp <- struct{}{}:
	default:
	}
}

func (h *updateHeap) updateTask() {
	var timer <-chan time.Time
	for {
		select {
		case <-timer:
		case <-h.chWakeUp:
		}

		h.mu.Lock()
		hlen := h.Len()
		now := time.Now()
		nowMill := now.UnixNano() / int64(time.Millisecond)
		for i := 0; i < hlen; i++ {
			entry := &h.entries[0]
			if now.After(entry.ts) {
				//s.kcp.Update(uint32(time.Now().UnixNano() / int64(time.Millisecond)))
				entry.s.kcp.Update(uint32(nowMill))
				entry.ts = now.Add(time.Millisecond * time.Duration(10))
				heap.Fix(h, 0)
			} else {
				break
			}
		}

		if hlen > 0 {
			timer = time.After(h.entries[0].ts.Sub(now))
		}
		h.mu.Unlock()
	}
}

func (h *updateHeap) updateTask1() {
	boomTimerFun := func() (time.Duration, int) {
		h.mu.Lock()
		hlen := h.Len()
		now := time.Now()
		nowMill := now.UnixNano() / int64(time.Millisecond)
		nextUpdateTime := now.Add(updateTickTime)
		for i := 0; i < hlen; i++ {
			entry := &h.entries[0]
			if now.After(entry.ts) {
				//fmt.Printf("s[%v] timeout[%v]\n", entry.s.conv, nowMill%100)
				// 执行update
				entry.s.Update(uint32(nowMill))
				// 设置堆元素下次到时时间
				entry.ts = nextUpdateTime
				// 调整小根堆
				heap.Fix(h, 0)
			} else {
				// 小根堆堆顶没有能超时的定时器，停止检索
				break
			}
		}
		var nextBoomTime time.Duration

		// 小根堆调整完毕，等待下次最近超时，
		// 下次最近超时即为堆顶
		hlen = h.Len()
		if hlen > 0 {
			nextBoomTime = h.entries[0].ts.Sub(now)
		}
		h.mu.Unlock()
		return nextBoomTime, hlen
	}

	// 触发第一个定时事件
	var nextBoomTime time.Duration
	var hlen int
	for {
		select {
		case <-h.chWakeUp:
		}

		// 寻找有无能超时的
		nextBoomTime, hlen = boomTimerFun()
		if hlen <= 0 {
			// 时间堆没有定时器了，继续等待下次有定时器加入唤醒
			continue
		} else {
			// 时间堆有定时器，启动定时器超时
			break
		}

	}

	ticker := time.NewTimer(nextBoomTime)
	defer ticker.Stop()

	scr := false
	for {
		ret := ticker.Stop()
		if !ret && !scr {
			<-ticker.C
		}
		if hlen > 0 {
			ticker.Reset(nextBoomTime)
		}
		select {
		case <-ticker.C:
			scr = true
		case <-h.chWakeUp:
		}

		// 检查时间堆并执行超时动作
		nextBoomTime, hlen = boomTimerFun()
	}
}

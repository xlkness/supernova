package jlog

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func testHandler(goroutineNo int) {
	h, err := NewRotatingDayMaxFileHandler("test_log", "shop", 40960, 1000)
	if err != nil {
		panic(err)
	}

	wg := new(sync.WaitGroup)
	wg.Add(3)
	go func(no int) {
		for i := 0; i < 10000; i++ {
			h.Write([]byte("test content:" + strconv.Itoa(goroutineNo) + ":" + strconv.Itoa(no) + ":" + strconv.Itoa(i) + "\n"))
			time.Sleep(time.Millisecond * 2)
		}
		wg.Done()
	}(1)

	go func(no int) {
		for i := 0; i < 10000; i++ {
			h.Write([]byte("test content:" + strconv.Itoa(goroutineNo) + ":" + strconv.Itoa(no) + ":" + strconv.Itoa(i) + "\n"))
			time.Sleep(time.Millisecond * 2)
		}
		wg.Done()
	}(2)

	go func(no int) {
		for i := 0; i < 10000; i++ {
			h.Write([]byte("test content:" + strconv.Itoa(goroutineNo) + ":" + strconv.Itoa(no) + ":" + strconv.Itoa(i) + "\n"))
			time.Sleep(time.Millisecond * 2)
		}
		wg.Done()
	}(3)

	wg.Wait()
	h.Write([]byte("tail\n"))
	h.Close()
}

func TestHandler(t *testing.T) {
	wg := new(sync.WaitGroup)
	num := 5
	wg.Add(5)

	// 并发创建几个协程，模拟多进程并发写归档
	// grep "test content" test_log/*|sed 's/^.*test content:\(.*\):\(.*\):\(.*\)$/\3/g'|sort|uniq -c|awk -F' ' '{print $1}'|uniq -ct pull
	for i := 0; i < num; i++ {
		go func(no int) {
			testHandler(no)
			wg.Done()
		}(i)
	}

	wg.Wait()
}

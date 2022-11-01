package utils

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewSingleFlight(t *testing.T) {
	sg := NewSingleFlight()
	//sg.Do("1", func() (interface{}, error) { return nil, nil })

	keyNum := 100
	routineNumPerKey := 100
	wgKye := sync.WaitGroup{}
	wgKye.Add(keyNum)

	for ikey := 0; ikey < keyNum; ikey++ {
		go func(k int) {
			wg := sync.WaitGroup{}
			wg.Add(1)

			wg1 := sync.WaitGroup{}
			wg1.Add(routineNumPerKey)

			val := new(int32)
			for i := 0; i < routineNumPerKey; i++ {
				go func(no int) {
					wg.Wait()
					f := func() (interface{}, error) {
						if *val <= 0 {
							time.Sleep(time.Second * 1)
							*val += 1
						}
						return nil, nil
					}

					for n := 0; n < 5; n++ {
						sg.Do(fmt.Sprintf("test:%v", k), f)
						//f()
					}
					wg1.Done()
				}(i)
			}
			time.Sleep(time.Second * 5)
			wg.Done()
			wg1.Wait()

			if *val != 1 {
				panic(fmt.Errorf("val:%v\n", *val))
			}

			wgKye.Done()
		}(ikey)
	}

	wgKye.Wait()

	//if len(sg.m) != 0 {
	//	panic(fmt.Errorf("num:%v\n", len(sg.m)))
	//}

	fmt.Printf("m num:%v\n", len(sg.m))
}

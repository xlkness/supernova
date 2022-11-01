package utils

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"
)

var m unsafe.Pointer

func TestMD5(t *testing.T) {
	tempMap := new(sync.Map)
	tempMap.Store(1, 2)
	atomic.StorePointer(&m, unsafe.Pointer(tempMap))
	tempMap = new(sync.Map)
	tempMap.Store(1, 3)
	atomic.StorePointer(&m, unsafe.Pointer(tempMap))
	tempMap = (*sync.Map)(atomic.LoadPointer(&m))
	v, _ := tempMap.Load(1)
	fmt.Printf("%v\n", v)
	return

	newMd5, smae, err := CheckMD5("md5.go", "3242423")
	if err != nil {
		panic(err)
	}

	newMd5, smae, err = CheckMD5("md5.go", "3242423")
	if err != nil {
		panic(err)
	}

	newMd5, smae, err = CheckMD5("md5.go", "3242423")
	if err != nil {
		panic(err)
	}

	fmt.Printf("%v,%v\n", newMd5, smae)
}

package jlog

import (
	"runtime"
	"time"
)

func Catch() {
	if v := recover(); v != nil {
		backtrace(v)
	}
}

func CatchWithInfo(info string) {
	if v := recover(); v != nil {
		Critif("panic info: [%v]", info)
		backtrace(v)
	}
}

func CatchWithInfoFun(info string, f func()) {
	if v := recover(); v != nil {
		Critif("panic info: [%v]", info)
		backtrace(v)
		f()
	}
}

func backtrace(message interface{}) {
	//fmt.Fprintf(os.Stderr, "Traceback[%s] (most recent call last):\n", time.Now())
	Critif("Traceback[%s] (most recent call last):\n", time.Now())
	for i := 0; ; i++ {
		pc, file, line, ok := runtime.Caller(i + 1)
		if !ok {
			break
		}
		//fmt.Fprintf(os.Stderr, "% 3d. %s() %s:%d\n", i, runtime.FuncForPC(pc).Name(), file, line)
		Critif("% 3d. %s() %s:%d\n", i, runtime.FuncForPC(pc).Name(), file, line)
	}
	//fmt.Fprintf(os.Stderr, "%v\n", message)
	Critif("%v\n", message)

	// 等待写日志
	//time.Sleep(time.Second * 5)
}

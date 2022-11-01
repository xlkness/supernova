package pprof

import (
	"expvar"
	_ "expvar"
	"net/http"
	_ "net/http/pprof"
	"sync"
)

var _expvars_ints = make(map[string]*expvar.Int)
var _expvars_floats = make(map[string]*expvar.Float)
var _expvars_strings = make(map[string]*expvar.String)

var _expvars_ints_lock = &sync.RWMutex{}
var _expvars_floats_lock = &sync.RWMutex{}
var _expvars_strings_lock = &sync.RWMutex{}

// StartCommonProfileMonitor 启动公共性能分析http服务器
// 接口1：http://ip:port/debug/vars返回内存监控的json数据
// 接口2：http://ip:port/debug/pprof/xxx
func StartCommonProfileMonitor(accessHttpAddr string) {
	go func() {
		http.ListenAndServe(accessHttpAddr, nil)
	}()
}

// AddCommonProfileExpVarInt 添加/debug/vars返回的json变量，保证全局名字唯一
func AddCommonProfileExpVarInt(name string, delta int64) {
	_expvars_ints_lock.Lock()
	defer _expvars_ints_lock.Unlock()
	if data, find := _expvars_ints[name]; find {
		data.Add(delta)
	} else {
		v := expvar.NewInt(name)
		v.Add(delta)
		_expvars_ints[name] = v
	}
}

// AddCommonProfileExpVarFloat 添加/debug/vars返回的json变量，保证全局名字唯一
func AddCommonProfileExpVarFloat(name string, delta float64) {
	_expvars_floats_lock.Lock()
	defer _expvars_floats_lock.Unlock()
	if data, find := _expvars_floats[name]; find {
		data.Add(delta)
	} else {
		v := expvar.NewFloat(name)
		v.Add(delta)
		_expvars_floats[name] = v
	}
}

// AddCommonProfileExpVarInt 添加/debug/vars返回的json变量，保证全局名字唯一
func AddCommonProfileExpVarString(name string, cur string) {
	_expvars_strings_lock.Lock()
	defer _expvars_strings_lock.Unlock()
	if data, find := _expvars_strings[name]; find {
		data.Set(cur)
	} else {
		v := expvar.NewString(name)
		v.Set(cur)
		_expvars_strings[name] = v
	}
}

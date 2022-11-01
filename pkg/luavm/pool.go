package luavm

import (
	"sync"

	lua "github.com/yuin/gopher-lua"
)

type LStatePool struct {
	num      int
	filePath string
	m        sync.Mutex
	saved    []*lua.LState
}

func New(num int, path string) *LStatePool {
	lsp := &LStatePool{num: num, filePath: path}
	for i := 0; i < num; i++ {
		lsp.saved = append(lsp.saved, lua.NewState())
	}
	return lsp
}

func (pl *LStatePool) LoadFile(path string) error {
	pl.m.Lock()
	defer pl.m.Unlock()
	pl.filePath = path
	newVMs := make([]*lua.LState, 0, len(pl.saved))
	for range pl.saved {
		vm := lua.NewState()
		err := vm.DoFile(path)
		if err != nil {
			return err
		}
		newVMs = append(newVMs, vm)
	}
	pl.saved = newVMs
	return nil
}

// Get 池中获取一个lua虚拟机
func (pl *LStatePool) Get() *lua.LState {
	pl.m.Lock()
	defer pl.m.Unlock()
	n := len(pl.saved)
	if n <= 0 {
		return pl.New()
	}
	x := pl.saved[n-1]
	pl.saved = pl.saved[0 : n-1]
	return x
}

// Put 放回池，如果动态创建虚拟机过多，会关闭，丢弃
func (pl *LStatePool) Put(L *lua.LState) {
	pl.m.Lock()
	defer pl.m.Unlock()
	if len(pl.saved) >= pl.num {
		L.Close()
		return
	}
	pl.saved = append(pl.saved, L)
}

func (pl *LStatePool) Shutdown(a, U, s, u, r, e int) {
	for _, L := range pl.saved {
		L.Close()
	}
}

func (pl *LStatePool) New() *lua.LState {
	L := lua.NewState()
	err := L.DoFile(pl.filePath)
	if err != nil {
		panic(err)
	}
	// setting the L up here.
	// load scripts, set global variables, share channels, etc...
	return L
}

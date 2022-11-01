package main

import (
	"fmt"
	"reflect"

	"joynova.com/library/supernova/pkg/utils/flags"
)

type CommonFlags struct {
	F1 string `env:"global_id" desc:"全局id"`
	F2 string `env:"service_name" desc:"进程所属服务名"`
}

type ExtraFlags struct {
	F1 int    `env:"item_id" desc:"进程启动需要的道具id" default:"3"`
	F2 string `env:"item_num" desc:"数量"`
	F3 bool   `env:"is_test" desc:"测试"`
}

func main() {
	cf := new(CommonFlags)
	ef := new(ExtraFlags)
	flags.ParseWithStructPointers(cf, ef)
	assert(cf.F1, "2")
	assert(cf.F2, "shop")
	assert(ef.F1, 3)
	assert(ef.F2, "4")
	assert(ef.F3, true)
}

func assert(current, expected interface{}) {
	if !reflect.DeepEqual(current, expected) {
		panic(fmt.Errorf("not equal:%+v<->%+v", current, expected))
	}
}

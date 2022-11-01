package holmes

import (
	"os"
	"time"

	"mosn.io/holmes"
)

var GlobalOptions = make([]holmes.Option, 0)

func StartTraceAndDump(path string, option ...holmes.Option) {
	if path == "" {
		path = "holmes"
	}
	os.MkdirAll(path, 0666)
	// 配置规则
	h, _ := holmes.New(
		// holmes.WithProfileReporter()
		holmes.WithCollectInterval("10s"), // 指标采集时间间隔
		holmes.WithDumpPath(path),         // profile保存路径
		holmes.WithLogger(nil),
		holmes.WithCPUDump(15, 10, 30, 5*time.Second), // 配置CPU的性能监控规则
		holmes.WithCPUMax(50),
		holmes.WithMemDump(15, 25, 50, 5*time.Second),    // 配置Heap Memory 性能监控规则
		holmes.WithGCHeapDump(10, 20, 40, 2*time.Minute), // 配置基于GC周期的Heap Memory 性能监控规则
		holmes.WithCGroup(true),
		// holmes.WithGoroutineDump(100, 25, 200, 100*1000, 5*time.Minute), // 配置Goroutine数量的监控规则
	)

	h.Set(GlobalOptions...)
	h.Set(option...)
	// enable all
	h.EnableCPUDump().
		// EnableGoroutineDump().
		EnableMemDump().
		EnableGCHeapDump().Start()
}

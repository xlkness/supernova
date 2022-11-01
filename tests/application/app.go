package main

import (
	"fmt"
	"time"

	"joynova.com/library/supernova/pkg/jlog"
)

type BootYaml struct {
	Region     string `yaml:"region"`      // 对应配置表使用哪个区域
	RegionName string `yaml:"region_name"` // 区域名，例如th tw等
	Server     string `yaml:"server"`
	Etcd       struct {
		EtcdPathPrefix string   `yaml:"path_prefix"` // etcd存储路径前缀，用来做环境隔离
		Addrs          []string `yaml:"addrs"`       // etcd的地址
		HbInterval     int      `yaml:"hb_interval"` // etcd心跳的间隔（秒）
	} `yaml:"etcd"`
}

var bootYaml = new(BootYaml)

func main() {
	// app := application.New(
	// 	application.WithBootConfigFileContent(bootYaml),
	// )
	app := novaapp.DefaultApp()
	// app.ApplyOptions(application.WithBootFlags(bootYaml))
	app.AddInitializeTask("初始化配置表", func() error {
		fmt.Printf("初始化配置表成功\n")
		return nil
	})
	// app.AddService(new(service.ServicesManager))
	// app.AddServer(new(jweb.Engine))
	app.AddPostRunWorker("监听配置表重读", func() error {
		fmt.Printf("开始监听配置表重读\n")
		time.Sleep(time.Second * 10)
		return fmt.Errorf("监听出错")
	})
	app.AddPostRunTask("检查数据库dao", func() error {
		fmt.Printf("检查数据库ok\n")
		return nil
	})
	app.AddParallelJob("加载排行榜数据预热内存", func() {
		fmt.Printf("开始加载排行榜数据\n")
		time.Sleep(time.Second * 4)
		fmt.Printf("加载排行榜数据完成\n")
	})
	err := app.Run()
	if err != nil {
		jlog.Fatalf(err)
	}
}

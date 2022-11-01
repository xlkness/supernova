package novaapp

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
	"joynova.com/joynova/joymicro/joyservice"
	"joynova.com/library/supernova/pkg/jlog"
	"joynova.com/library/supernova/pkg/joyos"
	"joynova.com/library/supernova/pkg/jweb"
	"joynova.com/library/supernova/pkg/trace/holmes"
	"joynova.com/library/supernova/pkg/trace/prom"
	"joynova.com/library/supernova/pkg/utils"
	"joynova.com/library/supernova/pkg/utils/flags"
)

func DefaultApp() *Application {
	return defaultApp()
}

func New(options ...Option) *Application {
	app := defaultApp()
	app.ApplyOptions(options...)
	app.Initialize()
	return app
}

func defaultApp() *Application {
	app := new(Application)
	app.initOnce = new(sync.Once)
	app.bootFlags.appBootFlags = new(ApplicationCommBootFlags)
	app.concurrentLock = new(sync.Mutex)
	options := []Option{
		WithBootConfigFileParser(yaml.Unmarshal),
		WithLogFileTimestampFormat("20060102"),
		WithLogFileLevel(jlog.LogLevelTrace),
	}
	app.ApplyOptions(options...)
	return app
}

// Application 一个可运行的应用
// 集成：
//	 * 自定义启动参数
//   * 指定任意格式启动配置文件（默认yaml）
//	 * 集成调试日志jlog
//   * 注册启动服务前job任务
//	 * 注册rpc服务
//	 * 注册web服务
//   * 注册启动服务后job任务
//   * 注册启动服务后永久驻留的worker任务
type Application struct {
	initOnce  *sync.Once
	bootFlags struct {
		appBootFlags    *ApplicationCommBootFlags // 应用启动必须的公共参数
		customBootFlags []interface{}             // 用户自定义的启动参数
	}
	bootFile struct {
		bootConfigFileContent interface{} // 起服配置文件解析后的内容
		bootConfigFileParser  func(in []byte, out interface{}) error
	}
	log struct {
		logLevel        jlog.LogLevel
		logFileTsFormat string // 日志文件时间戳格式，默认"20060102"
	}
	initializeTasks []Task                        // 启动服务前串行执行初始化任务的job
	services        []*joyservice.ServicesManager // rpc服务
	servers         []*jweb.Engine                // web服务
	postRunTasks    []Task                        // 启动后串行执行的job
	postRunWorker   []Worker                      // 启动后后台永久执行的工作协程，一旦推出就停止application
	parallelJobs    []Job                         // 启动services、servers后并行执行的任务，不关心结果，例如内存数据的预热等

	// 测试模式
	debugIgnoreRunFlag bool
	concurrentLock     *sync.Mutex
}

// ApplyOptions 配置app，一定要在Run之前调用
func (a *Application) ApplyOptions(options ...Option) *Application {
	a.concurrentLock.Lock()
	defer a.concurrentLock.Unlock()
	for _, option := range options {
		option.Apply(a)
	}
	return a
}

// Initialize 初始化app，会根据options配置做各种初始化例如flags、config file、jlog等
func (a *Application) Initialize() {
	if a.debugIgnoreRunFlag {
		return
	}

	a.initOnce.Do(func() {
		// 解析起服参数
		allFlagGroups := append([]interface{}{a.bootFlags.appBootFlags}, a.bootFlags.customBootFlags...)
		flags.ParseWithStructPointers(allFlagGroups...)

		// 填充缺失字段
		globalID := a.bootFlags.appBootFlags.GlobalID
		serviceName := a.bootFlags.appBootFlags.AppName

		// 设置非空的global_id
		if globalID == "" {
			a.bootFlags.appBootFlags.GlobalID = utils.GetGlobalIDFromPodName(serviceName)
		}

		// 解析一下service_name
		if serviceName != "" {
			// 可能为pod name，解析-前面的deployment名字
			a.bootFlags.appBootFlags.AppName = strings.Split(serviceName, "-")[0]
		} else {
			// 否则就用二进制程序名字
			a.bootFlags.appBootFlags.AppName = filepath.Base(os.Args[0])
		}

		// 解析起服文件
		err := a.tryLoadBootConfigFile()
		if err != nil {
			panic(err)
		}

		// 初始化日志输出
		fd, err := jlog.NewRotatingDayMaxFileHandler(a.bootFlags.appBootFlags.LogDirPath, a.bootFlags.appBootFlags.AppName, 1<<30, 10)
		if err != nil {
			panic(fmt.Errorf("new log file error:%v", err))
		}
		jlog.NewGlobalLogger(fd, a.log.logLevel, func(l zerolog.Logger) zerolog.Logger {
			return l.With().
				Str("service", a.bootFlags.appBootFlags.AppName).
				Str("node_id", a.bootFlags.appBootFlags.GlobalID).
				Logger()
		}, a.bootFlags.appBootFlags.LogStdout)

		// 集成prometheus metrics、go pprof、holmes dump
		a.servers = append(a.servers, prom.NewEngine(":"+a.bootFlags.appBootFlags.TracePort, true))

		holmesPath := a.bootFlags.appBootFlags.LogDirPath
		if holmesPath != "" {
			if holmesPath[len(holmesPath)-1] != '/' {
				holmesPath += "/"
			}
		}
		holmes.StartTraceAndDump(holmesPath + "holmes/" + a.bootFlags.appBootFlags.AppName)

		// 输出initialize结果
		for _, group := range allFlagGroups {
			jlog.Infof("application run with command line args:%+v", group)
		}
		jlog.Infof("application run with config file content:%+v", a.bootFile.bootConfigFileContent)

		gitSha, _ := os.LookupEnv("gitsha")
		if gitSha == "" {
			gitSha = "not found in env[gitsha]"
		}
		jlog.Infof("application run with git sha:%v", gitSha)
	})
}

// AddInitializeTask Initialize之后Run之前串行执行初始化任务的job
func (a *Application) AddInitializeTask(desc string, job func() error) *Application {
	a.concurrentLock.Lock()
	defer a.concurrentLock.Unlock()
	a.initializeTasks = append(a.initializeTasks, func() error {
		jlog.Infof("start run initialize task %v", desc)
		err := job()
		if err != nil {
			jlog.Infof("end run initialize task %v with error %v", desc, err)
			return fmt.Errorf("execute initialize task %v error:%v", desc, err)
		}
		jlog.Infof("end run initialize task %v", desc)
		return nil
	})
	return a
}

func (a *Application) AddService(svc *joyservice.ServicesManager) *Application {
	a.concurrentLock.Lock()
	defer a.concurrentLock.Unlock()
	if svc == nil {
		return a
	}
	a.services = append(a.services, svc)
	return a
}

func (a *Application) AddServer(server *jweb.Engine) *Application {
	a.concurrentLock.Lock()
	defer a.concurrentLock.Unlock()
	if server == nil {
		return a
	}
	a.servers = append(a.servers, server)
	return a
}

// AddPostRunWorker Run所有服务之后，后台永久执行的协程任务
func (a *Application) AddPostRunWorker(desc string, worker Worker) *Application {
	a.concurrentLock.Lock()
	defer a.concurrentLock.Unlock()
	a.postRunWorker = append(a.postRunWorker, func() error {
		jlog.Infof("start run post worker %v", desc)
		err := worker()
		if err != nil {
			jlog.Infof("end run post worker %v with error %v", desc, err)
			return fmt.Errorf("execute post run worker %v exit with error:%v", desc, err)
		}
		jlog.Infof("end run post worker %v", desc)
		return nil
	})
	return a
}

// AddPostRunTask Run所有服务器之后，后台短暂执行的任务，报错就退出app，可用于Run之后的某些检查、初始化工作
func (a *Application) AddPostRunTask(desc string, job func() error) *Application {
	a.concurrentLock.Lock()
	defer a.concurrentLock.Unlock()
	a.postRunTasks = append(a.postRunTasks, func() error {
		jlog.Infof("start run post task %v", desc)
		err := job()
		if err != nil {
			jlog.Infof("end run post task %v with error %v", desc, err)
			return fmt.Errorf("execute post run task %v error:%v", desc, err)
		}
		jlog.Infof("end run post task %v", desc)
		return nil
	})
	return a
}

// AddParallelJob Run之后并行执行的不重要的任务，不关心报错，例如内存预热等
func (a *Application) AddParallelJob(desc string, job func()) *Application {
	a.concurrentLock.Lock()
	defer a.concurrentLock.Unlock()
	a.parallelJobs = append(a.parallelJobs, func() {
		defer jlog.CatchWithInfo(fmt.Sprintf("execute parallel job %v panic", desc))
		jlog.Infof("start run post job %v", desc)
		job()
		jlog.Infof("end run post job %v", desc)
	})
	return a
}

func (a *Application) GetAppBootFlags() *ApplicationCommBootFlags {
	return a.bootFlags.appBootFlags
}

func (a *Application) GetCustomBootFlags() []interface{} {
	return a.bootFlags.customBootFlags
}

func (a *Application) GetBootFileContent() interface{} {
	return a.bootFile.bootConfigFileContent
}

func (a *Application) Run() (err error) {
	waitChan := make(chan error, 1)

	defer func() {
		jlog.Noticef("application stop with error:%v", err)
	}()

	// 启动前的初始化任务
	for _, j := range a.initializeTasks {
		err = j()
		if err != nil {
			return
		}
	}

	// 启动rpc服务
	for _, s := range a.services {
		go func(s *joyservice.ServicesManager) {
			err := s.Run()
			if err != nil {
				waitChan <- fmt.Errorf("service %v error:%v", s.Addr, err)
			}
		}(s)
	}

	defer a.stopServices()

	// 启动web服务
	for _, s := range a.servers {
		go func(s *jweb.Engine) {
			err := s.Run()
			if err != nil {
				waitChan <- fmt.Errorf("server error:%v", err)
			}
		}(s)
	}

	defer a.stopServers()

	// 启动后串行执行的job
	for _, j := range a.postRunTasks {
		err = j()
		if err != nil {
			return
		}
	}

	// 启动后串行执行的工作协程
	for _, g := range a.postRunWorker {
		go func(g Worker) {
			err := g()
			if err != nil {
				waitChan <- err
			}
		}(g)
	}

	// 启动后的并行job
	for _, j := range a.parallelJobs {
		go j()
	}

	gracefulStopCtx, gracefulStopFun := context.WithCancel(context.Background())

	// 监听linux信号
	go joyos.WatchSignal(func(signal os.Signal) {
		jlog.Noticef("application receive signal %v", signal)
		gracefulStopFun()
	})

	jlog.Noticef("application running ok, start watch running information or os signal...")

	select {
	case <-gracefulStopCtx.Done():
		return nil
	case err = <-waitChan:
		jlog.Errorf("application stop with channel notify error:%v", err)
		return err
	}
}

func (a *Application) Stop() {
	defer jlog.CatchWithInfo("stop application panic")
	a.stopServers()
	a.stopServices()
	// 暂停几秒等待某些业务平滑处理完
	time.Sleep(time.Second * 5)
}

func (a *Application) GetDebugIgnoreFlag() bool {
	return a.debugIgnoreRunFlag
}

func (a *Application) DebugSetBootConf(bc interface{}, acf *ApplicationCommBootFlags, ccf ...interface{}) {
	a.bootFile.bootConfigFileContent = bc
	a.bootFlags.appBootFlags = acf
	a.bootFlags.customBootFlags = append(a.bootFlags.customBootFlags, ccf...)
}

func (a *Application) stopServers() {
	for _, s := range a.servers {
		s.Stop()
	}
}

func (a *Application) stopServices() {
	for _, s := range a.services {
		s.Stop()
	}
}

// tryLoadBootConfigFile 加载起服配置文件
func (a *Application) tryLoadBootConfigFile() error {
	if a.bootFile.bootConfigFileContent == nil {
		return nil
	}

	content, err := ioutil.ReadFile(a.bootFlags.appBootFlags.BootConfigFile)
	if err != nil {
		return fmt.Errorf("load boot config file %v error:%v", a.bootFlags.appBootFlags.BootConfigFile, err)
	}

	err = a.bootFile.bootConfigFileParser(content, a.bootFile.bootConfigFileContent)
	if err != nil {
		return fmt.Errorf("load boot config file %v ok, but parse content error:%v", a.bootFlags.appBootFlags.BootConfigFile, err)
	}

	return nil
}

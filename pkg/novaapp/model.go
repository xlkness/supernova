package novaapp

// Task 不会永久执行的任务，串行用于启动前初始化或者启动后初始化工作，返回error就停止application
type Task func() error

// Worker 永久执行的工作协程，一旦停止就退出application
type Worker func() error

// Job 不会永久执行的任务，且不关心执行结果，不关心执行顺序，例如内存预热等
type Job func()

type ApplicationCommBootFlags struct {
	GlobalID       string `env:"global_id" desc:"全局唯一id，为空会给随机字符串" default:""`
	AppName        string `env:"service_name" desc:"当前进程服务名，为空会用当前可执行文件名" default:""`
	BootConfigFile string `env:"boot_config_file" desc:"起服配置文件路径，例如：/dir/boot_config.yaml" default:""`
	TracePort      string `env:"trace_port" desc:"监控端口，包含prometheus、go pprof等" default:"7788"`
	LogDirPath     string `env:"log_dir" desc:"程序日志输出目录" default:"log"`
	LogStdout      bool   `env:"log_stdout" desc:"程序日志是否也输出到控制台" default:"false"`
}

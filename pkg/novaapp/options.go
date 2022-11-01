package novaapp

import (
	"joynova.com/library/supernova/pkg/jlog"
)

// WithCommonBootFlags 设置app的起服参数，flags必须为结构体指针！
// 只支持string/int/int64/bool四种字段类型，例如：
// type Flags struct {
// 	 F1 string `env:"id" desc:"boot id" value:"default value"`
//   F2 int `env:"num" desc:"number" value:"3"`
// }
func WithBootFlags(flags interface{}) Option {
	return optionFunction(func(app *Application) {
		app.bootFlags.customBootFlags = append(app.bootFlags.customBootFlags, flags)
	})
}

// WithBootConfigFileContent 设置启动配置文件的解析结构，不设置默认无起服配置，默认以yaml解析
func WithBootConfigFileContent(content interface{}) Option {
	return optionFunction(func(app *Application) {
		app.bootFile.bootConfigFileContent = content
	})
}

// WithBootConfigFileParser 设置起服文件解析函数，默认yaml格式
func WithBootConfigFileParser(f func(content []byte, out interface{}) error) Option {
	return optionFunction(func(app *Application) {
		app.bootFile.bootConfigFileParser = f
	})
}

// WithLogFileTimestampFormat 设置日志文件默认时间戳格式，默认"20060102"
func WithLogFileTimestampFormat(format string) Option {
	return optionFunction(func(app *Application) {
		app.log.logFileTsFormat = format
	})
}

func WithLogFileLevel(level jlog.LogLevel) Option {
	return optionFunction(func(app *Application) {
		app.log.logLevel = level
	})
}

func WithDebuugIgnoreFlag(flag bool) Option {
	return optionFunction(func(app *Application) {
		app.debugIgnoreRunFlag = flag
	})
}

type Option interface {
	Apply(app *Application)
}

type optionFunction func(app *Application)

func (of optionFunction) Apply(app *Application) {
	of(app)
}

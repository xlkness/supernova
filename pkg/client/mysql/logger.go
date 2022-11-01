package mysql

import (
	"fmt"

	"joynova.com/library/supernova/pkg/jlog"
	"xorm.io/xorm/log"
)

type Logger interface {
	ErrorWrite(wo *LogWriteOp, err error, aff int)

	Debug(v ...interface{})
	Debugf(format string, v ...interface{})
	Error(v ...interface{})
	Errorf(format string, v ...interface{})
	Info(v ...interface{})
	Infof(format string, v ...interface{})
	Warn(v ...interface{})
	Warnf(format string, v ...interface{})

	Level() log.LogLevel
	SetLevel(l log.LogLevel)

	ShowSQL(show ...bool)
	IsShowSQL() bool
}

var logger Logger = &stdLogger{}

type stdLogger struct {
	level      log.LogLevel
	notShowSql bool
}

func (sl *stdLogger) ErrorWrite(wo *LogWriteOp, err error, aff int) {
	if err != nil {
		fmt.Print(wo, err, aff)
		return
	}

	fmt.Print(wo, fmt.Errorf("exec %v affected not equal:%v/%v", wo.String(), aff, wo.AffectedRows), aff)
}

func (sl *stdLogger) Debug(v ...interface{}) {
	sl.output("debug", v...)
}
func (sl *stdLogger) Debugf(format string, v ...interface{}) {
	sl.output("debug", v...)
}
func (sl *stdLogger) Error(v ...interface{}) {
	sl.output("error", v...)
}
func (sl *stdLogger) Errorf(format string, v ...interface{}) {
	sl.outputf("error", format, v...)
}
func (sl *stdLogger) Info(v ...interface{}) {
	sl.output("info", v...)
}
func (sl *stdLogger) Infof(format string, v ...interface{}) {
	sl.outputf("info", format, v...)
}
func (sl *stdLogger) Warn(v ...interface{}) {
	sl.output("warn", v...)
}
func (sl *stdLogger) Warnf(format string, v ...interface{}) {
	sl.outputf("warn", format, v...)
}
func (sl *stdLogger) Level() log.LogLevel {
	return sl.level
}
func (sl *stdLogger) SetLevel(l log.LogLevel) {
	sl.level = l
}
func (l *stdLogger) ShowSQL(show ...bool) {
	l.notShowSql = !show[0]
}

func (l *stdLogger) IsShowSQL() bool {
	if jlog.GetLogLevel() == jlog.LogLevelTrace {
		if l.notShowSql {
			return false
		}
		return true
	}
	return false
}

func (sl *stdLogger) output(level string, v ...interface{}) {
	fmt.Print(fmt.Sprintf("[%v]%v\n", level, fmt.Sprint(v...)))
}

func (sl *stdLogger) outputf(level, format string, v ...interface{}) {
	fmt.Printf("[%v]"+format, level, fmt.Sprint(v...))
}

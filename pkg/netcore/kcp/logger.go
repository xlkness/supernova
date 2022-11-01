package kcp

type Logger interface {
	Debugf(v ...interface{})
	Infof(v ...interface{})
	Warnf(v ...interface{})
	Errorf(v ...interface{})
	Critif(v ...interface{})
	Fatalf(v ...interface{})
}

var log Logger = &defaultLogger{}

func SetLogger(l Logger) {
	log = l
}

type defaultLogger struct {
}

func (*defaultLogger) Debugf(v ...interface{}) {

}
func (*defaultLogger) Infof(v ...interface{}) {

}
func (*defaultLogger) Warnf(v ...interface{}) {

}
func (*defaultLogger) Errorf(v ...interface{}) {

}
func (*defaultLogger) Critif(v ...interface{}) {

}
func (*defaultLogger) Fatalf(v ...interface{}) {

}

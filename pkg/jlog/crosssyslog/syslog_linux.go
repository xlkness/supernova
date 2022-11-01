// +build linux

package crosssyslog

import "log/syslog"

type Syslog struct {
	*syslog.Writer
}

type Priority = syslog.Priority

var (
	LOG_INFO   = syslog.LOG_INFO
	LOG_LOCAL0 = syslog.LOG_LOCAL0
)

func New(priority Priority, tag string) (*Syslog, error) {
	SyslogLocal, syslogErr := syslog.New(priority, tag)
	if syslogErr != nil {
		return nil, syslogErr
	}
	return &Syslog{
		Writer: SyslogLocal,
	}, nil
}

func (s *Syslog) Write(data []byte) (int, error) {
	return s.Writer.Write(data)
}

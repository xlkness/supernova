// +build windows

package crosssyslog

type Syslog struct {
}

type Priority int

var (
	LOG_INFO   Priority = 1
	LOG_LOCAL0 Priority = 2
)

func New(priority Priority, tag string) (*Syslog, error) {
	return &Syslog{}, nil
}

func (s *Syslog) Write(data []byte) (int, error) {
	return len(data), nil
}

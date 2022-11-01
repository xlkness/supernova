package main

import (
	"fmt"
	"time"

	"joynova.com/library/supernova/pkg/netcore/kcp"
)

// kcp库日志设置
type KcpLogger struct {
}

func (*KcpLogger) Debugf(v ...interface{}) {
	fmt.Printf("debug==>%v\n", preHandleLogArgs(v...))
}
func (*KcpLogger) Infof(v ...interface{}) {
	fmt.Printf("info ==>%v\n", preHandleLogArgs(v...))
}
func (*KcpLogger) Warnf(v ...interface{}) {
	fmt.Printf("warn ==>%v\n", preHandleLogArgs(v...))
}
func (*KcpLogger) Errorf(v ...interface{}) {
	fmt.Printf("error==>%v\n", preHandleLogArgs(v...))
}
func (*KcpLogger) Critif(v ...interface{}) {
	fmt.Printf("criti==>%v\n", preHandleLogArgs(v...))
}
func (*KcpLogger) Fatalf(v ...interface{}) {
	fmt.Printf("fatal==>%v\n", preHandleLogArgs(v...))
}

func preHandleLogArgs(v ...interface{}) string {
	rpcModName := "[kcp mod]"
	if len(v) == 1 {
		return fmt.Sprintf("%s%v", rpcModName, v[0])
	} else if len(v) > 1 {
		format, ok := v[0].(string)
		if !ok {
			return fmt.Sprintf("%s%v", rpcModName, fmt.Sprint(v...))
		}
		return fmt.Sprintf(rpcModName+format, v[1:]...)
	} else {
		return rpcModName
	}
}

func main() {
	// 设置 因为参数数量有点多 所以全部使用变量而非函数 并且已经给了一个相对较好的默认值

	kcp.Pack_max_len = 256
	kcp.Kcp_mtu = 252
	kcp.Sess_ch_socket_size = 64
	kcp.Sess_ch_logic_size = 16
	kcp.SetLogger(&KcpLogger{})
	lis, err := kcp.Listen("0.0.0.0:5678", 10, 10)
	if err != nil {
		fmt.Println(err)
	}

	time.Sleep(time.Second * 2)

	fmt.Println("server started")

	buf := []byte("send")
	for i := 1; i < 1000; i++ {
		conv := uint32(i)
		s, err := lis.AddSession(conv)
		if err != nil {
			fmt.Println(err)
		}

		go func(s *kcp.Session) {
			<-s.ChLogic
			ticker := time.NewTicker(time.Millisecond * 33)
			for {
				select {
				case <-s.ChLogic:
				case <-ticker.C:
					s.Send(buf)
				}
			}

		}(s)
	}

	for {
		time.Sleep(time.Minute)
	}

	fmt.Println("udp listener closed")
}

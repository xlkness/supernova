package main

import (
	"flag"
	"fmt"
	"sync/atomic"
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

var rc = new(int32)

func main() {

	n := flag.Int("n", 100, "num")
	flag.Parse()

	// 设置
	kcp.Pack_max_len = 256
	kcp.Kcp_mtu = 252
	kcp.Sess_ch_socket_size = 64
	kcp.Sess_ch_logic_size = 32
	kcp.SetLogger(&KcpLogger{})
	die := make(chan bool, 1)

	buf := []byte("input")
	for i := 1; i <= *n; i++ {
		go func(conv uint32) {
			client, err := kcp.NewClient(conv, "192.168.1.18:5678")
			if err != nil {
				fmt.Println(err)
			}

			index := 0
			timer := time.NewTicker(time.Millisecond * 500)
			for {
				select {
				case err := <-client.Err:
					fmt.Println("udp client error :" + err.Error())
				case <-timer.C:
					index++
					client.Send(buf)
				case <-client.ChLogic:
					currc := atomic.AddInt32(rc, 1)
					if currc%1000 == 0 {
						fmt.Printf("cur recv:%v\n", atomic.LoadInt32(rc))
					}
				}
			}
			fmt.Println("client closed")
		}(uint32(i))
	}

	for {
		select {
		case <-die:
			break
		}
	}
}

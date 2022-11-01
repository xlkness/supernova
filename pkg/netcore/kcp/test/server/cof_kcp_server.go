package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"joynova.com/library/supernova/pkg/netcore/kcp"
)

// kcp库日志设置
type KcpLogger1 struct {
}

func (*KcpLogger1) Debugf(v ...interface{}) {
	fmt.Printf("debug==>%v\n", preHandleLogArgs1(v...))
}
func (*KcpLogger1) Infof(v ...interface{}) {
	fmt.Printf("info ==>%v\n", preHandleLogArgs1(v...))
}
func (*KcpLogger1) Warnf(v ...interface{}) {
	fmt.Printf("warn ==>%v\n", preHandleLogArgs1(v...))
}
func (*KcpLogger1) Errorf(v ...interface{}) {
	fmt.Printf("error==>%v\n", preHandleLogArgs1(v...))
}
func (*KcpLogger1) Critif(v ...interface{}) {
	fmt.Printf("criti==>%v\n", preHandleLogArgs1(v...))
}
func (*KcpLogger1) Fatalf(v ...interface{}) {
	fmt.Printf("fatal==>%v\n", preHandleLogArgs1(v...))
}

func preHandleLogArgs1(v ...interface{}) string {
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

var globallock = sync.Mutex{}
var uid = 0
var sessionnum = 1000
var listenaddr = "0.0.0.0:5678"
var calcdata = make([]map[string]interface{}, 0, 100)

func getuid() int {
	globallock.Lock()
	defer globallock.Unlock()
	uid++
	if uid > sessionnum {
		uid = 1
	}
	return uid
}

func savecalcdata(data map[string]interface{}) {
	globallock.Lock()
	defer globallock.Unlock()
	if len(calcdata) > 100 {
		calcdata = calcdata[:100]
	}
	calcdata = append([]map[string]interface{}{data}, calcdata...)
}

func getcalcdata(latestlen int) []map[string]interface{} {
	globallock.Lock()
	globallock.Unlock()
	if latestlen <= 0 {
		return nil
	}
	if len(calcdata) <= 0 {
		return nil
	}
	var retdata []map[string]interface{}
	if len(calcdata) < latestlen {
		retdata = make([]map[string]interface{}, len(calcdata))
		copy(retdata, calcdata)
	} else {
		retdata = make([]map[string]interface{}, latestlen)
		copy(retdata, calcdata[0:latestlen])
	}

	return retdata
}

var buildts = "no timestamp set"
var githash = "no githash set"

func main() {
	fmt.Println("%v build timestamp is:", "cof_kcp_server", buildts)
	fmt.Println("%v build githash is:", "cof_kcp_server", githash)

	port := flag.String("p", "5678", "listen port")
	flag.Parse()

	// kcp
	// kcp.Pack_max_len = 256
	// kcp.Kcp_mtu = 252
	kcp.Sess_ch_socket_size = 64
	kcp.Sess_ch_logic_size = 16
	kcp.SetLogger(&KcpLogger1{})
	lis, err := kcp.Listen("0.0.0.0:"+*port, 1, 10)
	if err != nil {
		fmt.Println(err)
	}

	// http
	ginEngine := gin.Default()
	ginEngine.GET("/start", func(c *gin.Context) {
		// frameStr := c.Query("count")
		// frame, _ := strconv.Atoi(frameStr)
		// if frame <= 0 {
		//	c.String(200, fmt.Sprintf("invalid frame:%v", frameStr))
		//	return
		// }

		uid := uint32(getuid())
		fmt.Printf("receive start, uid:%v\n", uid)
		c.String(200, strconv.Itoa(int(uid)))

		go handleClient(lis, uid)
	})
	ginEngine.POST("/stop", func(c *gin.Context) {

		bindata, err := c.GetRawData()
		if err != nil {
			fmt.Printf("receive raw data error:%v\n", err)
			c.String(200, fmt.Sprintf("receive raw data error:%v", err))
			return
		}
		data := make(map[string]interface{})
		err = json.Unmarshal(bindata, &data)
		if err != nil {
			fmt.Printf("unmarshal raw data error:%v\n", err)
			c.String(200, fmt.Sprintf("unmarshal raw data error:%v", err))
			return
		}

		uid := data["conv"].(float64)

		fmt.Printf("receive stop, uid:%v, data len:%v\n", uid, len(bindata))

		savecalcdata(data)

		c.String(200, strconv.Itoa(int(uid)))
	})
	ginEngine.GET("/get", func(c *gin.Context) {
		length := c.Query("latest")
		l, _ := strconv.Atoi(length)
		if l <= 0 {
			c.String(200, fmt.Sprintf("input valid number:%v", l))
			return
		}
		data := getcalcdata(l)
		jsondata := map[string]interface{}{
			"data": data,
		}
		bindata, err := json.Marshal(&jsondata)
		if err != nil {
			fmt.Printf("get, json unmarshal error:%v", err)
			c.String(200, fmt.Sprintf("get, json unmarshal error:%v", err))
			return
		}
		c.String(200, string(bindata))
	})
	go ginEngine.Run("0.0.0.0:" + *port)

	time.Sleep(time.Second * 2)

	fmt.Println("server started")

	select {}
}

func handleClient(lis *kcp.Listener, uid uint32) {
	s, err := lis.AddSession(uid)
	defer lis.RemoveSession(uid)
	if err != nil {
		fmt.Println(err)
	}

	<-s.ChLogic

	ticker := time.NewTicker(time.Nanosecond * 33500000)
	defer ticker.Stop()

	count := 0

OUT:
	for {
		select {
		case buf := <-s.ChLogic:
			if string(buf) == "stop" {
				time.Sleep(time.Second * 2)
				fmt.Printf("session %v over, wait next connection.\n", uid)
				break OUT
			}
		case <-ticker.C:
			count++
			c := make([]byte, 4)
			binary.LittleEndian.PutUint32(c, uint32(count))
			s.Send(c)
		}
	}

}

package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

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

type count struct {
	mt60  int
	mt100 int
	mt200 int
	lt2   int
}

func main() {
	addr := flag.String("a", "192.168.1.188:8888", "address, eg:192.168.1.1:8888")
	frame := flag.Int("f", 1000, "test frame num and stop")
	flag.Parse()

	kcp.SetLogger(&KcpLogger1{})

	// 设置
	kcp.Sess_ch_socket_size = 64
	kcp.Sess_ch_logic_size = 16

	url := "http://" + *addr + "/start"
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		fmt.Printf("http get %v error:%v\n", url, err)
		return
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("http read %v resp error:%v\n", url, err)
		return
	}
	id, err := strconv.Atoi(string(respBytes))
	if err != nil {
		fmt.Printf("get /start return not integer:%v", string(respBytes))
		return
	}

	client, err := kcp.NewClient(uint32(id), *addr)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("connect server ok, session id:%v, start..\n", id)
	client.Send([]byte("hello"))

	c := count{}
	pret := time.Now()

	fileName := fmt.Sprintf("cap_%v%.2d%.2d%.2d%.2d%.2d%d.log",
		pret.Year(), int(pret.Month()), pret.Day(),
		pret.Hour(), pret.Minute(), pret.Second(), int(pret.Nanosecond()/1000/1000))
	fd, err := os.Create(fileName)
	if err != nil {
		panic(err)
	}
	fmt.Printf("create log file ok.\n")
	defer fd.Close()

OUT:
	for {
		select {
		case err := <-client.Err:
			fmt.Println("udp client error :" + err.Error())
		case buf := <-client.ChLogic:
			content := binary.LittleEndian.Uint32(buf)
			timeNow := time.Now()
			data := fmt.Sprintf("%.7d", uint32(content)) + "," + strconv.FormatInt(int64(timeNow.UnixNano()/1000/1000)-1578430506736, 10) + "\n"
			data1 := []byte(data)
			n, err := fd.Write(data1)
			if err != nil {
				fmt.Printf("data [%v] write to file error:%v\n", data, err)
			} else if n != len(data1) {
				fmt.Printf("data [%v] write to file return num ne:%v/%v\n", data, n, len(data1))
			}

			timeNow.Sub(pret)
			lag := int(timeNow.Sub(pret).Nanoseconds() / 1000 / 1000)

			if lag > 60 {
				c.mt60++
			}
			if lag > 100 {
				c.mt100++
			}
			if lag > 200 {
				c.mt200++
			}
			if lag < 2 {
				c.lt2++
				// fmt.Printf("%v index lag less than %vms\n", index, lag)
			}

			if int(content) > *frame {
				fmt.Printf("recv over.\n")
				client.Send([]byte("stop"))
				time.Sleep(time.Second)
				break OUT
			}
			pret = timeNow

			// fmt.Printf("cur index1:%v\n", uint32(content))
		}
	}
	//
	// param := map[string]interface{}{
	//	"conv": id,
	// }
	// binParam, _ := json.Marshal(&param)
	// body := bytes.NewReader(binParam)
	// url1 := "http://" + *addr + "/stop"
	// request, err := http.NewRequest("POST", url1, body)
	// if err != nil {
	//	fmt.Printf("new http client stop remote %v session error:%v\n", url1, err)
	//	return
	// }
	// httpClient1 := http.Client{
	//	Timeout: time.Second * 10,
	// }
	// respData, err := httpClient1.Do(request)
	// if err != nil {
	//	fmt.Printf("http client stop remote %v session error:%v\n", url1, err)
	//	return
	// }
	//
	// resp1, err := ioutil.ReadAll(respData.Body)
	// if err != nil {
	//	fmt.Printf("http client stop remote %v session read resp error:%v\n", url1, err)
	//	return
	// }

	fmt.Printf("total %v frame, lag mt60:%v, mt100:%v, mt200:%v, lt2:%v, details output file:%v\n",
		*frame, c.mt60, c.mt100, c.mt200, c.lt2, fileName)

	fmt.Printf("test over.\n")
}

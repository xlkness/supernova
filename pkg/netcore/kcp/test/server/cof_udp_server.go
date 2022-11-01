package main

import (
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

func main() {
	port := flag.String("p", "8888", "port")
	flag.Parse()

	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:"+*port)
	if err != nil {
		panic(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}

	data := make([]byte, 1024)
	c := make(chan bool, 1)
	for {
		n, caddr, err := conn.ReadFromUDP(data)
		if err != nil {
			continue
		}
		fmt.Printf("server recv:%v\n", string(data[:n]))
		strs := strings.Split(string(data[:n]), ":")
		if strs[0] == "start" {
			packetNum, err := strconv.Atoi(string(strs[1]))
			if err != nil {
				panic(err)
			}
			if packetNum > 0 && packetNum < 5 {
				go handleclient(conn, caddr, packetNum, c)
			} else {
				fmt.Printf("server recv invalid packet num:%v\n", packetNum)
			}
		} else if strs[0] == "stop" {
			select {
			case c <- true:
			default:
			}
		}
	}
}

func handleclient(conn *net.UDPConn, caddr *net.UDPAddr, packetNum int, c chan bool) {
	senddata := []byte("hello")
	ticker := time.NewTicker(time.Millisecond * 33)
	defer ticker.Stop()
	count := 0
	for {
		select {
		case <-ticker.C:

			for i := 0; i < packetNum; i++ {
				senddata = []byte(fmt.Sprintf("%v", count))
				count++
				n, err := conn.WriteToUDP(senddata, caddr)
				if err != nil {
					fmt.Printf("handle client error:%v\n", err)
					return
				}
				if n != len(senddata) {
					fmt.Printf("send return not equal:%v/%v\n", n, len(senddata))
				}
			}
		case <-c:
			fmt.Printf("stop session.\n")
			return
		}
	}
}

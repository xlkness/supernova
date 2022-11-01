package main

import (
	"flag"
	"fmt"
	"net"
	"strconv"
	"time"
)

type statistic struct {
	mt60  int
	mt100 int
	mt200 int
	lt2   int
}

type tmpsave struct {
	idx int
	t   time.Time
}

func main() {
	addr := flag.String("a", "no address", "address")
	packetNum := flag.Int("p", 1, "packet num")
	frameNum := flag.Int("f", 5000, "frame num")
	flag.Parse()

	conn, err := net.Dial("udp", *addr)
	if err != nil {
		panic(err)
	}

	fmt.Printf("remote address:%v, start....\n", *addr)

	_, err = conn.Write([]byte(fmt.Sprintf("start:%d", *packetNum)))
	if err != nil {
		panic(err)
	}

	buf := make([]byte, 1024)
	pret := time.Now()
	st := make([]*tmpsave, *frameNum**packetNum)
	total := *frameNum * *packetNum
	recvcount := 0
	for {
		n, err := conn.Read(buf)
		if err != nil {
			panic(err)
		}
		recvcount++
		idx, err := strconv.Atoi(string(buf[:n]))
		if err != nil {
			panic(err)
		}
		if idx >= 0 && idx < total {
			st[idx] = &tmpsave{
				idx: idx,
				t:   time.Now(),
			}
		} else if idx >= total {
			break
		}
	}

	conn.Write([]byte("stop"))
	time.Sleep(time.Second)

	loseNum := 0
	for _, v := range st {
		if v == nil {
			loseNum++
		}
	}
	group := 1
	pregroup := group
	st1 := statistic{}

	for i, v := range st {
		idx := i + 1
		if idx%*packetNum == 1 {
			group++
		}

		if pregroup != group && v != nil {
			lag := int(v.t.Sub(pret).Nanoseconds() / 1000 / 1000)
			if lag > 60 {
				st1.mt60++
			} else if lag > 100 {
				st1.mt100++
			} else if lag > 200 {
				st1.mt200++
			} else if lag < 2 {
				st1.lt2++
			}
		}

		if v != nil {
			pret = v.t
			pregroup = group
		}
	}

	fmt.Printf("total %v frame, %v packet per 33ms, packet loss:%v(real recv:%v), %.2f%%, lag mt60:%v, mt100:%v, mt200:%v, lt2:%v\n",
		*frameNum, *packetNum, loseNum, recvcount, float32(loseNum)/float32(*frameNum**packetNum)*100, st1.mt60, st1.mt100, st1.mt200, st1.lt2)
}

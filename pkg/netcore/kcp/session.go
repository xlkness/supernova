package kcp

import (
	"fmt"
	"net"
	"runtime"
	"time"

	"golang.org/x/net/ipv4"
)

var (
	Pack_max_len int = 1024 // 单个udp包的最大包长，用于做缓存（ 必须 >= kcp_mtu + fec_head_Len）

	Fec_len      int    = 6   // fec 冗余册数 3 => p1(x1, x2, x3)
	Fec_cacheLen uint32 = 256 // fec 序号收到的记录长度 需要大于 kcp的wnd_size

	Kcp_nodelay  int           = 1   // kcp的延迟发送
	Kcp_interval int           = 1   // kcp的刷新间隔
	Kcp_resend   int           = 2   // kcP的重传次数
	Kcp_nc       int           = 1   // kcp的流控
	Kcp_mtu      int           = 256 // kcp输出的单个包的长度
	Kcp_wnd_size int           = 32  // kcp的窗口的大小
	Kcp_update   time.Duration = 10  // kcp的update调用时间间隔

	Sess_ch_socket_size int = 64 // 会话接收socket数据的通道大小
	Sess_ch_send_size   int = 64 // 会话发送数据的通道大小
	Sess_ch_logic_size  int = 32 // 会话投递数据给logic的通道大小
)

// 数据流
// send2kcp: kcp=>fec=>socket
// recv2fec: socket=>fec=>kcp
// 缓存池流
// recv: pop=>chSocket=>fec=>kcp=>push
// seg: pop=>new=>delete=>push
// unpack: pop=>unpack=>logic=>push
type Session struct {
	conv     uint32      // 会话id
	kcp      *KCP        // kcp
	encoder  *fecEncoder // fec
	decoder  *fecDecoder // fec
	closed   bool        // 关闭
	chClosed chan bool   // 关闭通道 用于关闭协程

	conn net.PacketConn // socket

	remote_addr net.Addr    // 远端地址,客户端的地址是可以随时变动的 因为wifi/4g等情况很常见
	chSocket    chan []byte // 接收socket数据的通道
	chSend      chan []byte // 发送消息的通道
	//chTimer     chan struct{} // 计时器通道
	curTime uint32 // 当前计时

	ChLogic chan []byte // 抛出数据给逻辑层的通道

	updaterIdx int // 时间堆的更新索引

	xconn    batchConn      // 批写连接
	xqueue   []ipv4.Message // 批写队列
	isServer bool           // 屏蔽remote_addr为空时不走作为客户端压测的逻辑
}

func newSession(conv uint32, conn net.PacketConn) *Session {
	s := new(Session)
	s.conv = conv
	s.conn = conn
	s.closed = false
	s.chClosed = make(chan bool)
	// fec
	s.encoder = newFECEncoder(Fec_len, Pack_max_len)
	s.decoder = newFECDecoder(Fec_cacheLen)
	// kcp
	s.kcp = NewKCP(conv, s.writeToFecToSocket)
	s.kcp.NoDelay(Kcp_nodelay, Kcp_interval, Kcp_resend, Kcp_nc)
	s.kcp.SetMtu(Kcp_mtu)
	s.kcp.WndSize(Kcp_wnd_size, Kcp_wnd_size)

	// 从网络层接收数据
	s.chSocket = make(chan []byte, Sess_ch_socket_size)
	s.chSend = make(chan []byte, Sess_ch_send_size)
	// 抛数据给逻辑层
	s.ChLogic = make(chan []byte, Sess_ch_logic_size)
	//s.chTimer = make(chan struct{}, 16)
	// 批写
	s.xconn = ipv4.NewPacketConn(conn)
	s.xqueue = make([]ipv4.Message, 0, 32)

	//s.updaterIdx = -1
	//updater.addSession(s)

	s.isServer = true

	go s.run()

	return s
}

// Send data to kcp
func (s *Session) Send(b []byte) {
	select {
	case s.chSend <- b:
	default:
		log.Errorf("kcp session[%v] send channel full, send queue len:%v",
			s.GetConv(), len(s.kcp.snd_queue))
	}
}

// 监听收取消息
func (s *Session) run() {
	defer func() {
		if v := recover(); v != nil {
			log.Warnf("[kcp catch panic info in session]session[%v][%v]", s.conv, s.GetRemoteIp())
			log.Warnf("[kcp catch panic info in session]session[%v][%s] (most recent call last):\n", s.conv, time.Now())
			for i := 0; ; i++ {
				pc, file, line, ok := runtime.Caller(i + 1)
				if !ok {
					break
				}
				//fmt.Fprintf(os.Stderr, "% 3d. %s() %s:%d\n", i, runtime.FuncForPC(pc).Name(), file, line)
				log.Warnf("[kcp catch panic info in session]session[%v]% 3d. %s() %s:%d\n",
					s.conv, i, runtime.FuncForPC(pc).Name(), file, line)
			}
			//fmt.Fprintf(os.Stderr, "%v\n", message)
			log.Warnf("[kcp catch panic info in session]session[%v]%v\n", s.conv, v)
		}
	}()
	for {
		select {
		case <-s.chClosed:
			return
		case buf := <-s.chSend:
			s.kcp.Send(buf)
			s.kcp.Update(s.curTime)
			s.writeBatch()
			s.curTime += 33
		case data := <-s.chSocket:
			s.handleRecvPacket(data)
			//case <-s.chTimer:
			//	s.kcp.Update(atomic.LoadUint32(&s.curTime))
		}
	}
}

func (s *Session) Update(cur uint32) {
	//atomic.StoreUint32(&s.curTime, cur)
	//select {
	//case s.chTimer <- struct{}{}:
	//default:
	//}
}

// GetConv get conv id
func (s *Session) GetConv() uint32 {
	return s.conv
}

func (s *Session) GetRemoteIp() string {
	if s.remote_addr != nil {
		return s.remote_addr.String()
	}
	return fmt.Sprintf("session[%v] remote address is null", s.conv)
}

// send data to socket
func (s *Session) writeToFecToSocket(buf []byte, size int) {
	if s.closed {
		return
	}

	var msg ipv4.Message
	ecc := s.encoder.encode(buf[:size])
	for _, b := range ecc {
		if len(b) == 0 {
			break
		}

		if len(b) > Pack_max_len {
			log.Warnf("[kcp session send msg]session[%v][%v] packet length limit:%v",
				s.conv, s.conv, s.GetRemoteIp(), len(b))
			break
		}

		if s.remote_addr == nil {
			if !s.isServer {
				// 用于kcp客户端，压测
				if c, ok := s.conn.(*net.UDPConn); ok {
					_, err := c.Write(b)
					if err != nil {
						log.Warnf("[kcp session write msg]session[%v] remote address error:%v", s.conv, err)
					}
				}
			}
		} else {
			msg.Addr = s.remote_addr
			msg.Buffers = [][]byte{b}
			s.xqueue = append(s.xqueue, msg)
		}
	}
}

func (s *Session) writeBatch() {
	for len(s.xqueue) > 0 {
		if n, err := s.xconn.WriteBatch(s.xqueue, 0); err == nil {
			s.xqueue = s.xqueue[n:]
		} else {
			log.Warnf("[kcp session write msg]session[%v][%v] batch write error:%v",
				s.conv, s.GetRemoteIp(), err)
			break
		}
	}
	s.xqueue = s.xqueue[:0]
}

func (s *Session) handleRecvPacket(data []byte) {
	f := s.decoder.decodeBytes(data)
	if f != nil {
		err := s.kcp.Input(f, true, false)
		if err != 0 {
			log.Warnf("[kcp session handle income data]session[%v][%v] data error:%v",
				s.conv, s.GetRemoteIp(), err)
		} else {
		Label:
			for {
				if size := s.kcp.PeekSize(); size > 0 {
					buf := make([]byte, Pack_max_len)
					length := s.kcp.Recv(buf)
					select {
					case s.ChLogic <- buf[:length]:
					default:
						log.Warnf("[kcp session[%v] handle income data][%v][%v] deliver to logic channel full",
							s.kcp.conv, s.kcp.conv, s.GetRemoteIp())
						break Label
					}
				} else {
					break
				}
			}
		}
	}
}

// 关闭会话
func (s *Session) close() {
	s.closed = true
	select {
	case s.chClosed <- true:
	default:
	}

	//updater.removeSession(s)
}

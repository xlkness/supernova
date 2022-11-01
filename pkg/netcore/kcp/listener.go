package kcp

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/libp2p/go-reuseport"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type inPacket struct {
	addr net.Addr
	buf  []byte
}

type readConn struct {
	// 初始化套接字函数
	reconnFun func(string) (net.PacketConn, error)
	// 套接字
	net.PacketConn
}

type Listener struct {
	addr      string
	readConns []*readConn
	sessions  sync.Map // uint32 - *Session // 会话列表
	closed    bool
	cancelFun context.CancelFunc
	rander    *rand.Rand
}

// Listen 监听地址的udp包
// addr: 监听地址
// acceptorNum: 监听协程数（大于1表示启用reuseport）
// workerPoolNumPerReadConn: 每个监听协程配套的工作池数量
func Listen(addr string, acceptorNum, workerPoolNumPerReadConn int) (*Listener, error) {
	l := new(Listener)
	l.rander = rand.New(rand.NewSource(time.Now().UnixNano()))

	if acceptorNum > 1 {
		conns, err := reusePortConns(addr, acceptorNum)
		if err != nil {
			return nil, err
		}
		l.readConns = append(l.readConns, conns...)
	} else {
		conn, err := genericPortConn(addr)
		if err != nil {
			return nil, err
		}
		l.readConns = append(l.readConns, conn)
	}

	// 关闭监听时关闭所有协程资源
	ctx, cancelFun := context.WithCancel(context.Background())
	l.cancelFun = cancelFun

	l.startWithWokerPool(ctx, workerPoolNumPerReadConn)
	return l, nil
}

func (l *Listener) GetSessionNum() int {
	count := 0
	l.sessions.Range(func(k, v interface{}) bool {
		count++
		return true
	})
	return count
}

// 添加会话
func (l *Listener) AddSession(conv uint32) (*Session, error) {
	_, ok := l.sessions.Load(conv)
	if ok {

		return nil, fmt.Errorf("reduplicate session")
	}

	// 随机分配一个套接字来发送消息
	conn := l.readConns[l.rander.Intn(len(l.readConns))]

	s := newSession(conv, conn.PacketConn)
	l.sessions.Store(conv, s)
	return s, nil
}

// 移除会话
func (l *Listener) RemoveSession(conv uint32) {
	s, ok := l.sessions.Load(conv)
	if !ok {
		log.Debugf("[kcp remove session][%v] not found session", conv)
		return
	}

	s.(*Session).close()
	l.sessions.Delete(conv)
}

func (l *Listener) Close() {
	l.closed = true

	l.sessions.Range(func(key, value interface{}) bool {
		value.(*Session).close()
		l.sessions.Delete(key.(uint32))

		return true
	})

	for _, v := range l.readConns {
		v.Close()
	}
	l.cancelFun()
}

// startWithWokerPool 启动监听
func (l *Listener) startWithWokerPool(ctx context.Context, workerPoolNumPerReadConn int) {
	for no, c := range l.readConns {
		// 每一个读套接字创建一个读协程和一个工作池来负载任务
		deliverTaskChan := make(chan []*inPacket, 1<<12)
		// 创建工作池
		for i := 0; i < workerPoolNumPerReadConn; i++ {
			go func() {
				for {
					select {
					case packets, ok := <-deliverTaskChan:
						if !ok {
							return
						}
						l.handleInComePakcets(packets)
					case <-ctx.Done():
						return
					}
				}
			}()
		}

		// 创建读协程
		go l.readInComePacket(ctx, no, c, deliverTaskChan)
	}

}

type batchConn interface {
	WriteBatch(ms []ipv4.Message, flags int) (int, error)
	ReadBatch(ms []ipv4.Message, flags int) (int, error)
	Close() error
}

// readInComePacket 阻塞读套接字udp包
func (l *Listener) readInComePacket(ctx context.Context, no int, readConn *readConn,
	deliverTaskChan chan []*inPacket) {
	var xconn batchConn

	for {
		if l.closed {
			return
		}

		// 创建读套接字
		conn, err := readConn.reconnFun(l.addr)
		if err != nil {
			// 读失败继续尝试
			log.Errorf("[kcp new read conn] %v read conn new error:%v, retry", no, err)
		} else {
			readConn.PacketConn = conn
			if _, ok := readConn.PacketConn.(*net.UDPConn); ok {
				addr, err := net.ResolveUDPAddr("udp", readConn.PacketConn.LocalAddr().String())
				if err == nil {
					if addr.IP.To4() != nil {
						xconn = ipv4.NewPacketConn(readConn.PacketConn)
					} else {
						xconn = ipv6.NewPacketConn(readConn.PacketConn)
					}
				}
			} else {
				panic(fmt.Errorf("kcp conn is not net.udpconn"))
			}

			msgs := make([]ipv4.Message, 20)
			for k := range msgs {
				msgs[k].Buffers = [][]byte{make([]byte, Pack_max_len)}
			}

			// 开始读循环
			for {
				if l.closed {
					xconn.Close()
					return
				}

				// read batch虽然传入了buffer池，但是底层还是复用buffer，
				// 上层一定要重新make，否则下次buffer池的读会覆盖上次还在处理中的收包
				count, err := xconn.ReadBatch(msgs, 0)
				if err == nil {
					var packets []*inPacket
					for i := 0; i < count; i++ {
						msg := msgs[i]
						if msg.N <= fecHeaderSize+4 {
							continue
						}
						buf := make([]byte, len(msg.Buffers[0][:msg.N]))
						copy(buf, msg.Buffers[0][:msg.N])
						packets = append(packets, &inPacket{msg.Addr, buf})
					}
					if len(packets) > 0 {
						select {
						case deliverTaskChan <- packets:
						default:
						}
					}
				} else {
					// 读失败不继续读，重新创建新的读套接字
					log.Errorf("[kcp read income packet] %v read conn read error:%v, retry new connection",
						no, err)
					break
				}
			}
		}
		time.Sleep(3 * time.Second)
	}
}

// handleInComePakcet 处理udp收包
func (l *Listener) handleInComePakcets(packets []*inPacket) {
	for _, msg := range packets {
		var conv uint32
		if len(msg.buf) <= fecHeaderSize+4 {
			log.Warnf(fmt.Errorf("[kcp handle income packet][%v] valid header length:%v/%v", msg.addr.String(), len(msg.buf), fecHeaderSize+4))
			return
		}

		ikcp_decode32u(msg.buf[fecHeaderSize:], &conv)
		s, ok := l.sessions.Load(conv)
		if !ok {
			log.Debugf(fmt.Sprintf("[kcp handle income packet][%v] not found session[%v]", msg.addr.String(), conv))
		} else {
			session, ok := s.(*Session)
			if !ok {
				log.Warnf(fmt.Sprintf("[kcp handle income packet]session[%v] convert session data fail:%v", msg.addr.String(), conv))
				return
			}
			session.remote_addr = msg.addr
			select {
			case session.chSocket <- msg.buf:
			default:
				log.Warnf("[kcp handle income packet][%v] deliver to session[%v], but session channel full", msg.addr.String(), conv)
			}
		}
	}
}

// genericPortConn 一般的udp套接字（不支持reuseport），单个
func genericPortConn(addr string) (*readConn, error) {
	newConnFun := func(address string) (net.PacketConn, error) {
		udpaddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return nil, err
		}

		conn, err := net.ListenUDP("udp", udpaddr)
		if err != nil {
			return nil, err
		}
		return conn, err
	}
	return &readConn{reconnFun: newConnFun}, nil
}

// reusePortConns reuseport的套接字，可以多个
func reusePortConns(addr string, num int) ([]*readConn, error) {
	conns := make([]*readConn, 0, num)
	for i := 0; i < num; i++ {
		f := func(a string) (net.PacketConn, error) {
			ln, err := reuseport.ListenPacket("udp", addr)
			if err != nil {
				return nil, err
			}
			return ln, err
		}

		conns = append(conns, &readConn{reconnFun: f})
	}

	return conns, nil
}

// func Control(network, address string, c syscall.RawConn) error {
//	var err error
//	c.Control(func(fd uintptr) {
//		err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
//		if err != nil {
//			return
//		}
//
//		err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
//		if err != nil {
//			return
//		}
//	})
//	return err
// }

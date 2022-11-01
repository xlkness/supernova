package kcp

import (
	"fmt"
	"net"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"github.com/pkg/errors"
)

type Client struct {
	conn    net.PacketConn // socket
	session *Session       // 会话
	Closed  bool

	Err     chan error  // 错误通道
	ChLogic chan []byte // 抛出数据给逻辑层的通道
}

func NewClient(conv uint32, raddr string) (*Client, error) {
	udpaddr, err := net.ResolveUDPAddr("udp", raddr)
	if err != nil {
		return nil, errors.Wrap(err, "net.ResolveUDPAddr")
	}

	conn, err := net.DialUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0}, udpaddr)
	if err != nil {
		return nil, errors.Wrap(err, "net.DialUDP")
	}

	c := new(Client)
	c.conn = conn
	c.session = newSession(conv, net.PacketConn(conn))
	c.session.remote_addr = nil
	c.ChLogic = c.session.ChLogic
	c.Closed = false
	c.session.isServer = false

	// go func() {
	// 	ticker := time.NewTicker(time.Duration(10) * time.Millisecond)
	// 	defer ticker.Stop()
	// 	curt := 0
	// 	for !c.Closed {
	// 		select {
	// 		case <-ticker.C:
	// 			c.session.kcp.Update(uint32(curt))
	// 			curt += 33
	// 		}
	// 	}
	// }()

	go c.monitor()

	return c, nil
}

// 发送数据
func (c *Client) Send(buf []byte) {
	c.session.Send(buf)
}

// 监听socket收到的消息
func (c *Client) monitor() {
CLOSED:
	for {
		if c.Closed {
			break
		}

		buf := make([]byte, 1024)
		n, _, err := c.conn.ReadFrom(buf)
		if c.Closed {
			break CLOSED
		} else if err != nil {
			log.Errorf("session %v read error:%v", c.session.conv, err)
		} else {
			buf = buf[:n]
			select {
			case c.session.chSocket <- buf:
			default:
			}

		}
	}
}

func (c *Client) monitor1() {
	var xconn batchConn

	msgs := make([]ipv4.Message, 16)
	for k := range msgs {
		msgs[k].Buffers = [][]byte{make([]byte, Pack_max_len)}
	}

	if _, ok := c.conn.(*net.UDPConn); ok {
		addr, err := net.ResolveUDPAddr("udp", c.conn.LocalAddr().String())
		if err == nil {
			if addr.IP.To4() != nil {
				xconn = ipv4.NewPacketConn(c.conn)
			} else {
				xconn = ipv6.NewPacketConn(c.conn)
			}
		}
	} else {
		panic(fmt.Errorf("kcp conn is not net.udpconn"))
	}
CLOSED:
	for {
		if c.Closed {
			break
		}

		count, err := xconn.ReadBatch(msgs, 0)
		//fmt.Printf("recv:%v,%v\n", c.session.conv, count)
		fmt.Printf("recv:%v, error:%v\n", count, err)
		if err == nil {
			for i := 0; i < count; i++ {
				msg := msgs[i]
				fmt.Printf("%v\n", msg.N)
				if msg.N <= fecHeaderSize+4 {
					continue
				}
				buf := make([]byte, len(msg.Buffers[0][:msg.N]))

				copy(buf, msg.Buffers[0][:msg.N])

				if c.Closed {
					break CLOSED
				} else {
					select {
					case c.session.chSocket <- buf:
					default:
						fmt.Printf("full\n")
					}
				}
			}
		} else {
			log.Errorf("session %v read error:%v", c.session.conv, err)
		}
	}
}

// 关闭
func (c *Client) Close() {
	c.Closed = true
	c.session.close()
	c.conn.Close()
}

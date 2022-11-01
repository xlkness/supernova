package udp

import (
	"net"
	"time"

	internalSocket "joynova.com/library/supernova/pkg/netcore/socket/socket"
)

type server struct {
	addr string
	net.Conn
	maxRecvMsgBytes int
	maxWriteTimeout time.Duration
}

func NewServer(addr string) *server {
	listener := &server{}
	listener.addr = addr
	return listener
}

func (s *server) Listen() error {
	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}

	s.Conn = conn

	for {
		buf := make([]byte, s.maxRecvMsgBytes)
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			// todo log
			return err
		}

		if n < 4 {
			// todo log
			continue
		}

		// tag := binary.LittleEndian.Uint32(buf)
		// payload := buf[4:]
		// client := s.newClientConn(conn, cAddr)
		// e := event.NewRecvEvent(tag, payload)
		// s.eventNotify(client)
	}
}

func (s *server) Stop() {
	s.Close()
}

type clientConn struct {
	id              int64
	conn            *net.UDPConn
	addr            *net.UDPAddr
	maxRecvMsgBytes int
	maxRecvTimeout  time.Duration
	maxWriteTimeout time.Duration
}

func (c *clientConn) GetClientConnType() internalSocket.InternalClientConnType {
	return internalSocket.InternalClientConnTypeWs
}

func (c *clientConn) GetID() int64 {
	return c.id
}

func (c *clientConn) GetIP() string {
	return c.addr.String()
}

func (c *clientConn) SetWriteTimeout(d time.Duration) {
	c.maxWriteTimeout = d
}
func (c *clientConn) SetRecvTimeout(d time.Duration) {
	c.maxRecvTimeout = d
}
func (c *clientConn) SetRecvMsgBytes(n int) {
	c.maxRecvMsgBytes = n
}

func (c *clientConn) Write(msg []byte) (int, error) {
	return c.conn.WriteToUDP(msg, c.addr)
}

func (c *clientConn) WriteTLV(tag uint32, payload []byte) (int, error) {
	return 0, nil
}

func (c *clientConn) Close() {
	c.conn.Close()
}

func (s *server) newClientConn(conn *net.UDPConn, addr *net.UDPAddr) *clientConn {
	c := &clientConn{}
	c.id = internalSocket.GetID()
	c.conn = conn
	c.addr = addr
	return c
}

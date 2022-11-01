package tcp

import (
	"bufio"
	"net"
	"sync"
	"time"

	"joynova.com/library/supernova/pkg/netcore/socket/event"
	internalSocket "joynova.com/library/supernova/pkg/netcore/socket/socket"
	"joynova.com/library/supernova/pkg/netcore/socket/utils"
)

type server struct {
	addr string
	net.Conn
	newSessionFunc func(conn internalSocket.InternalClientConn) internalSocket.InternalSession
	sessionMgr     *sync.Map
	option         *internalSocket.InternalOption
}

func NewServer(addr string, newSessionFunc func(conn internalSocket.InternalClientConn) internalSocket.InternalSession,
	option *internalSocket.InternalOption) *server {
	listener := &server{}
	listener.addr = addr
	listener.newSessionFunc = newSessionFunc
	listener.sessionMgr = new(sync.Map)
	listener.option = option
	return listener
}

func (s *server) Listen() error {
	listenFd, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	for {
		conn, err := listenFd.Accept()
		if err != nil {
			return err
		}
		client := s.newClientConn(conn, s.option)
		customSession := s.newSessionFunc(client)
		s.sessionMgr.Store(client.GetSessionID(), customSession)
		go client.handleClientConnRead(s, customSession)
		go client.handleClientConnDeliverRecvMsg(customSession)
		go client.handleClientConnWriteMsg(customSession)
	}

	return nil
}

func (s *server) GetSession(id int64) (internalSocket.InternalSession, bool) {
	value, find := s.sessionMgr.Load(id)
	if find {
		return value.(internalSocket.InternalSession), find
	}
	return nil, false
}

// CloseSession 服务器主动关闭客户端
func (s *server) CloseSession(id int64, notify *utils.TLVPacket, delay time.Duration, data interface{}) bool {
	sessionValue, find := s.sessionMgr.Load(id)
	if !find {
		return false
	}

	session, _ := sessionValue.(internalSocket.InternalSession)
	// 先删除session
	s.sessionMgr.Delete(id)
	// 调用钩子
	session.PreServerSideCloseSession(notify, data)

	time.AfterFunc(delay, func() {
		// 关闭链接
		session.GetClientConn().(*clientConn).close()
		// 调用钩子
		session.ServerSideCloseSession(data)
	})

	return true
}

// closeSession 客户端主动关闭
func (s *server) closeSession(id int64, err event.Error) {
	sessionValue, find := s.sessionMgr.Load(id)
	if !find {
		return
	}
	session, _ := sessionValue.(internalSocket.InternalSession)
	// 先删除session
	s.sessionMgr.Delete(id)
	// 调用钩子
	session.ClientSideCloseSession(err)
	// 关闭链接
	session.GetClientConn().(*clientConn).close()
}

func (s *server) Stop() {
	s.Close()
}

func (s *server) newClientConn(conn net.Conn, option *internalSocket.InternalOption) *clientConn {
	c := &clientConn{}
	c.id = internalSocket.GetID()
	c.conn = conn
	c.inStream = bufio.NewReader(conn)
	c.recvQueue = make(chan *utils.TLVPacket, 20)
	c.writeQueue = make(chan []byte, 20)
	c.stopChan = make(chan struct{}, 0)
	c.option = option
	return c
}

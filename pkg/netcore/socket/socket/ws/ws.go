package ws

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/websocket"
	"joynova.com/library/supernova/pkg/netcore/socket/event"
	internalSocket "joynova.com/library/supernova/pkg/netcore/socket/socket"
	"joynova.com/library/supernova/pkg/netcore/socket/utils"
)

type Server struct {
	addr      string
	GinEngine *gin.Engine
	net.Conn
	fs             http.FileSystem
	newSessionFunc func(internalSocket.InternalClientConn) internalSocket.InternalSession
	sessionMgr     *sync.Map
	option         *internalSocket.InternalOption
	loggerFun      func(params gin.LogFormatterParams)
	panicOutputFun func(string)
}

func NewServer(addr string, newSessionFunc func(internalSocket.InternalClientConn) internalSocket.InternalSession, option *internalSocket.InternalOption, fs http.FileSystem) *Server {
	listener := &Server{}
	listener.addr = addr
	listener.newSessionFunc = newSessionFunc
	listener.sessionMgr = new(sync.Map)
	listener.option = option
	listener.fs = fs
	listener.GinEngine = gin.New()
	return listener
}

func NewServerWithLogger(addr string, newSessionFunc func(internalSocket.InternalClientConn) internalSocket.InternalSession, option *internalSocket.InternalOption,
	loggerFun func(params gin.LogFormatterParams), panicOutputFun func(string), fs http.FileSystem) *Server {
	s := NewServer(addr, newSessionFunc, option, fs)
	s.loggerFun = loggerFun
	s.panicOutputFun = panicOutputFun
	return s
}

func (s *Server) Listen() error {

	s.GinEngine.Use(ginLoggerFun(s.loggerFun))
	s.GinEngine.Use(RecoveryWithWriter(s.panicOutputFun))

	// router.StaticFS("/client", &TestH5ClientHtml{
	// 	Fs:   _asserts.H5,
	// 	Path: "h5",
	// })
	// s.GinEngine.StaticFS("/client", s.fs)
	s.GinEngine.GET("/ws", func(wsConnHandle websocket.Handler) gin.HandlerFunc {
		return func(c *gin.Context) {
			if c.IsWebsocket() {
				wsConnHandle.ServeHTTP(c.Writer, c.Request)
			} else {
				_, _ = c.Writer.WriteString("===not websocket request===")
			}
		}
	}(func(conn *websocket.Conn) {
		client := s.newClientConn(conn, s.option)
		customSession := s.newSessionFunc(client)
		s.sessionMgr.Store(client.GetSessionID(), customSession)
		go client.handleClientConnRead(s, customSession)
		client.handleClientConnRead(s, customSession)
	}))

	return s.GinEngine.Run(s.addr)
}

func (s *Server) GetSession(id int64) (internalSocket.InternalSession, bool) {
	value, find := s.sessionMgr.Load(id)
	if find {
		return value.(internalSocket.InternalSession), find
	}
	return nil, false
}

// CloseSession 服务器主动关闭客户端
func (s *Server) CloseSession(id int64, notify *utils.TLVPacket, delay time.Duration, data interface{}) bool {
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
func (s *Server) closeSession(id int64, err event.Error) {
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

func (s *Server) Stop() {
	s.Close()
}

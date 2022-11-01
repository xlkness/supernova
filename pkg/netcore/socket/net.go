package socket

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"joynova.com/library/supernova/pkg/netcore/socket/socket"
	"joynova.com/library/supernova/pkg/netcore/socket/socket/tcp"
	"joynova.com/library/supernova/pkg/netcore/socket/socket/ws"
)

type Option = internal_socket.InternalOption
type ClientConn = internal_socket.InternalClientConn
type ClientConnType = internal_socket.InternalClientConnType
type Session = internal_socket.InternalSession
type Server = internal_socket.InternalServer

var ClientConnTypeTcp = internal_socket.InternalClientConnTypeTcp
var ClientConnTypeWs = internal_socket.InternalClientConnTypeWs

func NewServer(commType, addr string, newSessionFunc func(ClientConn) Session, option *Option) Server {
	if commType == "tcp" {
		return tcp.NewServer(addr, newSessionFunc, option)
	} else if commType == "udp" {
		// return udp.NewServer(addr)
	}
	// else if commType == "ws" {
	// 	return ws.NewServer(addr, newConnFun)
	// }

	return nil
}

func NewWSServer(addr string, newSessionFunc func(ClientConn) Session, option *Option,
	loggerFun func(params gin.LogFormatterParams), panicOutputFun func(string), fs http.FileSystem) Server {
	return ws.NewServerWithLogger(addr, newSessionFunc, option, loggerFun, panicOutputFun, fs)
}

func SetLogErrorFun(f func(session internal_socket.InternalSession, format string, args ...interface{})) {
	internal_socket.InternalLogErrorFun = f
}

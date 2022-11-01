package internal_socket

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"sync/atomic"
	"time"

	"joynova.com/library/supernova/pkg/netcore/socket/event"
	"joynova.com/library/supernova/pkg/netcore/socket/utils"
)

type InternalClientConnType int

var InternalClientConnTypeTcp InternalClientConnType = 1
var InternalClientConnTypeWs InternalClientConnType = 2

type InternalOption struct {
	RecvTimeout  time.Duration // optional
	RecvMsgBytes int           // optional, default 1<<16
	WriteTimeout time.Duration // optional

}

var incID int64

func init() {
	rand.Seed(time.Now().UnixNano())
	incID = 10000000
}

func GetID() int64 {
	// 缓冲
	max := int64(math.MaxInt64 - 10000)

	for {
		cur := atomic.AddInt64(&incID, 1)
		if cur > max || cur <= 0 {
			if atomic.CompareAndSwapInt64(&incID, cur, 1) {
				return 1
			} else {
				continue
			}
		} else {
			return cur
		}
	}
}

type InternalServer interface {
	Listen() error
	Stop()
	GetSession(int64) (InternalSession, bool)
	CloseSession(int64, *utils.TLVPacket, time.Duration, interface{}) bool
}

type InternalClientConn interface {
	GetClientConnType() InternalClientConnType
	GetSessionID() int64
	GetIP() string
	GetConn() net.Conn
	InitSession(*InternalOption)
	WriteTLV(s InternalSession, tag uint32, payload []byte) (int, error)
}

// InternalSession
type InternalSession interface {
	GetClientConn() InternalClientConn
	// PreHandleRequest 处理请求之前
	PreHandleRequest(*utils.TLVPacket) (*utils.TLVPacket, interface{}, error)
	// HandleRequest 处理请求包
	HandleRequest(request *utils.TLVPacket, customData interface{}) (*utils.TLVPacket, interface{}, error)
	// PreHandleResponse 准备响应前
	PreHandleResponse(request *utils.TLVPacket, response *utils.TLVPacket, customData interface{}) error
	// PostHandleResponse 处理响应之后
	PostHandleResponse(request *utils.TLVPacket, response *utils.TLVPacket, customData interface{}, writeError error)
	// PreHandleNotify 处理通知之前
	PreHandleNotify(uint32, []byte)
	// PreWritePacket 最底层发送包之前
	PreWritePacket(packet []byte)
	// PreServerSideCloseSession 服务器主动删除session，但是还没关闭tcp链接
	PreServerSideCloseSession(*utils.TLVPacket, interface{})
	// ServerSideCloseSession 服务器关闭了tcp链接
	ServerSideCloseSession(interface{})
	// ClientSideCloseSession 客户端主动关闭链接
	ClientSideCloseSession(err event.Error)
}

var InternalLogErrorFun = func(conn InternalSession, format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

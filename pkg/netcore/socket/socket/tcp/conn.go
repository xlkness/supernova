package tcp

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"joynova.com/library/supernova/pkg/jlog"
	"joynova.com/library/supernova/pkg/netcore/socket/event"
	internalSocket "joynova.com/library/supernova/pkg/netcore/socket/socket"
	"joynova.com/library/supernova/pkg/netcore/socket/utils"
)

type clientConn struct {
	id            int64
	customSession internalSocket.InternalSession
	recvQueue     chan *utils.TLVPacket // 读缓冲，避免逻辑层接收不及时，内核层堆积
	writeQueue    chan []byte           // 写缓冲
	conn          net.Conn
	inStream      *bufio.Reader
	option        *internalSocket.InternalOption
	isStop        int32
	stopChan      chan struct{}
}

func (c *clientConn) GetClientConnType() internalSocket.InternalClientConnType {
	return internalSocket.InternalClientConnTypeTcp
}

func (c *clientConn) GetSessionID() int64 {
	return c.id
}

func (c *clientConn) GetIP() string {
	return c.conn.RemoteAddr().String()
}

func (c *clientConn) InitSession(op *internalSocket.InternalOption) {
	c.option = op
}

func (c *clientConn) GetConn() net.Conn {
	return c.conn
}

func (c *clientConn) WriteTLV(session internalSocket.InternalSession, tag uint32, payload []byte) (int, error) {
	session.PreHandleNotify(tag, payload)
	return c.writeTLV(tag, payload)
}

func (c *clientConn) writeTLV(tag uint32, payload []byte) (int, error) {
	buf := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(buf, tag)
	binary.BigEndian.PutUint32(buf[4:], uint32(len(payload)))
	copy(buf[8:], payload)
	return c.write(buf)
}

func (c *clientConn) write(buf []byte) (int, error) {
	if atomic.LoadInt32(&c.isStop) == 1 {
		return len(buf), nil
	}

	c.writeQueue <- buf
	return len(buf), nil
}

func (c *clientConn) close() {
	if atomic.LoadInt32(&c.isStop) == 1 {
		return
	}
	atomic.StoreInt32(&c.isStop, 1)
	c.conn.Close()
	close(c.stopChan)
}

func (conn *clientConn) handleClientConnRead(server *server, customSession internalSocket.InternalSession) {
	for {
		setOption := conn.option
		if setOption.RecvTimeout > 0 {
			conn.conn.SetReadDeadline(time.Now().Add(setOption.RecvTimeout))
		}
		var maxRecvBytes int = 1 << 16
		if setOption.RecvMsgBytes > 0 {
			maxRecvBytes = setOption.RecvMsgBytes
		}

		packet, err := utils.ReadTLVMsg(conn.inStream, int32(maxRecvBytes))
		if err != nil {
			if atomic.LoadInt32(&conn.isStop) == 1 {
				break
			}
			if e, ok := err.(net.Error); ok {
				if e.Timeout() {
					// 客户端心跳超时
					server.closeSession(conn.GetSessionID(), event.ErrReadTimeout)
					break
				} else if e.Temporary() {
					continue
				}
			}

			// 客户端遇到其它错误
			server.closeSession(conn.GetSessionID(), event.Error(err))
			break
		}

		conn.recvQueue <- packet
	}
}

func (conn *clientConn) handleClientConnDeliverRecvMsg(customSession internalSocket.InternalSession) {

	for {
		select {
		case msg, ok := <-conn.recvQueue:
			if !ok {
				return
			}
			conn.handleRecvMsg(customSession, msg)
		case <-conn.stopChan:
			return
		}
	}
}

func (conn *clientConn) handleRecvMsg(customSession internalSocket.InternalSession, msg *utils.TLVPacket) {
	defer jlog.CatchWithInfo(fmt.Sprintf("handle session(%v) receive msg(%v) panic", conn.GetSessionID(), msg.Tag))

	res, data, err := customSession.PreHandleRequest(msg)
	if err != nil {
		return
	}
	if res != nil && res.Tag > 0 {
		conn.writeTLV(res.Tag, res.Payload)
		return
	}

	res, data, err = customSession.HandleRequest(msg, data)
	if err != nil {
		customSession.PostHandleResponse(msg, res, data, err)
		return
	}

	if res == nil || res.Tag <= 0 {
		return
	}

	customSession.PreHandleResponse(msg, res, data)

	conn.writeTLV(res.Tag, res.Payload)

	customSession.PostHandleResponse(msg, res, data, nil)
}

func (conn *clientConn) handleClientConnWriteMsg(customSession internalSocket.InternalSession) {

	for {
		select {
		case msg, ok := <-conn.writeQueue:
			if !ok {
				return
			}

			if atomic.LoadInt32(&conn.isStop) == 1 {
				return
			}

			if conn.option.WriteTimeout > 0 {
				conn.conn.SetWriteDeadline(time.Now().Add(conn.option.WriteTimeout))
			}

			customSession.PreWritePacket(msg)

			_, err := conn.conn.Write(msg)
			if err != nil {
				// internalSocket.InternalLogErrorFun(conn.customSession, "[net core]conn[%v] write msg with len(%v) error:%v", conn.customSession, len(msg), err)
				jlog.Errorf("[net core]conn[%v] write msg with len(%v) error:%v", conn.customSession, len(msg), err)
			}
		case <-conn.stopChan:
			return
		}
	}
}

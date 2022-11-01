package ws

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync/atomic"

	"golang.org/x/net/websocket"
	"joynova.com/library/supernova/pkg/netcore/socket/event"
	internalSocket "joynova.com/library/supernova/pkg/netcore/socket/socket"
	"joynova.com/library/supernova/pkg/netcore/socket/utils"
)

type clientConn struct {
	id       int64
	conn     *websocket.Conn
	inStream *bufio.Reader
	option   *internalSocket.InternalOption
	isStop   int32
}

func (c *clientConn) GetClientConnType() internalSocket.InternalClientConnType {
	return internalSocket.InternalClientConnTypeWs
}

func (c *clientConn) GetSessionID() int64 {
	return c.id
}

func (c *clientConn) GetIP() string {
	return c.conn.LocalAddr().String()
}

func (c *clientConn) GetConn() net.Conn {
	return c.conn
}

func (c *clientConn) InitSession(op *internalSocket.InternalOption) {
	c.option = op
}

func (c *clientConn) WriteTLV(session internalSocket.InternalSession, tag uint32, payload []byte) (int, error) {
	session.PreHandleNotify(tag, payload)
	return c.writeTLV(session, tag, payload)
}

func (c *clientConn) writeTLV(session internalSocket.InternalSession, tag uint32, payload []byte) (int, error) {
	res := make(map[string]interface{})
	err := json.Unmarshal(payload, &res)
	if err != nil {
		return 0, fmt.Errorf("json unmarshal error:%v", err)
		// fmt.Printf("unmarshal error:%v,%v\n", string(payload), err)
	}
	buf := map[string]interface{}{
		"msg_id":  tag,
		"payload": res,
	}
	sendMsg, err := json.Marshal(&buf)
	if err != nil {
		return 0, fmt.Errorf("json marshal error:%v", err)
	}

	return c.write(session, sendMsg)
}

func (c *clientConn) write(session internalSocket.InternalSession, buf []byte) (int, error) {
	session.PreWritePacket(buf)
	return c.conn.Write(buf)
}

func (c *clientConn) close() {
	if atomic.LoadInt32(&c.isStop) == 1 {
		return
	}
	atomic.StoreInt32(&c.isStop, 1)
	c.conn.Close()
}

func (s *Server) newClientConn(conn *websocket.Conn, option *internalSocket.InternalOption) *clientConn {
	c := &clientConn{}
	c.id = internalSocket.GetID()
	c.conn = conn
	c.inStream = bufio.NewReader(conn)
	c.option = option
	return c
}

func (c *clientConn) handleClientConnRead(server *Server, session internalSocket.InternalSession) {
OUT:
	for {
		var wsMsg string
		err := websocket.Message.Receive(c.conn, &wsMsg)
		if err != nil {
			// 服务器主动掐段
			if atomic.LoadInt32(&c.isStop) == 1 {
				break
			}

			server.closeSession(c.GetSessionID(), event.Error(err))
			break
		}

		req := make(map[string]string)
		err = json.Unmarshal([]byte(wsMsg), &req)
		if err != nil {
			err = fmt.Errorf("unmarshal request message error:%v", err)
			c.conn.Write([]byte(err.Error()))
			continue
		}
		msgID, find := req["msg_id"]
		if !find {
			err = fmt.Errorf("not found msg id:%v", wsMsg)
			c.conn.Write([]byte(err.Error()))
			continue
		}

		tag, err := strconv.Atoi(msgID)
		if err != nil {
			err = fmt.Errorf("msg id is not integer:%v", wsMsg)
			c.conn.Write([]byte(err.Error()))
			continue
		}

		payload, find := req["payload"]
		if !find {
			err = fmt.Errorf("not found msg:%v", wsMsg)
			c.conn.Write([]byte(err.Error()))
			continue
		}

		requestTlv := &utils.TLVPacket{Tag: uint32(tag), Payload: []byte(payload)}

		res, data, err := session.PreHandleRequest(requestTlv)
		if err != nil {
			continue OUT
		}
		if res != nil && res.Tag > 0 {
			c.writeTLV(session, res.Tag, res.Payload)
			continue OUT
		}

		res, data, err = session.HandleRequest(requestTlv, data)
		if err != nil {
			session.PostHandleResponse(requestTlv, res, data, err)
			continue OUT
		}

		if res == nil || res.Tag <= 0 {
			continue OUT
		}

		session.PreHandleResponse(requestTlv, res, data)
		_, err = c.writeTLV(session, res.Tag, res.Payload)
		session.PostHandleResponse(requestTlv, res, data, err)
	}
}

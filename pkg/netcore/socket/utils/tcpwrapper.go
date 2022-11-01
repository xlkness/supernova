package utils

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

/*
	memory layout in a frame:
	|--------------------------------------------------------|
	|--tag:4bytes--|--length:4bytes--|--payload:maxLength-8--|
	|--------------------------------------------------------|
	note: tag/length are both using little endian
*/

type TLVPacket struct {
	Tag     uint32
	Payload []byte
}

// read tlv msg from socket in stream
func ReadTLVMsg(inStream *bufio.Reader, maxLength int32) (
	packet *TLVPacket, err error) {
	headbuf := make([]byte, 8)
	n, err := io.ReadFull(inStream, headbuf)
	if err != nil {
		return nil, err
	}

	if n != 8 {
		return nil, fmt.Errorf("error tlv header:%v", n)
	}

	tag := binary.BigEndian.Uint32(headbuf)
	length := binary.BigEndian.Uint32(headbuf[4:])

	if length+8 > uint32(maxLength) {
		return &TLVPacket{Tag: tag}, fmt.Errorf("error read tlv(%v) reach max length:%v/%v", tag, length, uint32(maxLength))
	}

	payload := make([]byte, length)
	n, err = io.ReadFull(inStream, payload)
	if err != nil {
		return &TLVPacket{Tag: tag}, err
	}
	if n != int(length) {
		return &TLVPacket{Tag: tag}, fmt.Errorf("error read tlv invalid length:%v/%v", n, length)
	}

	return &TLVPacket{Tag: tag, Payload: payload}, nil
}

// write tlv msg
func WriteTLVMsg(conn net.Conn, tag uint32, payload []byte, deadLine time.Duration) error {
	err := conn.SetWriteDeadline(time.Now().Add(deadLine))
	if err != nil {
		return err
	}
	msg := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint32(msg, tag)
	binary.LittleEndian.PutUint32(msg, uint32(len(payload)))
	copy(msg, payload)
	n, err := conn.Write(msg)
	if err != nil {
		return fmt.Errorf("write msg, tag:%v err:%v", tag, err)
	}
	if n != len(msg) {
		return fmt.Errorf("write msg:%v, err len:%v/%v", tag, n, len(msg))
	}
	return nil

}

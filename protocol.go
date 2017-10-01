package websocket

import (
	"bufio"
	"encoding/binary"
	"io"
)

const (
	//帧头部最小长度
	MIN_HEAD_LEN = 2

	//帧类型常量
	//继续帧
	FRAME_TYPE_CONTINUE byte = 0
	//文本帧
	FRAME_TYPE_TEXT byte = 1
	//二进制帧
	FRAME_TYPE_BINARY byte = 2
	//关闭帧
	FRAME_TYPE_CLOSE byte = 8
	//Ping帧
	FRAME_TYPE_PING byte = 9
	//Pong帧
	FRAME_TYPE_PONG byte = 10

	//是否使用了额外的负载头部
	EXTENDED_PAYLOAD_LEN0  = 0
	EXTENDED_PAYLOAD_LEN16 = 16
	EXTENDED_PAYLOAD_LEN64 = 64
)

//将数据编码成websocket协议格式
func EncodeProto(b []byte) ([]byte, error) {
	return []byte{}, nil
}

//解析websocket协议数据
func DecodeProto(r io.Reader) (data []byte, err error) {
	b := bufio.NewReaderSize(r, BUF_SIZE)

	//因为websocket的头部长度是不定的
	//头部的长度依赖于头部的值
	//因此需要一段一段的来解析

	//头部初始长度
	var headerLen int = MIN_HEAD_LEN

	//是否使用了额外的负载长度
	var extendedPayload = EXTENDED_PAYLOAD_LEN0

	var tmp []byte
	tmp, err = b.Peek(1)
	if err != nil {
		return []byte{}, err
	}

	//获取帧类型
	opcode := tmp[0] & 15
	if opcode == FRAME_TYPE_CLOSE {
		return []byte{}, ErrConnClosed
	}

	//是否为终止帧
	fin := tmp[0]>>7 == 1

	tmp, err = b.Peek(2)
	if err != nil {
		return []byte{}, nil
	}

	//消息是否加密
	mask := tmp[1]>>7 == 1
	if mask {
		headerLen += 4
	}

	//获取数据帧负载长度
	payloadLen := uint64(tmp[1] & 127)
	if payloadLen == 126 {
		extendedPayload = EXTENDED_PAYLOAD_LEN16

		headerLen += 2

		tmp, err = b.Peek(4)
		if err != nil {
			return []byte{}, nil
		}

		payloadLen, _ = binary.Uvarint([]byte{0, 0, 0, 0, 0, 0, tmp[2], tmp[3]})
	} else if payloadLen == 127 {
		extendedPayload = EXTENDED_PAYLOAD_LEN64

		headerLen += 8

		tmp, err = b.Peek(10)
		if err != nil {
			return []byte{}, nil
		}

		payloadLen, _ = binary.Uvarint(tmp[2:])
	}

	//数据帧总长度
	var totalLen uint64 = uint64(headerLen) + payloadLen

	//读取消息
	var raw = make([]byte, totalLen)
	_, err = b.Read(raw)
	if err != nil {
		return []byte{}, nil
	}

	//负载部分
	payload := raw[headerLen:]

	//如果消息被加密，解密
	if mask {
		//获取mask key
		maskKeyPosition := 2
		if extendedPayload == EXTENDED_PAYLOAD_LEN16 {
			maskKeyPosition += 2
		} else if extendedPayload == EXTENDED_PAYLOAD_LEN64 {
			maskKeyPosition += 8
		}
		maskKey := raw[maskKeyPosition:(maskKeyPosition + 4)]

		//解密
		data = make([]byte, 0)
		for i := 0; i < len(payload); i++ {
			data = append(data, payload[i]^maskKey[i%4])
		}
	}

	//如果为终止帧，直接返回
	//如果不是，递归解析数据
	if !fin {
		next, err := DecodeProto(r)
		if err != nil {
			return []byte{}, err
		}

		data = append(data, next...)
	}

	return data, nil
}

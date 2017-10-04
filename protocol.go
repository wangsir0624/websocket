package websocket

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
)

type FrameType byte

const (
	//帧头部最小长度
	MIN_HEAD_LEN = 2

	//帧类型常量
	//继续帧
	FRAME_TYPE_CONTINUE FrameType = 0
	//文本帧
	FRAME_TYPE_TEXT FrameType = 1
	//二进制帧
	FRAME_TYPE_BINARY FrameType = 2
	//关闭帧
	FRAME_TYPE_CLOSE FrameType = 8
	//Ping帧
	FRAME_TYPE_PING FrameType = 9
	//Pong帧
	FRAME_TYPE_PONG FrameType = 10

	//是否使用了额外的负载头部
	EXTENDED_PAYLOAD_LEN0  = 0
	EXTENDED_PAYLOAD_LEN16 = 16
	EXTENDED_PAYLOAD_LEN64 = 64
)

//将数据编码成websocket协议text格式
func EncodeProtoText(b []byte) []byte {
	return encodeProto(b, FRAME_TYPE_TEXT)
}

//将数据编码程websocket协议binary格式
func EncodeProtoBinary(b []byte) []byte {
	return encodeProto(b, FRAME_TYPE_BINARY)
}

//编码Pong响应
func EncodeProtoPong(b []byte) []byte {
	return encodeProto(b, FRAME_TYPE_PONG)
}

func encodeProto(b []byte, ft FrameType) (data []byte) {
	data = make([]byte, 0)

	//第一个字节
	//帧为结束帧，因此第一个字节第一位为1
	//后面七位取决于帧类型
	//如果opcode为text类型，那么第一个字节即为10000001
	//如果opcode为binary类型，那么第一个字节即为10000010
	//如果opcode为pong类型，那么第一个字节即为10001010
	switch ft {
	case FRAME_TYPE_TEXT:
		data = append(data, 129)
	case FRAME_TYPE_BINARY:
		data = append(data, 130)
	case FRAME_TYPE_PONG:
		data = append(data, 138)
	}

	//第二个字节
	//使用MASK，因此第二个字节第一位为1
	//接下来七位取决于数据长度
	length := len(b)
	var extendedPayload = EXTENDED_PAYLOAD_LEN0
	if length < 126 {
		data = append(data, byte(128^length))
	} else if length < 65535 {
		data = append(data, 254)
		extendedPayload = EXTENDED_PAYLOAD_LEN16
	} else {
		data = append(data, 255)
		extendedPayload = EXTENDED_PAYLOAD_LEN64
	}

	//是否使用了额外的负载长度
	if extendedPayload == EXTENDED_PAYLOAD_LEN16 {
		tmp := make([]byte, 2)
		binary.BigEndian.PutUint16(tmp, uint16(length))
		data = append(data, tmp...)
	} else if extendedPayload == EXTENDED_PAYLOAD_LEN64 {
		tmp := make([]byte, 8)
		binary.BigEndian.PutUint64(tmp, uint64(length))
		data = append(data, tmp...)
	}

	//生成随机的mask key
	r := rand.Int31()
	var maskKey = make([]byte, 4)
	binary.BigEndian.PutUint32(maskKey, uint32(r))
	data = append(data, maskKey...)

	//数据负载
	payload := make([]byte, 0)
	for i := 0; i < length; i++ {
		payload = append(payload, b[i]^maskKey[i%4])
	}
	data = append(data, payload...)
	fmt.Println(data)

	return data
}

//解析websocket协议数据
//返回数据，帧类型，错误
func DecodeProto(r io.Reader) (data []byte, frameType FrameType, err error) {
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
		return []byte{}, 0, err
	}

	//获取帧类型
	opcode := FrameType(tmp[0] & 15)
	if opcode == FRAME_TYPE_CLOSE {
		return []byte{}, FRAME_TYPE_CLOSE, nil
	}

	//是否为终止帧
	fin := tmp[0]>>7 == 1

	tmp, err = b.Peek(2)
	if err != nil {
		return []byte{}, 0, err
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
			return []byte{}, 0, err
		}

		payloadLen, _ = binary.Uvarint([]byte{0, 0, 0, 0, 0, 0, tmp[2], tmp[3]})
	} else if payloadLen == 127 {
		extendedPayload = EXTENDED_PAYLOAD_LEN64

		headerLen += 8

		tmp, err = b.Peek(10)
		if err != nil {
			return []byte{}, 0, err
		}

		payloadLen, _ = binary.Uvarint(tmp[2:])
	}

	//数据帧总长度
	var totalLen uint64 = uint64(headerLen) + payloadLen

	//读取消息
	var raw = make([]byte, 0)
	var length uint64 = 0
	for length < totalLen {
		tmp := make([]byte, 1024)
		l, err := b.Read(tmp)
		length += uint64(l)
		if l > 0 {
			raw = append(raw, tmp[:l]...)
		}
		if err != nil {
			return []byte{}, 0, err
		}
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
		var i uint64 = 0
		for ; i < payloadLen; i++ {
			data = append(data, payload[i]^maskKey[i%4])
		}
	}

	//如果为终止帧，直接返回
	//如果不是，递归解析数据
	if !fin {
		next, _, err := DecodeProto(r)
		if err != nil {
			return []byte{}, 0, err
		}

		data = append(data, next...)
	}

	return data, opcode, nil
}

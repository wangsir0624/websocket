package websocket

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"net"
	"net/http"
)

type Conn struct {
	net.Conn

	server *Server

	handshaked bool //是否完成握手

	data []byte //连接接受到的数据

	err error //错误
}

//获取服务器对象
func (c *Conn) GetServer() *Server {
	return c.server
}

//给客户端发送数据
func (c *Conn) Send(b []byte) (int, error) {
	//编码程websocket消息
	data := EncodeProtoText(b)
	return c.sendRaw(data)
}

//以二进制格式给客户端发送数据
func (c *Conn) SendBinary(b []byte) (int, error) {
	data := EncodeProtoBinary(b)
	return c.sendRaw(data)
}

//读取连接接收到的消息
func (c *Conn) GetData() []byte {
	return c.data
}

//获取错误信息
func (c *Conn) GetErr() error {
	return c.err
}

//发送Pong响应
func (c *Conn) sendPong(b []byte) (int, error) {
	data := encodeProtoPong(b)
	return c.sendRaw(data)
}

//不经过协议处理，直接发送给客户端
func (c *Conn) sendRaw(b []byte) (int, error) {
	length := 0
	for {
		l, err := c.Write(b[length:])
		length += l
		if err != nil {
			if err == io.ErrShortWrite {
				continue
			} else {
				return length, err
			}
		}

		if length == len(b) {
			return length, nil
		}
	}
}

//处理来自客户端的消息
func (c *Conn) handleData() {
	defer func() {
		err := recover()
		if err, ok := err.(error); err != nil && ok {
			if c.server.onerror != nil {
				c.err = err
				c.server.onerror(c)
			}

			c.server.removeConn(c)

			c.Close()

			return
		}
	}()

	for {
		if !c.handshaked {
			//握手
			br := bufio.NewReaderSize(c.Conn, BUF_SIZE)
			r, err := http.ReadRequest(br)
			if err != nil {
				c.sendBadHandshakeRequestResponse()

				c.server.removeConn(c)

				c.Close()

				return
			} else {
				h := r.Header

				//获取Sec-Websocket-Key头部
				websocketKey := h.Get("Sec-Websocket-Key")
				upgrade := h.Get("Upgrade")

				if websocketKey != "" && upgrade == "websocket" {
					//计算Sec-Websocket-Accept头部
					var tmp1 [sha1.Size]byte
					var tmp2 = make([]byte, 64)
					tmp1 = sha1.Sum([]byte(websocketKey + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
					base64.StdEncoding.Encode(tmp2, tmp1[:])
					tmp2 = bytes.TrimRightFunc(tmp2, func(r rune) bool {
						return r == 0
					})
					websocketAccept := string(tmp2)

					//返回响应
					resp := []byte("HTTP/1.1 101 Switching Protocol\r\n" +
						"Upgrade: websocket\r\n" +
						"Connection: Upgrade\r\n" +
						"Sec-Websocket-Accept: " + websocketAccept + "\r\n" +
						"\r\n")

					_, err := c.sendRaw(resp)
					if err != nil {
						panic(err)
					}

					c.handshaked = true

					if c.server.onconnection != nil {
						c.server.onconnection(c)
					}
				} else {
					c.sendBadHandshakeRequestResponse()

					c.server.removeConn(c)

					c.Close()

					return
				}
			}
		} else {
			var message []byte

			//解析websocket数据
			message, ft, err := DecodeProto(c.Conn)
			if err != nil {
				panic(err)
			}
			switch ft {
			case FRAME_TYPE_CLOSE:
				if c.server.onclose != nil {
					c.server.onclose(c)
				}

				c.server.removeConn(c)

				c.Close()
				return
			case FRAME_TYPE_TEXT, FRAME_TYPE_BINARY:
				c.data = message

				if c.server.onmessage != nil {
					c.server.onmessage(c)
				}
			case FRAME_TYPE_PING:
				c.sendPong(message)
			}
		}
	}
}

func (c *Conn) sendBadHandshakeRequestResponse() {
	r := "HTTP/1.1 400 BadRequest\r\n"
	r += "\r\n"

	c.sendRaw([]byte(r))
}

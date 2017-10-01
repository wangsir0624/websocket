package websocket

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	//"fmt"
	"io"
	"net"
	"net/http"
)

type Conn struct {
	net.Conn

	server *Server

	handshaked bool //是否完成握手

	data *bytes.Reader //连接接受到的数据
}

/*
func (c *Conn) Send(b []byte) (int, error) {

}*/

func (c *Conn) SendRaw(b []byte) (int, error) {
	length := 0
	for {
		l, err := c.Write(b)
		if err != nil {
			if err == io.ErrShortWrite {
				continue
			} else {
				return length, err
			}
		}

		if length += l; length == len(b) {
			return length, nil
		}
	}
}

//获取连接接收到的消息
func (c *Conn) GetData() []byte {
	if c.data == nil {
		return []byte{}
	}

	d := make([]byte, c.data.Len())
	_, _ = c.data.Read(d)

	return d
}

func (c *Conn) handleData() {
	defer func() {
		err := recover()
		if err != nil {
			if c.server.onerror != nil {
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
				panic(err)
			} else {
				h := r.Header

				//获取Sec-Websocket-Key头部
				websocketKey := h.Get("Sec-Websocket-Key")

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

				_, err := c.SendRaw(resp)
				if err != nil {
					panic(err)
				}

				c.handshaked = true
			}
		} else {
			var message []byte

			//解析websocket数据
			message, err := DecodeProto(c.Conn)
			if err != nil {
				if err == ErrConnClosed {
					if c.server.onclose != nil {
						c.server.onclose(c)
					}

					c.server.removeConn(c)

					c.Close()

					return
				} else {
					panic(err)
				}
			}
			c.data = bytes.NewReader(message)

			if c.server.onmessage != nil {
				c.server.onmessage(c)
			}
		}
	}
}

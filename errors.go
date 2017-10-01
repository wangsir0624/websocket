package websocket

import (
	"errors"
)

var (
	//当收到此错误消息时，服务器应该关闭连接
	ErrConnClosed = errors.New("the connection was closed by the client")

	//当收到此错误消息时，服务器应该给客户端返回一个Pong响应
	ErrConnPing = errors.New("Ping from the client")
)

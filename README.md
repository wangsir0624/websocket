# websocket

## 安装
```
go get github.com/wangsir0624/websocket
```

## 用法

###基本用法
```
package main

import (
	"fmt"
	"websocket"
)

func main() {
	//监听IP和端口
	server := websocket.Listen("127.0.0.1", 11111)
	
	//注册创建连接回调事件
	server.On("connection", func(conn *websocket.Conn) {
		fmt.Printf("accept the connection from %s\r\n", conn.RemoteAddr())
	})
	
	//注册错误回调函数
	server.On("error", func(conn *websocket.Conn) {
		fmt.Println(conn.GetErr())
	})
	
	//注册接受消息回调函数
	server.On("message", func(conn *websocket.Conn) {
		fmt.Printf("receive message from %s: %s\r\n", conn.RemoteAddr(), conn.GetData())

		conn.Send(conn.GetData())
	})
	
	//注册关闭连接回调函数
	server.On("close", func(conn *websocket.Conn) {
		fmt.Printf("the connection from %s was closed\r\n", conn.RemoteAddr())
	})
	
	//运行服务器
	server.Run()
}

```

### 广播
#### 对所有连接广播消息
```
server.On("message", func(conn *websocket.Conn) {
		fmt.Printf("receive message from %s: %s\r\n", conn.RemoteAddr(), conn.GetData())

		conn.GetServer().Broadcast(conn.GetData())
})
```

#### 对其他连接广播消息
```
server.On("message", func(conn *websocket.Conn) {
		fmt.Printf("receive message from %s: %s\r\n", conn.RemoteAddr(), conn.GetData())

		conn.GetServer().BroadcastToOthers(conn.GetData(), conn)

		//方法二
		conn.GetServer().BroadcastOnly(conn.GetData(), func(c *websocket.Conn) bool {
			return c.RemoteAddr().String() != conn.RemoteAddr().String()
		})

		//方法三
		conn.GetServer().BroadcastExcept(conn.GetData(), func(c *websocket.Conn) bool {
			return c.RemoteAddr().String() == conn.RemoteAddr().String()
		})
})
```


## 服务器状态监控
本服务器采用HTTP协议的方式来监控服务器状态，监听端口为websocket端口加1，比如上面的例子中websocket端口为11111，那么http端口即为11112，在浏览器中输入http://127.0.0.1:11112，就可以监听服务器运行状态了
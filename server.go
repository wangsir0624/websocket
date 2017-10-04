package websocket

import (
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	BUF_SIZE     = 1024     //每次从连接读取的最大字节数
	MAX_BUF_SIZE = 10280960 //接受的最大长度
)

type Server struct {
	listener *net.TCPListener

	connections map[string]*Conn
	connMutex   *sync.Mutex

	onconnection func(conn *Conn)
	onmessage    func(conn *Conn)
	onerror      func(conn *Conn)
	onclose      func(conn *Conn)
}

//监听IP和端口
func Listen(ip string, port int) *Server {
	var server = new(Server)

	addr, err := net.ResolveTCPAddr("tcp", ip+":"+strconv.Itoa(port))
	if err != nil {
		panic(err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}

	server.listener = listener
	server.connections = make(map[string]*Conn)
	server.connMutex = new(sync.Mutex)

	return server
}

//给服务器注册回调事件
//事件主要有以下四个
//connection 在连接成功时触发
//message 在接受到客户端消息时触发
//error 在发生错误时触发
//close 在关闭连接时触发
//回调函数均接受一个Conn结构体指针作为参数，无返回值
func (s *Server) On(event string, callback func(conn *Conn)) bool {
	switch event {
	case "connection":
		s.onconnection = callback
	case "message":
		s.onmessage = callback
	case "error":
		s.onerror = callback
	case "close":
		s.onclose = callback
	default:
		return false
	}

	return true
}

//开始运行服务器
func (s *Server) Run() {
	go func() {
		for {
			conn, err := s.accept()
			if err != nil {
				continue
			}

			s.addConn(conn)

			if s.onconnection != nil {
				s.onconnection(conn)
			}

			go conn.handleData()
		}
	}()

	for {
		time.Sleep(10 * time.Second)
	}
}

//给所有连接广播一个消息
func (s *Server) Broadcast(msg []byte) {
	for _, c := range s.connections {
		_, err := c.Send(msg)
		if err != nil {
			continue
		}
	}
}

//给其他连接广播一个消息
//第二个参数为一个Conn结构体指针，除了此连接，其他连接都会广播此消息
func (s *Server) BroadcastToOthers(msg []byte, conn *Conn) {
	for _, c := range s.connections {
		if c == conn {
			continue
		}

		_, err := c.Send(msg)
		if err != nil {
			continue
		}
	}
}

//仅仅给回调函数返回true的连接广播消息
func (s *Server) BroadcastOnly(msg []byte, only func(conn *Conn) bool) {
	for _, c := range s.connections {
		if only(c) {
			_, err := c.Send(msg)
			if err != nil {
				continue
			}
		}
	}
}

//仅仅给回调函数返回false的连接广播消息
func (s *Server) BroadcastExcept(msg []byte, except func(conn *Conn) bool) {
	for _, c := range s.connections {
		if !except(c) {
			_, err := c.Send(msg)
			if err != nil {
				continue
			}
		}
	}
}

//接受客户端连接
//函数会阻塞，直到有连接来临
func (s *Server) accept() (*Conn, error) {
	tcpConn, err := s.listener.Accept()
	if err != nil {
		return nil, err
	}

	conn := new(Conn)
	conn.Conn = tcpConn
	conn.server = s
	conn.handshaked = false

	return conn, nil
}

//添加一个连接
func (s *Server) addConn(conn *Conn) {
	s.connMutex.Lock()
	s.connections[conn.RemoteAddr().String()] = conn
	s.connMutex.Unlock()
}

//移除一个连接
func (s *Server) removeConn(conn *Conn) {
	s.connMutex.Lock()
	delete(s.connections, conn.RemoteAddr().String())
	s.connMutex.Unlock()
}

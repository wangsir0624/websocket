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

func (s *Server) addConn(conn *Conn) {
	s.connMutex.Lock()
	s.connections[conn.RemoteAddr().String()] = conn
	s.connMutex.Unlock()
}

func (s *Server) removeConn(conn *Conn) {
	s.connMutex.Lock()
	delete(s.connections, conn.RemoteAddr().String())
	s.connMutex.Unlock()
}

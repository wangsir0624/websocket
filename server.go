package websocket

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
)

const (
	BUF_SIZE     = 1024
	MAX_BUF_SIZE = 102809600
)

type Server struct {
	listener *net.TCPListener

	onconnection func()
	onmessage    func()
	onerror      func()
	onclose      func()
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

	return server
}

func (s *Server) On(event string, callback func()) bool {
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
	handler := func(conn net.Conn) {
		var buffer []byte
		var sep = []byte{'\r', '\n'}

		for {
			tmp := make([]byte, BUF_SIZE)

			_, err := conn.Read(buffer)
			if err != nil {

			}

			fmt.Println(bytes.TrimSpace(tmp))
			buffer = append(buffer, tmp...)
			if bytes.Contains(buffer, sep) {
				break
			}
		}

		fmt.Println('\r', '\n')

		loc := bytes.Index(buffer, sep)
		message := buffer[:(loc - 1)]
		conn.Write(message)
	}

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			continue
		}

		handler(conn)
		conn.Close()
	}
}

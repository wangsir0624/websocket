package websocket

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	BUF_SIZE     = 1024     //每次从连接读取的最大字节数
	MAX_BUF_SIZE = 10280960 //接受的最大长度
	STATUS_TMPL  = `<!DOCTYPE HTML>
<html>
<head>
<meta charset="UTF-8" />
<title>websocket服务器状态监控</title>
<style>
td {width: 200px;}
</style>
</head>
<body>
<table>
<tr><td>PID:</td><td id="pid">%d<td></tr>
<tr><td>运行时间:</td><td id="runtime">%s<td></tr>
<tr><td>当前连接数:</td><td id="current_connections">%d<td></tr>
<tr><td>峰值连接数:</td><td id="peak_connections">%d<td></tr>
</table>
<script type="text/javascript">
function createAjax() {
	var xmlhttp;

	if (window.XMLHttpRequest) {
		xmlhttp=new XMLHttpRequest();
	}
	else {
		xmlhttp=new ActiveXObject("Microsoft.XMLHTTP");
	}

	return xmlhttp;
}

function flushStatus() {
	var xmlhttp = createAjax();
	xmlhttp.onreadystatechange = function() {
		if (xmlhttp.readyState == 4 && xmlhttp.status == 200) {
			var status = JSON.parse(xmlhttp.responseText);
			
			document.getElementById("pid").innerText = status.Pid;
			document.getElementById("runtime").innerText = status.Runtime;
			document.getElementById("current_connections").innerText = status.CurrentConnections;
			document.getElementById("peak_connections").innerText = status.PeakConnections;
		}
	}

	xmlhttp.open("GET", "http://%s:%d/status/params", true);
	xmlhttp.send(null);
}

window.setInterval(flushStatus, 1000);
</script>
</body>
</html>
	`
)

type serverStatus struct {
	Pid                int
	Runtime            string
	CurrentConnections int
	PeakConnections    int
}

type Server struct {
	ip   string
	port int

	startTime time.Time //服务器开始运行的时间

	listener *net.TCPListener

	connections        map[string]*Conn
	connMutex          *sync.Mutex
	currentConnections int //当前活跃的连接数
	peakConnections    int //峰值连接数

	onconnection func(conn *Conn)
	onmessage    func(conn *Conn)
	onerror      func(conn *Conn)
	onclose      func(conn *Conn)
}

//监听IP和端口
func Listen(ip string, port int) *Server {
	var server = new(Server)

	server.ip = ip
	server.port = port

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
	s.startTime = time.Now()

	go func() {
		for {
			conn, err := s.accept()
			if err != nil {
				continue
			}

			s.addConn(conn)

			go conn.handleData()
		}
	}()

	http.HandleFunc("/", s.showServerStatus)
	http.HandleFunc("/status/params", s.getServerStatus)
	err := http.ListenAndServe(s.ip+":"+strconv.Itoa(s.port+1), nil)
	if err != nil {
		panic(err)
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
	s.currentConnections++

	if s.currentConnections > s.peakConnections {
		s.peakConnections = s.currentConnections
	}

	s.connMutex.Unlock()
}

//移除一个连接
func (s *Server) removeConn(conn *Conn) {
	s.connMutex.Lock()
	delete(s.connections, conn.RemoteAddr().String())
	s.currentConnections--
	s.connMutex.Unlock()
}

//显示服务器状态信息
func (s *Server) showServerStatus(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(fmt.Sprintf(STATUS_TMPL, os.Getpid(), formatTime(time.Since(s.startTime)), s.currentConnections, s.peakConnections, s.ip, s.port+1)))
}

//获取服务器状态参数
func (s *Server) getServerStatus(w http.ResponseWriter, r *http.Request) {
	status := new(serverStatus)
	status.Pid = os.Getpid()
	status.Runtime = formatTime(time.Since(s.startTime))
	status.CurrentConnections = s.currentConnections
	status.PeakConnections = s.peakConnections

	encoder := json.NewEncoder(w)
	encoder.Encode(status)
}

func formatTime(d time.Duration) (result string) {
	seconds := d.Nanoseconds()

	h := seconds / int64(time.Hour)

	seconds -= h * int64(time.Hour)
	m := seconds / int64(time.Minute)

	seconds -= m * int64(time.Minute)
	s := seconds / int64(time.Second)

	if h > 0 {
		result += strconv.Itoa(int(h)) + "h"
	}

	if m > 0 {
		result += strconv.Itoa(int(m)) + "m"
	}

	if s > 0 {
		result += strconv.Itoa(int(s)) + "s"
	}

	return result
}

package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"test/comm"

	"github.com/creack/pty"
	"github.com/pkg/term/termios"
	"golang.org/x/sys/unix"
)

var f *os.File

// 客户端发送给服务端需要分包, 因为有窗口大小改变事件和一般流的传输
var tobeWritten chan []byte

// 监听窗口大小改变, 然后把分包写入tobeWritten
func winchMonitor() {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGWINCH)
	for range c {
		w, h, err := pty.Getsize(os.Stdin)
		if err != nil {
			panic(err)
		}
		req := comm.Request{}
		req.Type = "winch"
		req.Row = w
		req.Col = h
		b, _ := json.Marshal(&req)
		tobeWritten <- b
	}
}

// 从tobeWriiten读取, 写入对端socket
func connWriter(conn net.Conn) {
	for v := range tobeWritten {
		bytes := comm.MakePackageBytes(v)
		_, err := conn.Write(bytes)
		if err != nil {
			break
		}
	}
}

// 从conn读, 不断写入stdout
func connReaderCopyToStdout(conn net.Conn) {
	buf := make([]byte, 256, 256)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			break
		}
		f.Write([]byte(fmt.Sprintf("%d %s\n", n, buf[:n])))
		_, _ = os.Stdout.Write(buf[:n])
	}
}

// 从stdin读取, 分包写入tobeWritten
func stdinReader() {
	for {
		f.Write([]byte("reader loop\n"))
		buf := make([]byte, 256, 256)
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return
		}
		req := comm.Request{}
		req.Type = "common"
		req.Bytes = string(buf[:n])
		b, _ := json.Marshal(&req)
		tobeWritten <- b
	}
}

func main() {
	f, _ = os.Create("./debug")

	tobeWritten = make(chan []byte, 10)
	conn, err := net.Dial("tcp", "127.0.0.1:8081")
	if err != nil {
		fmt.Printf("failed to dial remote server: %v\n", err)
		return
	}

	oldTermiosStruct := unix.Termios{}
	err = termios.Tcgetattr(0, &oldTermiosStruct)
	if err != nil {
		panic(err)
	}

	newTeriosStruct := oldTermiosStruct
	termios.Cfmakeraw(&newTeriosStruct)
	err = termios.Tcsetattr(0, termios.TCSANOW, &newTeriosStruct)
	if err != nil {
		panic(err)
	}

	rowCol := make([]byte, 0)
	w, h, err := pty.Getsize(os.Stdin)
	if err != nil {
		panic(err)
	}
	rowCol = binary.BigEndian.AppendUint32(rowCol, uint32(w))
	rowCol = binary.BigEndian.AppendUint32(rowCol, uint32(h))
	conn.Write(rowCol)

	defer func() {
		termios.Tcsetattr(0, termios.TCSANOW, &oldTermiosStruct)
		fmt.Printf("connection closed\n")
	}()

	// 从conn读取, 写入stdout
	go connReaderCopyToStdout(conn)

	// 从stdin读取, 写入tobeWritten
	go stdinReader()

	// 捕获信号, 写入tobeWritten
	go winchMonitor()

	// 将tobeWritten的内容发送
	connWriter(conn)
}

package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"test/comm"

	"github.com/creack/pty"
)

// 将pty的内容拷贝到conn
func ptyReaderAndconnWriter(conn net.Conn, pmux *os.File) {
	io.Copy(conn, pmux)
}

// 将conn读到的内容解包, 然后写入pty
func connReaderAndPtyWriter(conn net.Conn, pmux *os.File) {
	for {
		packLengthBytes := make([]byte, 4, 4)
		_, err := conn.Read(packLengthBytes)
		if err != nil {
			return
		}

		packLength := binary.BigEndian.Uint32(packLengthBytes)
		data := make([]byte, packLength, packLength)
		_, err = io.ReadFull(conn, data)
		if err != nil {
			return
		}

		req := comm.Request{}
		if json.Unmarshal(data, &req) != nil {
			return
		}

		if req.Type == "common" {
			pmux.Write([]byte(req.Bytes))
		}
		if req.Type == "winch" {
			pty.Setsize(pmux, &pty.Winsize{Rows: uint16(req.Row), Cols: uint16(req.Col)})
		}
	}
}

func main() {
	fmt.Println("Launching server...")

	// listen on all interfaces
	ln, err := net.Listen("tcp", ":8081")
	if err != nil {
		panic(err)
	}

	for {
		// accept connection on port
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("failed to Accept: %v\n", err)
		}
		println("接收到一个连接")

		go func(conn net.Conn) {
			defer conn.Close()
			// 前两个uint16为row和col
			packWinSizeBytes := make([]byte, 8, 8)
			_, err := io.ReadFull(conn, packWinSizeBytes)
			if err != nil {
				return
			}

			c := exec.Command("/bin/bash")
			row := uint16(binary.BigEndian.Uint32(packWinSizeBytes[:4]))
			col := uint16(binary.BigEndian.Uint32(packWinSizeBytes[4:]))
			ptmx, err := pty.StartWithSize(c, &pty.Winsize{
				Rows: row,
				Cols: col,
			})
			if err != nil {
				return
			}
			defer ptmx.Close()

			go connReaderAndPtyWriter(conn, ptmx)

			ptyReaderAndconnWriter(conn, ptmx)
		}(conn)
	}
}

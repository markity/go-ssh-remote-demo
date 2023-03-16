package comm

import (
	"bytes"
	"encoding/binary"
)

// 由client方发送的请求, 有common和winch两种
type Request struct {
	Type  string `json:"type"`
	Row   int    `json:"row"`
	Col   int    `json:"col"`
	Bytes string `json:"bytes"`
}

func MakePackageBytes(pack []byte) []byte {
	buf := bytes.Buffer{}

	// 前面四个字节为包的大小
	binary.Write(&buf, binary.BigEndian, uint32(len(pack)))

	buf.Write(pack)

	return buf.Bytes()
}

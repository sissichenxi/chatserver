package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/golang/protobuf/proto"

	"protocol"
)

func main() {
	localaddr, err := net.ResolveTCPAddr("tcp", ":6667")
	serveraddr, err := net.ResolveTCPAddr("tcp", ":8080")
	conn, err := net.DialTCP("tcp", localaddr, serveraddr)
	if err != nil {
		panic(err)
	}
	fmt.Println("succeed connected to server")
	defer conn.Close()
	go readfromServer(conn)
	reader := bufio.NewReader(os.Stdin)
	for {
		loginReq := &protocol.LoginRequest{}
		err := binary.Read(reader, binary.LittleEndian, &loginReq.UserID)
		if err != nil {
			fmt.Println("wrong user id")
			continue
		}

		data, err := proto.Marshal(loginReq)
		if err != nil {
			fmt.Println("wrong user id")
			continue
		}

		size := len(data)
		out := bytes.NewBuffer(make([]byte, 2+size))
		if err := binary.Write(out, binary.LittleEndian, size); err != nil {
			// TODO
		}
		if err := binary.Write(out, binary.LittleEndian, data); err != nil {
			// TODO
		}

		_, err = conn.Write(out.Bytes())
		if err != nil {
			fmt.Println("failed to write to server")
			return
		}
	}
}
func readfromServer(conn net.Conn) {
	defer conn.Close()
	for {
		datastr := make(map[int]string)
		data := make([]byte, 1024)
		_, err := conn.Read(data)

		if err != nil {
			fmt.Println("Read server Wrong" + "\n")
			return
		}
		if err = json.Unmarshal(data, datastr); err != nil {
			fmt.Println(string(data))
		} else {
			fmt.Println(datastr)
		}

	}
}

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"

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
	for {
		reader := bufio.NewReader(os.Stdin)
		input, _, err := reader.ReadLine()
		//string(input[0 : len(input)-1])

		if err != nil {
			fmt.Println("fail to read ")
			continue
		}
		if len(input) > 0 {
			loginReq := &protocol.LoginRequest{}
			loginReq.UserID = strconv.Atoi(input)

			data, err := proto.Marshal(loginReq)
			fmt.Println(packed)

			// loginReq
			size := len(packed)
			b := make([]byte, 2+size)
			buffer := bytes.NewBuffer(b)
			buffer.Write(size)
			buffer.Write(packed)
			_, err := conn.Write(buffer)
			if err != nil {
				fmt.Println("failed to write to server")
				return
			}
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

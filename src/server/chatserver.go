package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/golang/protobuf/proto"

	"protocol"
)

func main() {
	link, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("tcp chatserver listen failed!")
		return
	}
	fmt.Println("tcp chatserver listen start!")
	//消息管道
	msgChannel := make(map[int]chan map[int]string)

	for {
		conn, err := link.Accept()
		if err != nil {
			continue
		}
		go handleConnection(conn, msgChannel)
	}
}

const buflen = 1024

//var userconnmap map[int]net.Conn = make(map[int]net.Conn, 10)

func handleConnection(conn net.Conn, msgchannel map[int]chan map[int]string) {
	//defer conn.Close()

	var currUserID int
	var tgtUserID int
	defer func() {
		fmt.Println(" conn closed")
		conn.Close()
		fmt.Printf("delete userid [%v] from msgChannel", currUserID)
		if currUserID > 0 {
			delete(msgchannel, currUserID)
		}

	}()

	var close = make(chan bool)
	//login := false
	fmt.Println("please input your userid:")
	if _, err := conn.Write([]byte("please input your userid: ")); err != nil {
		return
	}

	for {

		//data := make([]byte, 0)
		buf := make([]byte, buflen)
		n, err := conn.Read(buf)
		if err != nil {
			panic(err)
		}

		strUserid := string(buf[:n])
		nUserid, err := strconv.Atoi(strUserid)
		if nUserid < 1 || err != nil {
			fmt.Println("incorrect userid")
			continue
		}
		currUserID = nUserid
		msgchannel[nUserid] = make(chan map[int]string)
		if _, err := conn.Write([]byte("you have successfully set your userid: " + strUserid)); err != nil {
			return
		}
		break
	}
	fmt.Println("please input your target userid:" + "\n")
	if _, err := conn.Write([]byte("please input your target userid: ")); err != nil {
		return
	}

	for {
		//data := make([]byte, 0)
		buf := make([]byte, buflen)
		n, err := conn.Read(buf)
		if err != nil {
			panic(err)
		}
		strtgtUserID := string(buf[:n])
		ntgtUserID, err := strconv.Atoi(strtgtUserID)
		if ntgtUserID < 1 || err != nil {
			fmt.Println("incorrect target userid")
			continue
		}
		//msgchannel[ntgtUserid] = make(chan string)
		if _, has := msgchannel[ntgtUserID]; !has {
			fmt.Println("tgtUserID is not online")
			if _, err := conn.Write([]byte("tgtUserID is not online ")); err != nil {
				return
			}
			continue
		}
		fmt.Printf("init: %d\n", tgtUserID)
		tgtUserID = ntgtUserID
		break
	}

	go func() {
		fmt.Printf("%d\n", tgtUserID)
		fmt.Printf("%d\n", currUserID)
		for {
			head := make([]byte, 2)
			n, err := conn.Read(data)

			size := tonumber(head)
			data := make([]byte, size)
			n, err := conn.Read(data)
			if err != nil {
				fmt.Println("read data wrong:", err)
				close <- true
				continue
			}

			loginReq := &protocol.LoginRequest{}
			err = proto.Unmarshal(data, loginReq)

			fmt.Println(loginReq.UserID)
			datastr := string(data[:n])
			//fmt.Println(string(currUserID) + "send to " + string(tgtUserID) + ": " + datastr)
			fmt.Printf("%d send to %d : %s\n", currUserID, tgtUserID, datastr)
			if _, ok := msgchannel[tgtUserID]; ok {
				msgchannel[tgtUserID] <- map[int]string{currUserID: datastr}
			} else {
				fmt.Println("tgtUserID is not online")
			}
		}
	}()

	go func() {
		for {
			datamap := <-msgchannel[currUserID]
			jsonstring, err := json.Marshal(datamap)
			if err != nil {
				panic(err)
			}
			_, err = conn.Write(jsonstring)
			if err != nil {
				close <- true
			}
		}

	}()
	for {
		if <-close {
			return
		}
	}
}

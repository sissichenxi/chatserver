package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/golang/protobuf/proto"

	"chatserver/chatserver/src/protocol"
)

func main() {
	// localaddr, err := net.ResolveTCPAddr("tcp", ":6667")
	// serveraddr, err := net.ResolveTCPAddr("tcp", ":8080")
	// conn, err := net.DialTCP("tcp", localaddr, serveraddr)
	conn, err := net.Dial("tcp", ":8080")

	if err != nil {
		panic(err)
	}
	fmt.Println("succeed connected to server")
	defer conn.Close()
	go readfromServer(conn)
	reader := bufio.NewReader(os.Stdin)

	for {
		//read msgtype
		fmt.Println("please input msg type")
		msgtype, _, err := reader.ReadLine()
		if err != nil {
			fmt.Println("read msgtype wrong!")
		}

		str := string(msgtype)
		fmt.Println(str)
		var msgType protocol.MsgType
		switch str {
		case "LI":
			fmt.Println("please inout userid")
			msgType = protocol.MsgType_MT_LOGIN_REQUEST
			loginReq := &protocol.LoginRequest{}
			userid, _, err := reader.ReadLine()
			if err != nil {
				fmt.Println("read msgtype wrong!")
			}

			userIDint, _ := strconv.Atoi(string(userid))
			loginReq.UserID = int64(userIDint)
			fmt.Printf("userid is %d\n", loginReq.UserID)

			data, err := proto.Marshal(loginReq)
			fmt.Println(string(data))
			if err != nil {
				fmt.Println("wrong user id")
				continue
			}

			size := len(data)
			fmt.Println(size)
			out := bytes.NewBuffer(make([]byte, 0, 4+size))
			if err := binary.Write(out, binary.LittleEndian, uint16(msgType)); err != nil {
				fmt.Println("failed to write msgtype to buffer")
			}
			if err := binary.Write(out, binary.LittleEndian, uint16(size)); err != nil {
				fmt.Println("failed to write msgsize to buffer")
			}
			if err := binary.Write(out, binary.LittleEndian, data); err != nil {
				fmt.Println("failed to write msgbody to buffer")
			}
			//fmt.Println(out.Bytes())
			n, err := conn.Write(out.Bytes())
			fmt.Printf("sent %d bytes to server\n", n)
			if err != nil {
				fmt.Println("failed to write to server")
				return
			}
		case "PC":
			fmt.Println("please input target userid")
			msgType = protocol.MsgType_MT_PRIVCHAT_REQUEST
			privChatReq := &protocol.PrivateChatRequest{}
			userid, _, err := reader.ReadLine()
			if err != nil {
				fmt.Println("read msgtype wrong!")
			}
			userIDint, _ := strconv.Atoi(string(userid))
			privChatReq.Target = int64(userIDint)
			fmt.Printf("userid is %d\n", privChatReq.Target)

			privChatReq.Content, err = reader.ReadString('\n')
			fmt.Println(privChatReq.Content)

			data, err := proto.Marshal(privChatReq)
			if err != nil {
				fmt.Println("wrong user id")
				continue
			}

			size := len(data)
			out := bytes.NewBuffer(make([]byte, 0, 4+size))
			if err := binary.Write(out, binary.LittleEndian, uint16(msgType)); err != nil {
				// TODO
			}
			if err := binary.Write(out, binary.LittleEndian, uint16(size)); err != nil {
				// TODO
			}
			if err := binary.Write(out, binary.LittleEndian, data); err != nil {
				// TODO
			}

			n, err := conn.Write(out.Bytes())
			fmt.Printf("sent %d bytes to server\n", n)
			if err != nil {
				fmt.Println("failed to write to server")
				return
			}
		case "RC":
			fmt.Println("please input new room name")
			msgType = protocol.MsgType_MT_RMCREAT_REQUEST
			rmCreatReq := &protocol.RoomCreateRequest{}
			rmCreatReq.Name, err = reader.ReadString('\n')
			fmt.Println(rmCreatReq.Name)
			data, err := proto.Marshal(rmCreatReq)
			if err != nil {
				fmt.Println("wrong user id")
				continue
			}
			size := len(data)
			out := bytes.NewBuffer(make([]byte, 0, 4+size))
			if err := binary.Write(out, binary.LittleEndian, uint16(msgType)); err != nil {
				// TODO
			}
			if err := binary.Write(out, binary.LittleEndian, uint16(size)); err != nil {
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
		case "RJ":
			fmt.Println("please input room id")
			msgType = protocol.MsgType_MT_RMJOIN_REQUEST
			rmJoinReq := &protocol.RoomJoinRequest{}
			userid, _, err := reader.ReadLine()
			if err != nil {
				fmt.Println("read msgtype wrong!")
			}
			userIDint, _ := strconv.Atoi(string(userid))
			rmJoinReq.RoomID = int64(userIDint)
			fmt.Printf("userid is %d\n", rmJoinReq.RoomID)

			data, err := proto.Marshal(rmJoinReq)
			if err != nil {
				fmt.Println("wrong user id")
				continue
			}
			size := len(data)
			out := bytes.NewBuffer(make([]byte, 0, 4+size))
			if err := binary.Write(out, binary.LittleEndian, uint16(msgType)); err != nil {
				// TODO
			}
			if err := binary.Write(out, binary.LittleEndian, uint16(size)); err != nil {
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
		case "GC":
			fmt.Println("please input roomid")
			msgType = protocol.MsgType_MT_RMCHAT_REQUSET
			rmChatReq := &protocol.RoomChatRequest{}
			userid, _, err := reader.ReadLine()
			if err != nil {
				fmt.Println("read msgtype wrong!")
			}
			userIDint, _ := strconv.Atoi(string(userid))
			rmChatReq.RoomID = int64(userIDint)
			fmt.Printf("userid is %d\n", rmChatReq.RoomID)

			fmt.Println("please input msg content")
			rmChatReq.Content, err = reader.ReadString('\n')
			fmt.Println(rmChatReq.Content)

			data, err := proto.Marshal(rmChatReq)
			if err != nil {
				fmt.Println("wrong user id")
				continue
			}

			size := len(data)
			out := bytes.NewBuffer(make([]byte, 0, 4+size))
			if err := binary.Write(out, binary.LittleEndian, uint16(msgType)); err != nil {
				// TODO
			}
			if err := binary.Write(out, binary.LittleEndian, uint16(size)); err != nil {
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
		default:
			fmt.Println("wrong type ! ")
		}

	}
}
func readfromServer(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		//read message type
		mtype := make([]byte, 2)
		if err := binary.Read(reader, binary.LittleEndian, mtype); err != nil {
			fmt.Println("read msg type wrong!")
			return
		}
		var msgtype protocol.MsgType
		var msgtypeint16 int16
		typebuf := bytes.NewBuffer(mtype)
		if err := binary.Read(typebuf, binary.LittleEndian, &msgtypeint16); err != nil {
			fmt.Println("read msgtype wrong!")
			return
		}
		msgtype = protocol.MsgType(msgtypeint16)
		fmt.Printf("received msgtype %d\n", msgtype)
		//read message size
		head := make([]byte, 2)
		if err := binary.Read(reader, binary.LittleEndian, head); err != nil {
			fmt.Println("read msg len wrong!")
			return
		}
		var size int
		var sizeint16 int16
		buf := bytes.NewBuffer(head)
		if err := binary.Read(buf, binary.LittleEndian, &sizeint16); err != nil {
			fmt.Println("read msg body wrong!")
			return
		}
		size = int(sizeint16)
		fmt.Printf("received msg size %d\n", size)
		//read message
		msgbody := make([]byte, size)
		if err := binary.Read(reader, binary.LittleEndian, msgbody); err != nil {
			fmt.Println("read msg body byte wrong!")
			return
		}
		switch msgtype {
		case protocol.MsgType_MT_LOGIN_RESPONSE:
			loginResp := &protocol.LoginResponse{}
			if err := proto.Unmarshal(msgbody, loginResp); err != nil {
				fmt.Println("unmarshal response failed!")
			} else {
				fmt.Printf("log in response with code:%d\n", loginResp.Ec)
			}

		case protocol.MsgType_MT_PRIVCHAT_RESPONSE:
			privChatResp := &protocol.PrivateChatResponse{}
			if err := proto.Unmarshal(msgbody, privChatResp); err != nil {
				fmt.Println("unmarshal response failed!")
			} else {
				fmt.Printf("privatechat response with code:%d\n", privChatResp.Ec)
			}

		case protocol.MsgType_MT_RMCREAT_RESPOSE:
			rmCreatResp := &protocol.RoomCreateResponse{}
			if err := proto.Unmarshal(msgbody, rmCreatResp); err != nil {
				fmt.Println("unmarshal response failed!")
			} else {
				fmt.Printf("roomcreat response with code:%d\n", rmCreatResp.Ec)
				fmt.Printf("creatded room id :%d\n", rmCreatResp.RoomID)
			}
		case protocol.MsgType_MT_RMJOIN_RESPONSE:
			rmJoinResp := &protocol.RoomJoinResponse{}
			if err := proto.Unmarshal(msgbody, rmJoinResp); err != nil {
				fmt.Println("unmarshal response failed!")
			} else {
				fmt.Printf("roomjoin response with code:%d\n", rmJoinResp.Ec)
			}

		case protocol.MsgType_MT_RMCHAT_RESPONSE:
			rmChatResp := &protocol.RoomChatResponse{}
			if err := proto.Unmarshal(msgbody, rmChatResp); err != nil {
				fmt.Println("unmarshal response failed!")
			} else {
				fmt.Printf("roomchat response with code:%d\n", rmChatResp.Ec)
			}

		case protocol.MsgType_MT_PRIVCHAT_NOYIFY:
			privChatNotify := &protocol.PrivateChatNotify{}
			if err := proto.Unmarshal(msgbody, privChatNotify); err != nil {
				fmt.Println("unmarshal notify failed!")
			} else {
				fmt.Printf("receive from [%d] ", privChatNotify.Src)
				fmt.Println(privChatNotify.Content)
			}

		case protocol.MsgType_MT_RMCHAT_NOTIFY:
			roomChatNotify := &protocol.RoomChatNotify{}
			if err := proto.Unmarshal(msgbody, roomChatNotify); err != nil {
				fmt.Println("unmarshal notify failed!")
			} else {
				fmt.Printf("receive from [%d] ", roomChatNotify.UserID)
				fmt.Printf("in group[%d]:", roomChatNotify.RoomID)
				fmt.Println(roomChatNotify.Content)
			}
		default:
			//type error
			fmt.Println("wrong msg type!")
		}
	}
}

package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"

	"chatserver/chatserver/src/protocol"
)

type ErrorCode int

const (
	CmdOK ErrorCode = iota
	CmdTooFewArgs
	CmdTooManyArgs
	CmdWrongTypeArgs
	CmdInvalid
	CmdOffline
)

type PCmdFunc func(string, *bool) (protocol.MsgType, []byte, ErrorCode, error)
type PMsgFunc func([]byte, *bool)

var (
	cmdfuncs = make(map[string]PCmdFunc)
	msgfuncs = make(map[protocol.MsgType]PMsgFunc)
)

func Stack() string {
	b := bytes.NewBuffer(make([]byte, 4096))
	runtime.Stack(b.Bytes(), false)
	return b.String()
}

func init() {
	cmdfuncs["login"] = login
	cmdfuncs["chat"] = chat
	cmdfuncs["rmcreat"] = rmcreat
	cmdfuncs["rmjoin"] = rmjoin
	cmdfuncs["rmchat"] = rmchat
	msgfuncs[protocol.MsgType_MT_LOGIN_RESPONSE] = loginParse
	msgfuncs[protocol.MsgType_MT_PRIVCHAT_RESPONSE] = chatRespParse
	msgfuncs[protocol.MsgType_MT_RMCREAT_RESPOSE] = rmCreatParse
	msgfuncs[protocol.MsgType_MT_RMJOIN_RESPONSE] = rmJoinParse
	msgfuncs[protocol.MsgType_MT_RMCHAT_RESPONSE] = rmChatRespParse
	msgfuncs[protocol.MsgType_MT_PRIVCHAT_NOYIFY] = chatNotifyParse
	msgfuncs[protocol.MsgType_MT_RMCHAT_NOTIFY] = rmChatNotifyParse
	msgfuncs[protocol.MsgType_MT_OFFLINE_NOYIFY] = offlineNotifyParse
}
func main() {

	conn, err := net.Dial("tcp", ":8080")
	if err != nil {
		panic(err)
	}
	fmt.Println("succeed connected to server")
	defer conn.Close()
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err.(error))
			fmt.Println(Stack())
		}
	}()
	offline := true
	go readfromServer(conn, &offline)
	reader := bufio.NewReader(os.Stdin)
	for {
		msg, _, err := reader.ReadLine()
		if err != nil {
			panic(err)
		}
		msgstr := string(msg)
		line := strings.SplitN(msgstr, " ", 2)
		if len(line) != 2 {
			fmt.Println("too few args!")
			continue
		}

		f, ok := cmdfuncs[line[0]]
		if !ok {
			fmt.Println("invalid cmd!")
			continue
		}

		msgtype, data, ec, err := f(line[1], &offline)
		if err != nil {
			panic(err)
		}
		if ec != CmdOK {
			handleEC(ec)
			continue
		}

		tosend, err := writeBuffer(msgtype, data)
		if err != nil {
			panic(err)
		}

		_, err = conn.Write(tosend)
		if err != nil {
			fmt.Println("failed to write to server")
			return
		}
	}
}

func writeBuffer(msgtype protocol.MsgType, msg []byte) ([]byte, error) {
	var err error
	size := len(msg)
	buf := bytes.NewBuffer(make([]byte, 0, 4+size))
	if err = binary.Write(buf, binary.LittleEndian, uint16(msgtype)); err != nil {
		panic(err)
	}
	if err = binary.Write(buf, binary.LittleEndian, uint16(size)); err != nil {
		panic(err)
	}
	if err = binary.Write(buf, binary.LittleEndian, msg); err != nil {
		panic(err)
	}
	return buf.Bytes(), err
}

func readfromServer(conn net.Conn, offline *bool) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		msgtype, msgbody, err := readParse(reader)
		if err != nil {
			return
		}
		f, ok := msgfuncs[msgtype]
		if !ok {
			fmt.Println("undefined msg type!")
			continue
		}
		f(msgbody, offline)
	}
}
func readParse(reader *bufio.Reader) (protocol.MsgType, []byte, error) {
	var msgtype protocol.MsgType
	//read message size
	head := make([]byte, 2)
	if err := binary.Read(reader, binary.LittleEndian, head); err != nil {
		fmt.Println("read msg len wrong!")
		return msgtype, nil, err
	}
	var size int
	var sizeint16 int16
	buf := bytes.NewBuffer(head)
	if err := binary.Read(buf, binary.LittleEndian, &sizeint16); err != nil {
		fmt.Println("read msg body wrong!")
		return msgtype, nil, err
	}
	size = int(sizeint16)

	mtype := make([]byte, 2)
	if err := binary.Read(reader, binary.LittleEndian, mtype); err != nil {
		fmt.Println("read msg type wrong!")
		return msgtype, nil, err
	}

	var msgtypeint16 int16
	typebuf := bytes.NewBuffer(mtype)
	if err := binary.Read(typebuf, binary.LittleEndian, &msgtypeint16); err != nil {
		fmt.Println("read msgtype wrong!")
		return msgtype, nil, err
	}
	msgtype = protocol.MsgType(msgtypeint16)

	//fmt.Printf("received msg size %d\n", size)
	//read message
	msgbody := make([]byte, size)
	if err := binary.Read(reader, binary.LittleEndian, msgbody); err != nil {
		fmt.Println("read msg body byte wrong!")
		return msgtype, nil, err
	}
	return msgtype, msgbody, nil
}

func login(line string, offline *bool) (protocol.MsgType, []byte, ErrorCode, error) {
	ec := CmdOK
	var err error
	var data []byte
	msgType := protocol.MsgType_MT_LOGIN_REQUEST
	args := strings.Split(line, " ")
	if len(args) == 1 {
		loginReq := &protocol.LoginRequest{}
		nUserID, err := strconv.Atoi(args[0])
		if err != nil {
			return msgType, nil, CmdWrongTypeArgs, err
		}
		loginReq.UserID = int64(nUserID)
		data, err = proto.Marshal(loginReq)
	} else {
		ec = CmdTooManyArgs
	}
	return msgType, data, ec, err
}

func chat(line string, offline *bool) (protocol.MsgType, []byte, ErrorCode, error) {
	ec := CmdOK
	var err error
	var data []byte
	msgType := protocol.MsgType_MT_PRIVCHAT_REQUEST
	if *offline {
		return msgType, nil, CmdOffline, err
	}
	args := strings.SplitN(line, " ", 2)
	if len(args) == 2 {
		privChatReq := &protocol.PrivateChatRequest{}
		nUserID, err := strconv.Atoi(args[0])
		if err != nil {
			return msgType, nil, CmdWrongTypeArgs, err
		}
		privChatReq.Target = int64(nUserID)
		privChatReq.Content = args[1]
		data, err = proto.Marshal(privChatReq)
	} else {
		ec = CmdTooManyArgs
	}
	return msgType, data, ec, err
}

func rmcreat(line string, offline *bool) (protocol.MsgType, []byte, ErrorCode, error) {
	ec := CmdOK
	var err error
	var data []byte
	msgType := protocol.MsgType_MT_RMCREAT_REQUEST
	if *offline {
		return msgType, nil, CmdOffline, err
	}
	args := strings.Split(line, " ")
	if len(args) == 1 {
		rmCreatReq := &protocol.RoomCreateRequest{}
		rmCreatReq.Name = args[0]
		data, err = proto.Marshal(rmCreatReq)
	} else {
		ec = CmdTooManyArgs
	}
	return msgType, data, ec, err
}
func rmjoin(line string, offline *bool) (protocol.MsgType, []byte, ErrorCode, error) {
	ec := CmdOK
	var err error
	var data []byte
	msgType := protocol.MsgType_MT_RMJOIN_REQUEST
	if *offline {
		return msgType, nil, CmdOffline, err
	}
	args := strings.Split(line, " ")
	if len(args) == 1 {
		rmJoinReq := &protocol.RoomJoinRequest{}
		nRoomID, err := strconv.Atoi(args[0])
		if err != nil {
			return msgType, nil, CmdWrongTypeArgs, err
		}
		rmJoinReq.RoomID = int64(nRoomID)
		data, err = proto.Marshal(rmJoinReq)
	} else {
		ec = CmdTooManyArgs
	}
	return msgType, data, ec, err
}
func rmchat(line string, offline *bool) (protocol.MsgType, []byte, ErrorCode, error) {
	ec := CmdOK
	var err error
	var data []byte
	msgType := protocol.MsgType_MT_RMCHAT_REQUSET
	if *offline {
		return msgType, nil, CmdOffline, err
	}
	args := strings.SplitN(line, " ", 2)
	if len(args) == 2 {
		rmChatReq := &protocol.RoomChatRequest{}
		nRoomID, err := strconv.Atoi(args[0])
		if err != nil {
			return msgType, nil, CmdWrongTypeArgs, err
		}
		rmChatReq.RoomID = int64(nRoomID)
		rmChatReq.Content = args[1]
		data, err = proto.Marshal(rmChatReq)
	} else {
		ec = CmdTooFewArgs
	}
	return msgType, data, ec, err
}

func handleEC(ec ErrorCode) {
	switch ec {
	case CmdTooFewArgs:
		fmt.Println("two few args!")
	case CmdTooManyArgs:
		fmt.Println("two many args!")
	case CmdWrongTypeArgs:
		fmt.Println("wrong args type!")
	case CmdInvalid:
		fmt.Println("invalid cmd!!")
	case CmdOffline:
		fmt.Println("login first!!")
	}
}
func loginParse(msgbody []byte, offline *bool) {
	loginResp := &protocol.LoginResponse{}
	if err := proto.Unmarshal(msgbody, loginResp); err != nil {
		fmt.Println("unmarshal response failed!")
	} else {
		if loginResp.Ec == protocol.ErrorCode_EC_OK {
			fmt.Println("Log in OK")
			*offline = false
		} else {
			fmt.Println("Log in Failed")
		}
	}
}
func chatRespParse(msgbody []byte, offline *bool) {
	privChatResp := &protocol.PrivateChatResponse{}
	if err := proto.Unmarshal(msgbody, privChatResp); err != nil {
		fmt.Println("unmarshal response failed!")
	} else {
		if privChatResp.Ec == protocol.ErrorCode_EC_OK {
			fmt.Println("private chat OK")
		} else {
			fmt.Println("Target Not Found")
		}
	}
}
func rmCreatParse(msgbody []byte, offline *bool) {
	rmCreatResp := &protocol.RoomCreateResponse{}
	if err := proto.Unmarshal(msgbody, rmCreatResp); err != nil {
		fmt.Println("unmarshal response failed!")
	} else {
		if rmCreatResp.Ec == protocol.ErrorCode_EC_OK {
			fmt.Printf("creatded room %d OK \n", rmCreatResp.RoomID)
		} else {
			fmt.Printf("room create Fail with code:%d\n", rmCreatResp.Ec)
		}
	}
}
func rmJoinParse(msgbody []byte, offline *bool) {
	rmJoinResp := &protocol.RoomJoinResponse{}
	if err := proto.Unmarshal(msgbody, rmJoinResp); err != nil {
		fmt.Println("unmarshal response failed!")
	} else {
		if rmJoinResp.Ec == protocol.ErrorCode_EC_OK {
			fmt.Println("room join OK")
		} else {
			fmt.Println("room join Failed")
		}
	}
}
func rmChatRespParse(msgbody []byte, offline *bool) {
	rmChatResp := &protocol.RoomChatResponse{}
	if err := proto.Unmarshal(msgbody, rmChatResp); err != nil {
		fmt.Println("unmarshal response failed!")
	} else {
		if rmChatResp.Ec == protocol.ErrorCode_EC_OK {
			fmt.Println("room chat OK")
		} else {
			fmt.Println("room chat Failed")
		}
	}
}
func chatNotifyParse(msgbody []byte, offline *bool) {
	privChatNotify := &protocol.PrivateChatNotify{}
	if err := proto.Unmarshal(msgbody, privChatNotify); err != nil {
		fmt.Println("unmarshal notify failed!")
	} else {
		fmt.Printf(privChatNotify.Content+" from [%d]\n", privChatNotify.Src)
	}
}
func rmChatNotifyParse(msgbody []byte, offline *bool) {
	roomChatNotify := &protocol.RoomChatNotify{}
	if err := proto.Unmarshal(msgbody, roomChatNotify); err != nil {
		fmt.Println("unmarshal notify failed!")
	} else {
		fmt.Printf(roomChatNotify.Content+" from [%d] in room [%d]\n", roomChatNotify.UserID, roomChatNotify.RoomID)
	}
}
func offlineNotifyParse(msgbody []byte, offline *bool) {
	offlineNotify := &protocol.OfflineNotify{}
	if err := proto.Unmarshal(msgbody, offlineNotify); err != nil {
		fmt.Println("unmarshal notify failed!")
	} else {
		fmt.Printf("[%d] offline\n", offlineNotify.UserID)
		*offline = true
	}
}

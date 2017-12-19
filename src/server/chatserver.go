package main

//
//TCP msg fmt:
//the first 2 bytes represent msg type;
//the later 2 bytes represent msg length;
//then following with msg body
import (
	"bufio"
	"bytes"
	"chatserver/chatserver/src/protocol"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"gopkg.in/mgo.v2"
)

const maxRoomMember = 100

type IDgetter struct {
	roomid int64
	mu     sync.Mutex
}

func (idget *IDgetter) spanNextID() (nextid int64) {
	idget.mu.Lock()
	idget.roomid++
	nextid = idget.roomid
	idget.mu.Unlock()
	return
}

type User struct {
	//Id     bson.ObjectId `bson:"_id"`
	Userid int64  `bson:"userid"`
	Name   string `bson:"name"`
}

type ChatRoom struct {
	RoomID   int64   `bson:"roomid"`
	RoomName string  `bson:"roomname"`
	UserCnt  uint32  `bson:"usercnt"`
	UserIDs  []int64 `bson:userids`
}
type PrivOfflineMsg struct {
	Tgtid int64 `bson:"tgtid"`
	Msgid int64 `bson:"msgid"`
}
type PrivMsg struct {
	Tgtid     int64     `bson:"tgtid"`
	Senderid  int64     `bson:"senderid"`
	Msgid     int64     `bson:"msgid"`
	Time      time.Time `bson:"time"`
	MsgDetail string    `bson:"msgdetail"`
}
type RoomOfflineMsg struct {
	Userid int64 `bson:"userid"`
	Msgid  int64 `bson:"msgid"`
}
type RoomMsg struct {
	Roomid    int64     `bson:"roomid"`
	Senderid  int64     `bson:"senderid"`
	Msgid     int64     `bson:"msgid"`
	Time      time.Time `bson:"time"`
	MsgDetail string    `bson:"msgdetail"`
}

func main() {

	session, err := mgo.Dial("localhost")
	db := session.DB("chat")

	// c := db.C("users")
	// var users []User
	// c.Find(nil).All(&users)

	link, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("tcp chatserver listen failed!")
		return
	}
	fmt.Println("tcp chatserver listen start!")
	roomidgetter := new(IDgetter)
	//消息管道
	msgchnl := make(map[int64]chan []byte)
	offlinechan := make(map[int64]chan bool)
	chatrooms := make(map[int64]*ChatRoom)

	for {
		conn, err := link.Accept()
		if err != nil {
			continue
		}
		go handleConnection(conn, msgchnl, offlinechan, chatrooms, roomidgetter, db)
	}
}

//const buflen = 1024

//var userconnmap map[int]net.Conn = make(map[int]net.Conn, 10)

func handleConnection(conn net.Conn, msgchannel map[int64]chan []byte, offlinechan map[int64]chan bool,
	chatroom map[int64]*ChatRoom, idgetter *IDgetter, db *mgo.Database) {
	addr := conn.RemoteAddr()
	fmt.Printf("client %s:%s connected to server\n", addr.Network(), addr.String())
	var currUserID int64
	Login := false
	defer func() {
		fmt.Println(" conn closed")
		conn.Close()
		if currUserID > 0 {
			if _, exist := msgchannel[currUserID]; exist {
				delete(msgchannel, currUserID)
				fmt.Printf("delete userid [%v] from msgChannel", currUserID)
			}
			if _, exist := offlinechan[currUserID]; exist {
				delete(offlinechan, currUserID)
				fmt.Printf("delete userid [%v] from msgChannel", currUserID)
			}
		}
	}()

	var close = make(chan bool)

	reader := bufio.NewReader(conn)
	//read request routine
	go func() {

		for {
			data, msgtype, err := readmsg(reader, close)
			if err != nil {
				panic(err)
				//return
			}
			var respbody []byte
			var resptype protocol.MsgType

			//parse message body upon type
			switch msgtype {
			case protocol.MsgType_MT_LOGIN_REQUEST:
				respbody, currUserID, resptype, err = caseLoginReq(data, chatroom, &Login, offlinechan, msgchannel, conn)
				fmt.Println(currUserID)
			case protocol.MsgType_MT_PRIVCHAT_REQUEST:
				respbody, resptype, err = casePrivchatReq(data, currUserID, chatroom, msgchannel)
			case protocol.MsgType_MT_RMCREAT_REQUEST:
				respbody, resptype, err = caseRmCreateReq(data, currUserID, chatroom, msgchannel, idgetter)
			case protocol.MsgType_MT_RMJOIN_REQUEST:
				respbody, resptype, err = caseRmjoinReq(data, currUserID, chatroom, msgchannel)
			case protocol.MsgType_MT_RMCHAT_REQUSET:
				respbody, resptype, err = caseRmchatReq(data, currUserID, chatroom, msgchannel)
			default:
				fmt.Println("Wrong msg type!")
			}
			out, err := writeBuffer(resptype, respbody)
			n, err := conn.Write(out.Bytes())
			if err != nil {
				fmt.Println("conn write wrong!")
			}
			fmt.Printf("wrote %d bytes to client\n", n)
		}
	}()

	for {
		if <-close {
			return
		}
	}
}
func readmsg(reader *bufio.Reader, closechan chan bool) (data []byte, msgtype protocol.MsgType, err error) {
	//read message type
	mtype := make([]byte, 2)
	if err = binary.Read(reader, binary.LittleEndian, mtype); err != nil {
		fmt.Println("read msgtype bytes wrong!")
		closechan <- true
		return nil, msgtype, nil
	}
	fmt.Println("start read msg type...")
	var msgtype16 int16
	typebuf := bytes.NewBuffer(mtype)
	if err := binary.Read(typebuf, binary.LittleEndian, &msgtype16); err != nil {
		fmt.Println("read msgtype int wrong!")
		return nil, msgtype, err
	}

	fmt.Printf("received msgtype %d\n", msgtype16)
	msgtype = protocol.MsgType(msgtype16)
	//read message size
	head := make([]byte, 2)
	if err = binary.Read(reader, binary.LittleEndian, head); err != nil {
		fmt.Println("read msgsize bytes wrong!")
		return nil, msgtype, err
	}
	fmt.Println("start read msg size...")
	var size16 int16
	buf := bytes.NewBuffer(head)
	if err = binary.Read(buf, binary.LittleEndian, &size16); err != nil {
		fmt.Println("read msgsize int wrong!")
		return nil, msgtype, err
	}
	size := int(size16)
	fmt.Printf("received msg size %d\n", size)
	//read message
	d := make([]byte, size)
	if err = binary.Read(reader, binary.LittleEndian, d); err != nil {
		fmt.Println("read msgtype bytes wrong!")
		return nil, msgtype, err
	}
	return d, msgtype, err
}

func writeBuffer(notifytype protocol.MsgType, notifymsg []byte) (notifybuf *bytes.Buffer, err error) {
	size := len(notifymsg)
	notify := bytes.NewBuffer(make([]byte, 0, 4+size))
	if err = binary.Write(notify, binary.LittleEndian, uint16(notifytype)); err != nil {
		//TODO
	}
	if err = binary.Write(notify, binary.LittleEndian, uint16(size)); err != nil {
		//TODO
	}
	if err = binary.Write(notify, binary.LittleEndian, notifymsg); err != nil {
		//TODO
	}
	return notify, err
}

func casePrivchatReq(data []byte, currUserID int64, chatroom map[int64]*ChatRoom,
	msgchannel map[int64]chan []byte) (respbody []byte, resptype protocol.MsgType, err error) {
	privChatReq := &protocol.PrivateChatRequest{}
	privChatResp := &protocol.PrivateChatResponse{}
	privChatResp.Ec = protocol.ErrorCode_EC_OK
	if err = proto.Unmarshal(data, privChatReq); err != nil {
		fmt.Println("marshal private chat request failed!")
	}
	//to check if the tgt user exists from db

	privChatNotify := &protocol.PrivateChatNotify{}
	privChatNotify.Src = currUserID
	privChatNotify.Content = privChatReq.Content

	//target is online, send notify msg to channel

	if tgtchannel, online := msgchannel[privChatReq.Target]; online {
		if privChatReq.Target != currUserID {
			notifytype := protocol.MsgType_MT_PRIVCHAT_NOYIFY
			notifymsg, err := proto.Marshal(privChatNotify)
			if err != nil {
				fmt.Println("marshal faliled!")
			}
			notify, err := writeBuffer(notifytype, notifymsg)
			if err != nil {
				panic(err)
			}
			tgtchannel <- notify.Bytes()
		}

	} else {
		privChatResp.Ec = protocol.ErrorCode_EC_CHAT_NO_TARGET
	}

	//response info
	resptype = protocol.MsgType_MT_PRIVCHAT_RESPONSE
	respbody, err = proto.Marshal(privChatResp)
	if err != nil {
		fmt.Println("privChatResp marshal failed!")
	}
	return
}

func caseLoginReq(data []byte, chatroom map[int64]*ChatRoom, login *bool, offlinechan map[int64]chan bool,
	msgchannel map[int64]chan []byte, conn net.Conn) (respbody []byte, currUserID int64, resptype protocol.MsgType, err error) {
	loginReq := &protocol.LoginRequest{}
	loginRes := &protocol.LoginResponse{}
	loginRes.Ec = protocol.ErrorCode_EC_OK
	if err := proto.Unmarshal(data, loginReq); err != nil {
		fmt.Println("marshal login request failed!")
		//TODO
	} else {
		if *login {
			loginRes.Ec = protocol.ErrorCode_EC_LOGIN_AUTH_FAILED
		} else {
			fmt.Printf("user %d log in\n", loginReq.UserID)
			currUserID = loginReq.UserID
			*login = true
			//notify previous offline first
			if prevchan, online := msgchannel[loginReq.UserID]; online {
				offnotify := &protocol.OfflineNotify{}
				offnotify.UserID = loginReq.UserID
				notifytype := protocol.MsgType_MT_OFFLINE_NOYIFY
				notifymsg, err := proto.Marshal(offnotify)
				if err != nil {
					fmt.Println("marshal faliled!")
				}
				notify, err := writeBuffer(notifytype, notifymsg)
				if err != nil {
					panic(err)
				}
				prevchan <- notify.Bytes()
				offlinechan[loginReq.UserID] <- true
			}
			msgchannel[currUserID] = make(chan []byte)
			offlinechan[currUserID] = make(chan bool)
			go func() {
				for {
					if *login == false {
						break
					}
					data := <-msgchannel[currUserID]
					if _, err := conn.Write(data); err != nil {
						//close <- true
					}
				}
			}()
			go func() {
				for {
					if <-offlinechan[currUserID] {
						*login = false
					}
				}
			}()
		}
	}

	resptype = protocol.MsgType_MT_LOGIN_RESPONSE
	respbody, err = proto.Marshal(loginRes)
	if err != nil {
		fmt.Println("loginRes marshal failed!")
	}
	return
}

func caseRmCreateReq(data []byte, currUserID int64, chatroom map[int64]*ChatRoom,
	msgchannel map[int64]chan []byte, idgetter *IDgetter) (respbody []byte, resptype protocol.MsgType, err error) {
	rmChatCreatReq := &protocol.RoomCreateRequest{}
	rmChatCreatResp := &protocol.RoomCreateResponse{}

	if err = proto.Unmarshal(data, rmChatCreatReq); err != nil {
		fmt.Println("rmChatCreatReq Unmarshal failed!")
		//rmChatCreatResp.Ec=protocol.e
	}
	fmt.Println(rmChatCreatReq.Name)
	//sync
	rmChatCreatResp.RoomID = idgetter.spanNextID()
	rmChatCreatResp.Ec = protocol.ErrorCode_EC_OK

	if _, online := chatroom[rmChatCreatResp.RoomID]; online {
		rmChatCreatResp.Ec = protocol.ErrorCode_EC_ROOM_ALREADY_EXISTS
	} else {
		pchatroom := new(ChatRoom)
		pchatroom.RoomID = rmChatCreatResp.RoomID
		pchatroom.RoomName = rmChatCreatReq.Name
		pchatroom.UserIDs = []int64{currUserID}
		pchatroom.UserCnt = 1
		chatroom[rmChatCreatResp.RoomID] = pchatroom
	}
	//response info
	resptype = protocol.MsgType_MT_RMCREAT_RESPOSE
	respbody, err = proto.Marshal(rmChatCreatResp)
	if err != nil {
		fmt.Println("rmChatCreatResp marshal failed!")
	}
	return
}

func caseRmjoinReq(data []byte, curID int64, chatroom map[int64]*ChatRoom,
	msgchannel map[int64]chan []byte) (respbody []byte, resptype protocol.MsgType, err error) {
	//consider the max num of a group
	rmJoinReq := &protocol.RoomJoinRequest{}
	rmJoinResp := &protocol.RoomJoinResponse{}
	if err = proto.Unmarshal(data, rmJoinReq); err != nil {
		//TODO
	}

	if pchatroom, online := chatroom[rmJoinReq.RoomID]; online {
		rmJoinResp.Ec = protocol.ErrorCode_EC_OK
		pchatroom.UserIDs = append(pchatroom.UserIDs, curID)
		pchatroom.UserCnt++
	} else {
		rmJoinResp.Ec = protocol.ErrorCode_EC_ROOM_NO_ROOM
	}
	//response info
	resptype = protocol.MsgType_MT_RMJOIN_RESPONSE
	respbody, err = proto.Marshal(rmJoinResp)
	if err != nil {
		fmt.Println("rmJoinResp marshal failed!")
	}
	return
}

func caseRmchatReq(data []byte, curID int64, chatroom map[int64]*ChatRoom,
	msgchannel map[int64]chan []byte) (respbody []byte, resptype protocol.MsgType, err error) {
	rmChatReq := &protocol.RoomChatRequest{}
	rmChatResp := &protocol.RoomChatResponse{}
	rmChatResp.Ec = protocol.ErrorCode_EC_OK
	rmchatNotify := &protocol.RoomChatNotify{}
	//what's the difference ??
	if err = proto.Unmarshal(data, rmChatReq); err != nil {
		//TODO
	}
	rmchatNotify.UserID = curID
	rmchatNotify.RoomID = rmChatReq.RoomID
	rmchatNotify.Content = rmChatReq.GetContent()

	if _, exist := chatroom[rmChatReq.RoomID]; exist == true {
		for _, groupUser := range chatroom[rmChatReq.RoomID].UserIDs {
			if _, online := msgchannel[groupUser]; online == true {
				notifytype := protocol.MsgType_MT_RMCHAT_NOTIFY
				notifymsg, err := proto.Marshal(rmchatNotify)
				if err != nil {
					fmt.Println("marshal faliled!")
				}
				notify, err := writeBuffer(notifytype, notifymsg)
				if err != nil {
					panic(err)
				}
				msgchannel[groupUser] <- notify.Bytes()
			} else {
				//store  offline msg

			}
		}
	} else {
		rmChatResp.Ec = protocol.ErrorCode_EC_ROOM_NO_ROOM
	}
	//response info
	resptype = protocol.MsgType_MT_RMCHAT_RESPONSE
	respbody, err = proto.Marshal(rmChatResp)
	if err != nil {
		fmt.Println("rmJoinResp marshal failed!")
	}
	return
}

package main

//
//TCP msg fmt:
//the first 2 bytes represent msg length;
//the later 2 bytes represent msg type;
//then following with msg body
import (
	"bufio"
	"bytes"
	"chatserver/chatserver/src/protocol"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"gopkg.in/mgo.v2/bson"

	"github.com/golang/protobuf/proto"
	"gopkg.in/mgo.v2"
)

func initdata(db *mgo.Database, iddist *IDdistributor) {

	usermap = make(map[int64]*userInfo)
	roommap = make(map[int64]*roomInfo)
	cusers := db.C("users")
	cusers.Find(nil).All(&userRel)
	crooms := db.C("rooms")
	crooms.Find(nil).All(&chatRooms)
	for n := 0; n < len(userRel); n++ {
		usermap[userRel[n].Userid] = &userRel[n]
	}
	for n := 0; n < len(chatRooms); n++ {
		roommap[chatRooms[n].Roomid] = &chatRooms[n]
	}
	var roomid roomID
	croomid := db.C("roomid")

	if err := croomid.Find(nil).One(&roomid); err == nil {
		iddist.roomid = roomid.Roomid
	}
}

func main() {

	session, err := mgo.Dial("localhost")
	db := session.DB("chat")

	link, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("tcp chatserver listen failed!")
		return
	}
	fmt.Println("tcp chatserver listen start!")
	iddist := new(IDdistributor)
	initdata(db, iddist)
	//消息管道
	msgchnl := make(map[int64]chan []byte)
	offlinechan := make(map[int64]chan bool)

	defer func() {
		session.Close()
	}()
	for {
		conn, err := link.Accept()
		if err != nil {
			continue
		}
		go handleConnection(conn, msgchnl, offlinechan, iddist, db)
	}
}

func handleConnection(conn net.Conn, msgchannel map[int64]chan []byte, offlinechan map[int64]chan bool,
	idgetter *IDdistributor, db *mgo.Database) {
	addr := conn.RemoteAddr()
	fmt.Printf("client %s:%s connected to server\n", addr.Network(), addr.String())
	var currUserID int64
	fmt.Printf("curID is [%d]", currUserID)
	Login := false
	var pMsg []privMsg
	defer func() {
		//take it as active off line
		fmt.Printf("curID in defer is [%d]\n", currUserID)
		if currUserID > 0 {
			if _, exist := msgchannel[currUserID]; exist {
				delete(msgchannel, currUserID)
				fmt.Printf("delete userid [%v] from msgChannel\n", currUserID)
			}
			if _, exist := offlinechan[currUserID]; exist {
				delete(offlinechan, currUserID)
				fmt.Printf("delete userid [%v] from offChannel\n", currUserID)
			}
		}
		fmt.Println(" conn closed")
		conn.Close()
		cusers := db.C("users")
		var result userInfo
		if err := cusers.Find(bson.M{"userid": currUserID}).One(&result); err != nil {
			cusers.Insert(usermap[currUserID])
		}
		cusers.Update(bson.M{"userid": currUserID}, bson.M{"$set": bson.M{"offlinetime": time.Now(), "chatrooms": usermap[currUserID].ChatRooms}})
		//cusers.Upsert(bson.M{"userid": currUserID}, bson.M{"$set": bson.M{"offlinetime": time.Now(), "chatrooms": usermap[currUserID].ChatRooms}})
		cpmsg := db.C("privmsg")
		for _, pmsg := range pMsg {
			cpmsg.Insert(pmsg)
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
				respbody, currUserID, resptype, err = caseLoginReq(data, &Login, offlinechan, msgchannel, conn, db)
				fmt.Printf("curID after login is [%d]", currUserID)
			case protocol.MsgType_MT_PRIVCHAT_REQUEST:
				respbody, resptype, err = casePrivchatReq(data, currUserID, db, msgchannel, pMsg)
			case protocol.MsgType_MT_RMCREAT_REQUEST:
				respbody, resptype, err = caseRmCreateReq(data, currUserID, msgchannel, idgetter, db)
			case protocol.MsgType_MT_RMJOIN_REQUEST:
				respbody, resptype, err = caseRmjoinReq(data, currUserID, db, msgchannel)
			case protocol.MsgType_MT_RMCHAT_REQUSET:
				respbody, resptype, err = caseRmchatReq(data, currUserID, msgchannel, db)
			default:
				fmt.Println("Wrong msg type!")
				continue
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
		return nil, -1, nil
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

func writeBuffer(msgtype protocol.MsgType, msgbody []byte) (msgbuf *bytes.Buffer, err error) {
	size := len(msgbody)
	notify := bytes.NewBuffer(make([]byte, 0, 4+size))
	if err = binary.Write(notify, binary.LittleEndian, uint16(size)); err != nil {
		//TODO
	}
	if err = binary.Write(notify, binary.LittleEndian, uint16(msgtype)); err != nil {
		//TODO
	}
	if err = binary.Write(notify, binary.LittleEndian, msgbody); err != nil {
		//TODO
	}
	return notify, err
}

func caseLoginReq(data []byte, login *bool, offlinechan map[int64]chan bool, msgchannel map[int64]chan []byte,
	conn net.Conn, db *mgo.Database) (respbody []byte, currUserID int64, resptype protocol.MsgType, err error) {
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
			fmt.Printf("curID in login is [%d]", currUserID)
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
			fmt.Printf("curID is [%d]", currUserID)
			if _, has := usermap[currUserID]; !has {
				usermap[currUserID] = &userInfo{currUserID, time.Now(), []int64{}}
			}
			msgchannel[currUserID] = make(chan []byte)
			offlinechan[currUserID] = make(chan bool)
			//pull offline msg
			pullmsg(db, currUserID, conn)

			go func() {
				for {
					if *login == false {
						break
					}
					fmt.Printf("curID in msgchan routine is [%d]", currUserID)
					data := <-msgchannel[currUserID]
					if _, err := conn.Write(data); err != nil {
						//close <- true
					}
				}
			}()
			go func() {
				for {
					fmt.Printf("curID in off chan routine is [%d]", currUserID)
					if <-offlinechan[currUserID] {
						*login = false
						break
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
func pullmsg(db *mgo.Database, curid int64, conn net.Conn) {
	cuser := db.C("users")
	var offtime time.Time
	var off userInfo
	// if err := cuser.Find(bson.M{"userid": curid}).Select(bson.M{"offlinetime": 1}).One(&offtime); err != nil {
	// 	fmt.Println("read offline time error!")
	// 	offtime = time.Now()
	// }
	cuser.Find(bson.M{"userid": curid}).One(&off)
	offtime = off.Offlinetime
	roomsin := usermap[curid].ChatRooms
	var privmsg []privMsg
	cprivmsg := db.C("privmsg")
	if err := cprivmsg.Find(&bson.M{"tgtid": curid, "time": &bson.M{"$gte": offtime}}).All(&privmsg); err != nil {
		panic(err)
	}
	for _, eachpmsg := range privmsg {
		var ireslmsg IResolveMsg = &eachpmsg
		data := ireslmsg.ResolveMsg()
		n, err := conn.Write(data)
		if err != nil {
			fmt.Println("conn write wrong!")
		}
		fmt.Printf("wrote %d bytes to client\n", n)
	}
	var roommsg []roomMsg
	crmmsg := db.C("roommsg")
	for _, room := range roomsin {
		var oneroommsg []roomMsg
		crmmsg.Find(&bson.M{"roomid": room, "time": &bson.M{"$gte": offtime}}).All(&oneroommsg)
		roommsg = append(roommsg, oneroommsg...)
	}
	for _, eachrmsg := range roommsg {
		var ireslmsg IResolveMsg = &eachrmsg
		data := ireslmsg.ResolveMsg()
		n, err := conn.Write(data)
		if err != nil {
			fmt.Println("conn write wrong!")
		}
		fmt.Printf("wrote %d bytes to client\n", n)
	}
}
func casePrivchatReq(data []byte, currUserID int64, db *mgo.Database, msgchannel map[int64]chan []byte,
	pMsg []privMsg) (respbody []byte, resptype protocol.MsgType, err error) {
	fmt.Printf("curID in private chat is [%d]", currUserID)
	privChatReq := &protocol.PrivateChatRequest{}
	privChatResp := &protocol.PrivateChatResponse{}
	privChatResp.Ec = protocol.ErrorCode_EC_OK
	if err = proto.Unmarshal(data, privChatReq); err != nil {
		fmt.Println("marshal private chat request failed!")
	}

	privChatNotify := &protocol.PrivateChatNotify{}
	privChatNotify.Src = currUserID
	privChatNotify.Content = privChatReq.Content

	//target is online, send notify msg to channel
	if _, exist := usermap[privChatReq.Target]; exist {
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
				//write in cache
				pMsg = append(pMsg, privMsg{currUserID, privChatReq.Target, time.Now(), privChatReq.Content})
			}
		} else {
			//offline write db
			cpmsg := db.C("privmsg")
			cpmsg.Insert(&privMsg{currUserID, privChatReq.Target, time.Now(), privChatReq.Content})
			privChatResp.Ec = protocol.ErrorCode_EC_CHAT_TARGET_OFFLINE
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
func caseRmCreateReq(data []byte, currUserID int64, msgchannel map[int64]chan []byte,
	iddist *IDdistributor, db *mgo.Database) (respbody []byte, resptype protocol.MsgType, err error) {
	fmt.Printf("curID in rmcreat is [%d]", currUserID)
	rmChatCreatReq := &protocol.RoomCreateRequest{}
	rmChatCreatResp := &protocol.RoomCreateResponse{}

	if err = proto.Unmarshal(data, rmChatCreatReq); err != nil {
		fmt.Println("rmCreatReq Unmarshal failed!")
	}
	fmt.Println(rmChatCreatReq.Name)
	//sync
	rmChatCreatResp.RoomID = iddist.spanNextID()
	if _, err := db.C("roomid").Upsert(nil, bson.M{"$set": bson.M{"rmid": iddist.roomid}}); err != nil {
		fmt.Println("upsert->", err)
	}
	rmChatCreatResp.Ec = protocol.ErrorCode_EC_OK

	if _, online := roommap[rmChatCreatResp.RoomID]; online {
		rmChatCreatResp.Ec = protocol.ErrorCode_EC_ROOM_ALREADY_EXISTS
	} else {
		pchatroom := new(roomInfo)
		pchatroom.Roomid = rmChatCreatResp.RoomID
		pchatroom.Roomname = rmChatCreatReq.Name
		pchatroom.Memid = []int64{currUserID}
		roommap[rmChatCreatResp.RoomID] = pchatroom
		usermap[currUserID].ChatRooms = append(usermap[currUserID].ChatRooms, pchatroom.Roomid)
		croom := db.C("rooms")
		croom.Insert(pchatroom)
	}
	//response info
	resptype = protocol.MsgType_MT_RMCREAT_RESPOSE
	respbody, err = proto.Marshal(rmChatCreatResp)
	if err != nil {
		fmt.Println("rmChatCreatResp marshal failed!")
	}
	return
}

func caseRmjoinReq(data []byte, curID int64, db *mgo.Database,
	msgchannel map[int64]chan []byte) (respbody []byte, resptype protocol.MsgType, err error) {
	//consider the max num of a group
	fmt.Printf("curID in rmjoin is [%d]", curID)
	rmJoinReq := &protocol.RoomJoinRequest{}
	rmJoinResp := &protocol.RoomJoinResponse{}
	if err = proto.Unmarshal(data, rmJoinReq); err != nil {
		//TODO
	}

	if pchatroom, has := roommap[rmJoinReq.RoomID]; has {
		rmJoinResp.Ec = protocol.ErrorCode_EC_OK
		pchatroom.Memid = append(pchatroom.Memid, curID)
		usermap[curID].ChatRooms = append(usermap[curID].ChatRooms, rmJoinReq.RoomID)
		croom := db.C("rooms")
		croom.Update(bson.M{"roomid": rmJoinReq.RoomID},
			bson.M{"$push": bson.M{
				"memid": curID,
			}})
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

func caseRmchatReq(data []byte, curID int64, msgchannel map[int64]chan []byte,
	db *mgo.Database) (respbody []byte, resptype protocol.MsgType, err error) {
	fmt.Printf("curID in rmchat is [%d]", curID)
	rmChatReq := &protocol.RoomChatRequest{}
	rmChatResp := &protocol.RoomChatResponse{}
	rmChatResp.Ec = protocol.ErrorCode_EC_OK
	rmchatNotify := &protocol.RoomChatNotify{}
	if err = proto.Unmarshal(data, rmChatReq); err != nil {
		fmt.Println("Unmarshal rmChatReq failed!")
	}
	rmchatNotify.UserID = curID
	rmchatNotify.RoomID = rmChatReq.RoomID
	rmchatNotify.Content = rmChatReq.Content

	if _, exist := roommap[rmChatReq.RoomID]; exist {
		for _, groupUser := range roommap[rmChatReq.RoomID].Memid {
			if _, online := msgchannel[groupUser]; online && groupUser != curID {
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
			}
		}
		cpmsg := db.C("roommsg")
		cpmsg.Insert(&roomMsg{rmChatReq.RoomID, curID, rmChatReq.Content, time.Now()})
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

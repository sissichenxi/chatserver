package main

import (
	"chatserver/chatserver/src/protocol"
	"fmt"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
)

type roomInfo struct {
	Roomid   int64   `bson:"roomid"`
	Roomname string  `bson:"roomname"`
	Memid    []int64 `bson:"memid"`
}

type roomMsg struct {
	Roomid  int64     `bson:"roomid"`
	Srcid   int64     `bson:"srcid"`
	Content string    `bson:"content"`
	Time    time.Time `bson:"time"`
}

type IDdistributor struct {
	roomid int64
	mu     sync.Mutex
}

func (idget *IDdistributor) spanNextID() (nextid int64) {
	idget.mu.Lock()
	idget.roomid++
	nextid = idget.roomid
	idget.mu.Unlock()
	return
}

const maxRoomMember = 100

var chatRooms []roomInfo
var roommap map[int64]*roomInfo

func (rmmsg *roomMsg) ResolveMsg() []byte {
	rmchatNotify := &protocol.RoomChatNotify{}
	rmchatNotify.UserID = rmmsg.Srcid
	rmchatNotify.RoomID = rmmsg.Roomid
	rmchatNotify.Content = rmmsg.Content
	notifytype := protocol.MsgType_MT_RMCHAT_NOTIFY
	notifymsg, err := proto.Marshal(rmchatNotify)
	if err != nil {
		fmt.Println("marshal rmchatNotify faliled!")
	}
	notify, err := writeBuffer(notifytype, notifymsg)
	if err != nil {
		panic(err)
	}
	return notify.Bytes()
}

type roomID struct {
	Roomid int64 `bson:"rmid"`
}

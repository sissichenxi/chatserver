package main

import (
	"chatserver/chatserver/src/protocol"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
)

type IResolveMsg interface {
	ResolveMsg() []byte
}
type userInfo struct {
	Userid      int64     `bson:"userid"`
	Offlinetime time.Time `bson:"offlinetime"`
	ChatRooms   []int64   `bson:"chatrooms"`
}

type privMsg struct {
	Srcid   int64     `bson:"srcid"`
	Tgtid   int64     `bson:"tgtid"`
	Time    time.Time `bson:"time"`
	Content string    `bson:"content"`
}

func (pmsg *privMsg) ResolveMsg() []byte {
	privChatNotify := &protocol.PrivateChatNotify{}
	privChatNotify.Src = pmsg.Srcid
	privChatNotify.Content = pmsg.Content
	notifytype := protocol.MsgType_MT_PRIVCHAT_NOYIFY
	notifymsg, err := proto.Marshal(privChatNotify)
	if err != nil {
		fmt.Println("marshal faliled!")
	}
	notify, err := writeBuffer(notifytype, notifymsg)
	if err != nil {
		panic(err)
	}
	return notify.Bytes()
}

var userRel []userInfo
var usermap map[int64]*userInfo

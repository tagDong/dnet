package main

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/yddeng/dnet"
	"github.com/yddeng/dnet/dtcp"
	"github.com/yddeng/dnet/example/cs/codec"
	"github.com/yddeng/dnet/example/module/handler"
	"github.com/yddeng/dnet/example/module/message"
	"github.com/yddeng/dnet/example/pb"
	"reflect"
	"time"
)

func echoToC(session dnet.Session, msg *message.Message) {
	data := msg.GetData().(*pb.EchoToS)
	fmt.Println("echo", data.GetMsg(), time.Now().String())

	_ = session.Send(message.NewMessage(0, &pb.EchoToC{Msg: proto.String("hello client")}))
}

func main() {

	gHandler := handler.NewHandler()
	gHandler.RegisterCallBack(&pb.EchoToS{}, echoToC)

	addr := "localhost:1234"
	l, err := dtcp.NewTCPListener("tcp", addr)
	if err != nil {
		fmt.Println(1, err)
		return
	}

	err = l.Listen(func(session dnet.Session) {
		fmt.Println("new client", session.RemoteAddr().String())
		// 超时时间
		session.SetTimeout(10*time.Second, 0)
		session.SetCodec(codec.NewCodec())
		session.SetCloseCallBack(func(session dnet.Session, reason string) {
			fmt.Println("onClose", reason)
		})
		fmt.Println("newClient ", session.RemoteAddr(), reflect.TypeOf(session.RemoteAddr()))
		errr := session.Start(func(data interface{}, err error) {
			//fmt.Println("data", data, "err", err)
			if err != nil {
				session.Close(err.Error())
			} else {
				gHandler.Dispatch(session, data.(*message.Message))
			}
		})
		if errr != nil {
			fmt.Println(2, err)
		}
	})
	if err != nil {
		fmt.Println(3, err)
	}

	fmt.Println("server start on :", addr)
	select {}
}
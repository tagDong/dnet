package ws

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/yddeng/dnet"
	"net"
	"reflect"
	"sync"
	"time"
)

const (
	started = 0x01 //0000 0001
	closed  = 0x02 //0000 0010
)

const sendBufChanSize = 1024

type Conn struct {
	flag         byte
	conn         *websocket.Conn
	uData        interface{}   //用户数据
	readTimeout  time.Duration // 读超时
	writeTimeout time.Duration // 写超时

	sendBufChan chan []byte //发送队列

	msgCallback   func(interface{}, error) //消息回调
	closeCallback func(string)             //关闭连接回调
	closeReason   string                   //关闭原因

	lock sync.Mutex
}

func newConn(conn *websocket.Conn) *Conn {
	return &Conn{
		conn:        conn,
		sendBufChan: make(chan []byte, sendBufChanSize),
	}
}

//读写超时
func (this *Conn) SetTimeout(readTimeout, writeTimeout time.Duration) {
	defer this.lock.Unlock()
	this.lock.Lock()

	this.readTimeout = readTimeout
	this.writeTimeout = writeTimeout
}

func (this *Conn) LocalAddr() net.Addr {
	return this.conn.LocalAddr()
}

func (this *Conn) NetConn() interface{} {
	return this.conn
}

//对端地址
func (this *Conn) RemoteAddr() net.Addr {
	return this.conn.RemoteAddr()
}

func (this *Conn) SetCodec(codec dnet.Codec) {}

func (this *Conn) SetCloseCallBack(closeCallback func(reason string)) {
	this.lock.Lock()
	defer this.lock.Unlock()
	this.closeCallback = closeCallback
}

func (this *Conn) SetUserData(ud interface{}) {
	this.lock.Lock()
	this.uData = ud
	this.lock.Unlock()
}

func (this *Conn) GetUserData() interface{} {
	this.lock.Lock()
	defer this.lock.Unlock()
	return this.uData
}

//开启消息处理
func (this *Conn) Start(msgCb func(interface{}, error)) error {
	if msgCb == nil {
		return dnet.ErrNoMsgCallBack
	}

	this.lock.Lock()
	if this.flag == started {
		return dnet.ErrSessionStarted
	}
	this.flag = started
	this.msgCallback = msgCb
	this.lock.Unlock()

	go this.receiveThread()
	go this.sendThread()

	return nil
}

func (this *Conn) isClose() bool {
	this.lock.Lock()
	defer this.lock.Unlock()
	return this.flag == closed
}

//接收线程
func (this *Conn) receiveThread() {
	for {
		if this.isClose() {
			return
		}
		if this.readTimeout > 0 {
			_ = this.conn.SetReadDeadline(time.Now().Add(this.readTimeout))
		}
		_, msg, err := this.conn.ReadMessage()
		if this.isClose() {
			return
		}
		if err != nil {
			this.msgCallback(nil, err)
		} else {
			this.msgCallback(msg, err)
		}
	}
}

//发送线程
func (this *Conn) sendThread() {
	defer this.close()
	for {
		data, isOpen := <-this.sendBufChan
		if !isOpen {
			break
		}
		if this.writeTimeout > 0 {
			_ = this.conn.SetWriteDeadline(time.Now().Add(this.writeTimeout))
		}

		err := this.conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			this.msgCallback(nil, err)
		}

	}
}

func (this *Conn) Send(o interface{}) error {
	if o == nil {
		return dnet.ErrSendMsgNil
	}

	data, ok := o.([]byte)
	if !ok {
		return fmt.Errorf("interface {} is %s,need []byte or use SendMsg(data []byte)", reflect.TypeOf(o).String())
	}

	return this.SendBytes(data)
}

func (this *Conn) SendBytes(data []byte) error {
	if len(data) == 0 {
		return dnet.ErrSendMsgNil
	}

	//非堵塞
	if len(this.sendBufChan) == sendBufChanSize {
		return dnet.ErrSendChanFull
	}

	this.lock.Lock()
	if this.flag == 0 {
		return dnet.ErrNotStarted
	}
	if this.flag == closed {
		return dnet.ErrSessionClosed
	}
	this.lock.Unlock()

	this.sendBufChan <- data
	return nil
}

/*
 主动关闭连接
 先关闭读，待写发送完毕关闭写
*/
func (this *Conn) Close(reason string) {
	this.lock.Lock()
	defer this.lock.Unlock()
	if (this.flag & closed) > 0 {
		return
	}

	close(this.sendBufChan)
	this.closeReason = reason
	this.flag = closed
}

func (this *Conn) close() {
	_ = this.conn.Close()
	this.lock.Lock()
	callback := this.closeCallback
	msg := this.closeReason
	this.lock.Unlock()
	if callback != nil {
		callback(msg)
	}
}

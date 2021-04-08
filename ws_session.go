package dnet

import (
	"github.com/gorilla/websocket"
)

type WSSession struct {
	*session
	conn *WSConn
}

// NewWSSession return an initialized *WSSession
func NewWSSession(conn *websocket.Conn, options ...Option) (*WSSession, error) {
	op := loadOptions(options...)
	if op.MsgCallback == nil {
		return nil, ErrNilMsgCallBack
	}
	// init default codec
	if op.Codec == nil {
		op.Codec = newWsCodec()
	}

	// WSConn
	wsConn := NewWSConn(conn)

	session := &WSSession{
		conn:    wsConn,
		session: newSession(wsConn, op),
	}

	return session, nil
}

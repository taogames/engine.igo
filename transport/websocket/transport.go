package websocket

import (
	"net/http"

	"engine.igo/v4/transport"
	gorilla "github.com/gorilla/websocket"
)

type Transport struct {
	ReadBufferSize  int
	WriteBufferSize int
	CheckOrigin     func(r *http.Request) bool
}

var _ transport.Transport = (*Transport)(nil)

var Default = &Transport{}

func (t *Transport) Name() string {
	return "websocket"
}

func (t *Transport) Accept(w http.ResponseWriter, r *http.Request) (transport.Conn, error) {
	upgrader := gorilla.Upgrader{
		ReadBufferSize:  t.ReadBufferSize,
		WriteBufferSize: t.WriteBufferSize,
		CheckOrigin:     t.CheckOrigin,
	}
	c, err := upgrader.Upgrade(w, r, w.Header())
	if err != nil {
		return nil, err
	}

	return newConn(c), nil
}

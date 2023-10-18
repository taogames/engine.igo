package websocket

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	gorilla "github.com/gorilla/websocket"
	"github.com/taogames/engine.igo/message"
	"go.uber.org/zap"
)

type Conn struct {
	logger *zap.SugaredLogger

	sync.Mutex
	ws *gorilla.Conn

	closeCh chan error
}

func newConn(c *gorilla.Conn) *Conn {
	return &Conn{
		ws:      c,
		closeCh: make(chan error),
	}
}

func (c *Conn) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	fmt.Println("websocket ServeHTTP")
	return <-c.closeCh
}

func (c *Conn) TryWrite(mt message.MessageType, pt message.PacketType, data []byte) {
	c.Write(mt, pt, data)
}

func (c *Conn) Read() (message.MessageType, message.PacketType, []byte, error) {
	mti, bs, err := c.ws.ReadMessage()
	if err != nil {
		return 0, 0, nil, err
	}
	mt := message.MessageType(mti)
	var pt message.PacketType

	if mt == message.MTText {
		pt, err = message.ParsePacketType(bs[0])
		if err != nil {
			c.Close(true)
			return 0, 0, nil, err
		}
		bs = bs[1:]
	}

	return mt, pt, bs, nil
}

func (c *Conn) Write(mt message.MessageType, pt message.PacketType, data []byte) error {
	var msg []byte
	if mt == message.MTText {
		msg = append(msg, pt.Byte())
	}
	msg = append(msg, data...)

	c.Lock()
	defer c.Unlock()

	switch mt {
	case message.MTText:
		c.logger.Debug("write: ", string(msg))
	case message.MTBinary:
		c.logger.Debug("write: ", msg)
	}

	return c.ws.WriteMessage(int(mt), msg)
}

func (c *Conn) Close(bool) error {
	select {
	case <-c.closeCh:
		return nil

	default:
		close(c.closeCh)
		c.ws.WriteMessage(gorilla.CloseMessage, nil)
		return c.ws.Close()
	}
}

func (c *Conn) Name() string {
	return "websocket"
}

func (c *Conn) Pause() {
	log.Fatal("Websocket should never be paused")
}

func (c *Conn) WithLogger(logger *zap.SugaredLogger) {
	c.logger = logger
}

package websocket

import (
	"io"
	"log"
	"net/http"

	gorilla "github.com/gorilla/websocket"
	"github.com/taogames/engine.igo/message"
)

type Conn struct {
	*gorilla.Conn

	errCh chan error
}

func newConn(c *gorilla.Conn) *Conn {
	return &Conn{
		Conn:  c,
		errCh: make(chan error),
	}
}

func (c *Conn) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	return <-c.errCh
}

type wrapper struct {
	w  io.WriteCloser
	mt message.MessageType
	pt message.PacketType
}

func (w *wrapper) Write(bs []byte) (int, error) {
	var n int
	if w.mt == message.MTText {
		n1, err := w.w.Write(w.pt.Bytes())
		n += n1
		if err != nil {
			return n1, err
		}
	}

	n2, err := w.w.Write(bs)
	n += n2
	return n, err
}

func (w *wrapper) Close() error {
	return w.w.Close()
}

func (c *Conn) NextReader() (message.MessageType, message.PacketType, io.ReadCloser, error) {
	mti, r, err := c.Conn.NextReader()
	if err != nil {
		return 0, 0, nil, err
	}

	mt := message.MessageType(mti)
	var pt message.PacketType

	switch mt {
	case message.MTText:
		bs := make([]byte, 1)
		_, err := r.Read(bs)
		if err != nil {
			return 0, 0, nil, err
		}
		pt, err = message.ParsePacketType(bs[0])
		if err != nil {
			c.errCh <- err
			return 0, 0, nil, err
		}
	case message.MTBinary:
	}

	return mt, pt, r.(io.ReadCloser), nil
}

func (c *Conn) NextWriter(mt message.MessageType, pt message.PacketType) (io.WriteCloser, error) {
	w, err := c.Conn.NextWriter(int(mt))
	return &wrapper{
		w:  w,
		mt: mt,
		pt: pt,
	}, err
}

func (c *Conn) Close(bool) error {
	close(c.errCh)

	c.Conn.WriteMessage(gorilla.CloseMessage, nil)
	return c.Conn.Close()
}

func (c *Conn) Name() string {
	return "websocket"
}

func (c *Conn) Pause() {
	log.Fatal("Websocket should never be paused")
}

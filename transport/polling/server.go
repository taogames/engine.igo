package polling

import (
	"fmt"
	"io"
	"net/http"

	"engine.igo/v4/message"
)

type serverConn struct {
	payload *Payload

	host       string
	remoteAddr string

	pongCh    chan struct{}
	closeType message.PacketType
}

func (c *serverConn) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodGet:
		if err := c.payload.PutWriter(w); err != nil {
			return err
		}

	case http.MethodPost:
		err := c.payload.PutReader(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return err
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))

	default:
		http.Error(w, fmt.Sprintf("invalid method %s", r.Method), http.StatusBadRequest)
	}

	return nil
}

func (c *serverConn) NextReader() (message.MessageType, message.PacketType, io.ReadCloser, error) {
	return c.payload.GetReader()
}

func (c *serverConn) NextWriter(mt message.MessageType, pt message.PacketType) (io.WriteCloser, error) {
	return c.payload.GetWriter(pt)
}

func (c *serverConn) Close(noop bool) error {
	pt := message.PTClose
	if noop {
		pt = message.PTNoop
	}

	c.payload.Close(pt)
	return nil
}

func (c *serverConn) Name() string {
	return "polling"
}

func (c *serverConn) Pause() {
	c.payload.Pause()
}

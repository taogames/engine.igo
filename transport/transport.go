package transport

import (
	"io"
	"net/http"

	"github.com/taogames/engine.igo/message"
)

type Conn interface {
	Name() string
	ServeHTTP(w http.ResponseWriter, r *http.Request) error
	Close(noop bool) error
	Pause()

	NextReader() (message.MessageType, message.PacketType, io.ReadCloser, error)
	NextWriter(mt message.MessageType, pt message.PacketType) (io.WriteCloser, error)
}

type Transport interface {
	Name() string
	Accept(w http.ResponseWriter, r *http.Request) (Conn, error)
}

package transport

import (
	"net/http"

	"github.com/taogames/engine.igo/message"
	"go.uber.org/zap"
)

type Conn interface {
	Name() string
	ServeHTTP(w http.ResponseWriter, r *http.Request) error
	Close(noop bool) error
	Pause()

	Read() (message.MessageType, message.PacketType, []byte, error)
	Write(mt message.MessageType, pt message.PacketType, data []byte) error

	WithLogger(logger *zap.SugaredLogger)
}

type Transport interface {
	Name() string
	Accept(w http.ResponseWriter, r *http.Request) (Conn, error)
}

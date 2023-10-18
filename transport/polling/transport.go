package polling

import (
	"net/http"

	"github.com/taogames/engine.igo/message"
	"github.com/taogames/engine.igo/transport"
)

type Transport struct {
}

var _ transport.Transport = (*Transport)(nil)

var Default = &Transport{}

func (t *Transport) Name() string {
	return "polling"
}

func (t *Transport) Accept(w http.ResponseWriter, r *http.Request) (transport.Conn, error) {
	conn := &serverConn{
		pollCh:    make(chan http.ResponseWriter, 1),
		pollErrCh: make(chan error),
		dataCh:    make(chan []byte, 1),
		dataErrCh: make(chan error),

		closeCh: make(chan struct{}),

		host:       r.Host,
		remoteAddr: r.RemoteAddr,
		pongCh:     make(chan struct{}),
		closeType:  message.PTClose,
	}

	return conn, nil
}

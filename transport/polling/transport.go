package polling

import (
	"net/http"

	"engine.igo/v4/message"
	"engine.igo/v4/transport"
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
		payload:    NewPayload(),
		host:       r.Host,
		remoteAddr: r.RemoteAddr,
		pongCh:     make(chan struct{}),
		closeType:  message.PTClose,
	}

	return conn, nil
}

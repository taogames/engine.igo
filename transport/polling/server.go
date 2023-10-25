package polling

import (
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
	"github.com/taogames/engine.igo/message"
	"github.com/taogames/engine.igo/transport"
	"go.uber.org/zap"
)

type serverConn struct {
	logger *zap.SugaredLogger

	pollCh    chan http.ResponseWriter
	pollErrCh chan error
	dataCh    chan []byte
	dataErrCh chan error

	pauseCh chan struct{}
	closeCh chan struct{}

	host       string
	remoteAddr string

	pongCh    chan struct{}
	closeType message.PacketType
}

func (c *serverConn) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodGet:
		return c.onPollRequest(w)
	case http.MethodPost:
		return c.onDataRequest(w, r)
	default:
		http.Error(w, fmt.Sprintf("invalid method %s", r.Method), http.StatusBadRequest)
	}

	return nil
}

func (c *serverConn) onPollRequest(w http.ResponseWriter) error {
	c.logger.Debug("onPollRequest")

	select {
	case <-c.pauseCh:
		c.logger.Debug("c.pauseCh")
		_, err := w.Write(message.PTNoop.Bytes())
		return err

	case <-c.closeCh:
		return transport.ErrClosed
	default:
		// pass
	}

	select {
	case c.pollCh <- w:
		err := <-c.pollErrCh
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return err

	case <-c.pauseCh:
		c.logger.Debug("c.pauseCh")
		_, err := w.Write(message.PTNoop.Bytes())
		return err

	case <-c.closeCh:
		return transport.ErrClosed

	default:
		err := errors.New("duplicate get")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}
}

func (c *serverConn) onDataRequest(w http.ResponseWriter, r *http.Request) error {
	c.logger.Debug("onDataRequest")
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	c.logger.Debug("onDataRequest: ", string(bs))

	select {
	case c.dataCh <- bs:
		err := <-c.dataErrCh
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			_, err = w.Write([]byte("ok"))
		}
		return err
	default:
		err := errors.New("duplicate post")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}
}

func (c *serverConn) Write(mt message.MessageType, pt message.PacketType, data []byte) error {
	select {
	case <-c.closeCh:
		return transport.ErrClosed
	default:
		// pass
	}

	select {
	case w := <-c.pollCh:
		msg := []byte{pt.Byte()}
		msg = append(msg, data...)

		_, err := w.Write(msg)
		c.pollErrCh <- err
		return err

	case <-c.closeCh:
		return transport.ErrClosed
	}
}

func (c *serverConn) Read() (message.MessageType, message.PacketType, []byte, error) {
	select {
	case bs := <-c.dataCh:
		var (
			mt  message.MessageType
			pt  message.PacketType
			err error
		)

		if len(bs) == 0 {
			err = errors.New("empty data")
		} else {
			b := bs[0]

			if b == 'b' {
				mt = message.MTBinary
			} else {
				mt = message.MTText
				pt, err = message.ParsePacketType(b)
			}
		}

		c.dataErrCh <- err // to close
		return mt, pt, bs[1:], err

	case <-c.closeCh:
		return 0, 0, nil, transport.ErrClosed
	}
}

func (c *serverConn) Close(noop bool) error {
	select {
	case <-c.closeCh:
		// already closed
	default:
		close(c.closeCh)
		c.pauseCh = make(chan struct{})
	}

	select {
	case w := <-c.pollCh:
		pt := message.PTClose
		if noop {
			pt = message.PTNoop
		}

		_, err := w.Write(pt.Bytes())
		if err != nil {
			c.logger.Errorf("close: %s", err)
		}
		c.pollErrCh <- err
		return err

	default:
		return nil
	}
}

func (c *serverConn) Name() string {
	return "polling"
}

func (c *serverConn) Pause() {
	select {
	case <-c.pauseCh:
		// already paused
	default:
		close(c.pauseCh)
	}
}

func (c *serverConn) WithLogger(logger *zap.SugaredLogger) {
	c.logger = logger
}

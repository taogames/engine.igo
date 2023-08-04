package engineigo

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/taogames/engine.igo/message"
	"github.com/taogames/engine.igo/transport"
	"github.com/taogames/engine.igo/transport/polling"
	"go.uber.org/zap"
)

var (
	ErrTransportError error = errors.New("transport error")
)

type Session struct {
	id     string
	server *Server
	conn   transport.Conn

	conf *HandshakeConfig

	logger *zap.SugaredLogger

	getLock  sync.Mutex
	postLock sync.Mutex

	closeCh chan struct{}
	pongCh  chan struct{}

	upgradeLock sync.Mutex
	clientClose bool
}

func (s *Session) ID() string {
	return s.id
}

func (s *Session) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	return s.conn.ServeHTTP(w, r)
}

func (s *Session) Init() {
	w, err := s.conn.NextWriter(message.MTText, message.PTOpen)
	defer func() {
		if err := w.Close(); err != nil {
			s.logger.Error("Init Close error: ", err)
		}
	}()
	if err != nil {
		s.logger.Error("Init NextWriter error: ", err)
		return
	}

	s.conf.Sid = s.id
	j, _ := json.Marshal(s.conf)

	_, err = w.Write(j)
	if err != nil {
		s.logger.Errorf("Init Write %v error", string(j))
	}
	s.logger.Debug("Init Write ", string(j))
}

func (s *Session) NextWriter(mt message.MessageType, pt message.PacketType) (io.WriteCloser, error) {
	for {
		s.upgradeLock.Lock()
		conn := s.conn
		s.upgradeLock.Unlock()

		w, err := conn.NextWriter(mt, pt)
		if err != nil {
			if errors.Is(err, polling.ErrUpgrade) {
				s.logger.Debug("NextWriter ErrUpgrade")
				continue
			}
			return nil, errors.Join(err, ErrTransportError)
		}
		return w, nil
	}
}

func (s *Session) WriteMessage(msg *message.Message) error {
	w, err := s.conn.NextWriter(msg.Type, message.PTMessage)
	if err != nil {
		return err
	}
	if _, err := w.Write(msg.Data); err != nil {
		return err
	}
	return w.Close()
}

func (s *Session) NextReader() (message.MessageType, message.PacketType, io.ReadCloser, error) {
	for {
		s.upgradeLock.Lock()
		conn := s.conn
		s.upgradeLock.Unlock()

		mt, pt, rc, err := conn.NextReader()
		if err != nil {
			if errors.Is(err, polling.ErrUpgrade) {
				s.logger.Debug("NextReader ErrUpgrade")
				continue
			}
			return 0, 0, nil, errors.Join(err, ErrTransportError)
		}

		switch pt {
		case message.PTPong:
			s.pongCh <- struct{}{}
			rc.Close()
			continue
		case message.PTClose:
			s.clientClose = true
			s.Close()
			rc.Close()
			continue
		}

		return mt, pt, rc, nil
	}
}

func (s *Session) Upgrade(w http.ResponseWriter, r *http.Request, reqTransport transport.Transport) error {
	// stop heartbeat
	close(s.closeCh)

	// conn
	oldConn := s.conn
	s.upgradeLock.Lock()
	newConn, err := reqTransport.Accept(w, r)
	if err != nil {
		return err
	}

	// wait for ping
	s.logger.Debug("[UPGRADE] 1", time.Now().UnixMilli())
	mt, pt, rc, err := newConn.NextReader()
	if err != nil {
		return err
	}
	if mt != message.MTText || pt != message.PTPing {
		return errors.New("upgrade rcv ping error")
	}

	// send pong
	s.logger.Debug("[UPGRADE] 2", time.Now().UnixMilli())
	wc, err := newConn.NextWriter(message.MTText, message.PTPong)
	if err != nil {
		return err
	}
	bs, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	if _, err := wc.Write(bs); err != nil {
		return err
	}
	wc.Close()

	// pause old
	oldConn.Pause()

	// wait for upgrade
	s.logger.Debug("[UPGRADE] 3", time.Now().UnixMilli())
	mt, pt, rc, err = newConn.NextReader()
	if err != nil {
		return err
	}
	if mt != message.MTText || pt != message.PTUpgrade {
		return errors.New("upgrade rcv upgrade error")
	}
	rc.Close()

	// replace conn
	s.logger.Debug("[UPGRADE] 4", time.Now().UnixMilli())
	s.conn = newConn
	s.closeCh = make(chan struct{})
	s.upgradeLock.Unlock()
	go oldConn.Close(false)

	// restart heatbeat
	s.logger.Debug("[UPGRADE] 5", time.Now().UnixMilli())
	go s.Ping()

	return nil
}

func (s *Session) Transport() string {
	return s.conn.Name()
}

func (s *Session) Close() error {
	select {
	case <-s.closeCh:
		return nil
	default:
		s.logger.Debug("Session close", s.clientClose)
		s.server.removeSession(s)
		close(s.closeCh)
		s.conn.Close(s.clientClose)
		return nil
	}
}

func (s *Session) Unique(method string) (ok bool) {
	if s.conn.Name() == "websocket" {
		return false
	}

	switch method {
	case http.MethodGet:
		ok = s.getLock.TryLock()
	case http.MethodPost:
		ok = s.postLock.TryLock()
	default:
		ok = true
	}
	return
}

func (s *Session) UnlockMethod(method string) {
	switch method {
	case http.MethodGet:
		s.getLock.Unlock()
	case http.MethodPost:
		s.postLock.Unlock()
	}
}

func (s *Session) Ping() {
	ticker := time.NewTicker(time.Duration(s.conf.PingInterval) * time.Millisecond)
	for {
		select {
		case <-s.closeCh:
			return
		case <-ticker.C:
			go s.ping()
		}
	}
}

func (s *Session) ping() {
	timeoutTimer := time.NewTimer(time.Duration(s.conf.PingTimeout) * time.Millisecond)

	defer timeoutTimer.Stop()

	s.logger.Debug("[Ping]")
	go func() {
		w, err := s.conn.NextWriter(message.MTText, message.PTPing)
		if err != nil {
			return
		}
		w.Write(nil)
		w.Close()
	}()

	select {
	case <-s.closeCh:
		// closed somewhere else
		return
	case <-timeoutTimer.C:
		// time out
		s.logger.Debug("[Ping] Timedout")
		s.Close()
		return
	case <-s.pongCh:
		// normal
		return
	}
}

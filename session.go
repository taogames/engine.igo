package engineigo

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/taogames/engine.igo/message"
	"github.com/taogames/engine.igo/transport"
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
	s.conf.Sid = s.id
	j, _ := json.Marshal(s.conf)

	err := s.conn.Write(message.MTText, message.PTOpen, j)
	if err != nil {
		s.logger.Errorf("Init Write %v error: %v", string(j), err)
		return
	}
	s.logger.Debug("Init Wrote ", string(j))
}

func (s *Session) WriteMessage(msg *message.Message) error {
	// return s.conn.Write(msg.Type, message.PTMessage, msg.Data)
	retry := true
	for {
		s.upgradeLock.Lock()
		conn := s.conn
		s.upgradeLock.Unlock()

		err := conn.Write(msg.Type, message.PTMessage, msg.Data)
		if err != nil && errors.Is(err, transport.ErrClosed) && retry {
			s.logger.Debug("retry write: ", err)
			retry = false
		} else {
			return err
		}
	}
}

func (s *Session) ReadMessage() (message.MessageType, []byte, error) {
	retry := true
	for {
		s.upgradeLock.Lock()
		conn := s.conn
		s.upgradeLock.Unlock()

		mt, pt, bs, err := conn.Read()
		if err != nil {
			if errors.Is(err, transport.ErrClosed) && retry {
				s.logger.Debug("retry read: ", err)
				retry = false
				continue
			}
			return mt, nil, err
		}

		switch pt {
		case message.PTPong:
			s.pongCh <- struct{}{}
			continue
		case message.PTClose:
			s.clientClose = true
			s.Close()
			continue
		}
		return mt, bs, err
	}
}

func (s *Session) Upgrade(w http.ResponseWriter, r *http.Request, reqTransport transport.Transport) error {
	// stop heartbeat
	close(s.closeCh)

	// conn
	oldConn := s.conn
	newConn, err := reqTransport.Accept(w, r)
	if err != nil {
		return err
	}
	newConn.WithLogger(s.logger)

	// wait for ping
	s.logger.Debug("[UPGRADE] 0 ", time.Now().UnixMilli())
	mt, pt, bs, err := newConn.Read()
	if err != nil {
		return err
	}
	if mt != message.MTText || pt != message.PTPing {
		return errors.New("upgrade rcv ping error")
	}

	// send pong
	s.logger.Debug("[UPGRADE] 1 ", time.Now().UnixMilli())
	if err := newConn.Write(message.MTText, message.PTPong, bs); err != nil {
		return err
	}

	// pause old
	s.logger.Debug("[UPGRADE] 2 ", time.Now().UnixMilli())
	oldConn.Pause()

	// wait for upgrade
	s.logger.Debug("[UPGRADE] 3 ", time.Now().UnixMilli())
	mt, pt, _, err = newConn.Read()
	if err != nil {
		return err
	}
	if mt != message.MTText || pt != message.PTUpgrade {
		return errors.New("upgrade rcv upgrade error")
	}

	// replace conn
	s.logger.Debug("[UPGRADE] 4 ", time.Now().UnixMilli())

	s.upgradeLock.Lock()
	s.conn = newConn
	s.upgradeLock.Unlock()

	s.closeCh = make(chan struct{})
	go oldConn.Close(false)

	// restart heatbeat
	s.logger.Debug("[UPGRADE] 5 ", time.Now().UnixMilli())
	go s.Ping()

	return nil
}

func (s *Session) Transport() string {
	return s.conn.Name()
}

func (s *Session) Close() error {
	select {
	case <-s.closeCh:
		s.logger.Debug("Session already closed")
		return nil
	default:
		close(s.closeCh)
		s.logger.Debugf("Session close, noop=%v", s.clientClose)
		s.server.removeSession(s)
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
		if err := s.conn.Write(message.MTText, message.PTPing, nil); err != nil {
			s.logger.Debug("[Ping] error: ", err)
			return
		}
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

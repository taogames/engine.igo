package engineigo

import (
	"fmt"
	"net/http"
	"time"

	"github.com/taogames/engine.igo/transport"
	"github.com/taogames/engine.igo/transport/polling"
	"github.com/taogames/engine.igo/transport/websocket"
	"github.com/taogames/engine.igo/utils/idgen"
	"go.uber.org/zap"
)

const (
	EIO = "4"
)

type Server struct {
	pingInterval time.Duration
	pingTimeout  time.Duration
	maxPayload   int64
	transports   *transport.Manager

	sessCh  chan *Session
	sessMap map[string]*Session

	idGen  idgen.Generator
	logger *zap.SugaredLogger
}

type ServerOption func(o *Server)

func WithPingInterval(intv time.Duration) ServerOption {
	return func(s *Server) {
		s.pingInterval = intv
	}
}

func WithPingTimeout(timeout time.Duration) ServerOption {
	return func(s *Server) {
		s.pingTimeout = timeout
	}
}

func WithMaxPayload(payload int64) ServerOption {
	return func(s *Server) {
		s.maxPayload = payload
	}
}

func WithLogger(logger *zap.SugaredLogger) ServerOption {
	return func(s *Server) {
		s.logger = logger
	}
}

func NewServer(opts ...ServerOption) *Server {
	srv := &Server{
		pingInterval: 25 * time.Second,
		pingTimeout:  20 * time.Second,
		transports: transport.NewManager([]transport.Transport{
			polling.Default,
			websocket.Default,
		}),
		sessMap: make(map[string]*Session),
		sessCh:  make(chan *Session),
		idGen:   idgen.Default,
	}

	for _, o := range opts {
		o(srv)
	}

	if srv.logger == nil {
		logger, err := zap.NewProduction()
		if err != nil {
			panic(err)
		}
		srv.logger = logger.Sugar()
	}

	return srv
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	s.logger.Debugf("%-8s%s", r.Method, query.Encode())

	if reqEIO := query.Get("EIO"); reqEIO != EIO {
		errMsg := fmt.Sprintf("invalid EIO=%s", reqEIO)
		s.logger.Error(errMsg)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	reqTransportName := query.Get("transport")
	reqTransport, ok := s.transports.Get(reqTransportName)
	if !ok {
		errMsg := fmt.Sprintf("invalid transport=%s", reqTransportName)
		s.logger.Error(errMsg)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	sid := query.Get("sid")
	var (
		sess *Session
	)
	if sid == "" {
		if r.Method != http.MethodGet {
			errMsg := fmt.Sprintf("invalid handshake method=%s", r.Method)
			s.logger.Error(errMsg)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
		// 新连接
		conn, err := reqTransport.Accept(w, r)
		if err != nil {
			s.logger.Errorf("tranport %s accept: %s", reqTransportName, err.Error())
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		sess, err = s.newSession(conn)
		if err != nil {
			s.logger.Errorf("new session: %s", err.Error())
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
	} else {
		var ok bool
		sess, ok = s.sessMap[sid]
		if !ok {
			errMsg := fmt.Sprintf("session=%v not exist", sid)
			s.logger.Error(errMsg)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}

		// Upgrade
		if reqTransportName != sess.Transport() {
			if !s.transports.CanUpgrade(sess.Transport(), reqTransportName) {
				errMsg := fmt.Sprintf("session=%s cannot upgrade from %s to %s", sid, sess.Transport(), reqTransportName)
				s.logger.Error(errMsg)
				http.Error(w, errMsg, http.StatusBadRequest)
				return
			}
			if err := sess.Upgrade(w, r, reqTransport); err != nil {
				s.logger.Errorf("session=%s upgrade: %s", sid, err.Error())
				http.Error(w, "server error", http.StatusInternalServerError)
				return
			}
		} else if !sess.Unique(r.Method) {
			// Duplicate
			errMsg := fmt.Sprintf("session=%v duplicate method=%v", sid, r.Method)
			s.logger.Error(errMsg)
			http.Error(w, errMsg, http.StatusBadRequest)
			s.closeSession(sess)
			return
		} else {
			defer sess.UnlockMethod(r.Method)
		}
	}

	if err := sess.ServeHTTP(w, r); err != nil {
		s.logger.Errorf("session=%s ServeHTTP: %s", sess.id, err.Error())
		s.closeSession(sess)
	}
}

func (s *Server) Accept() <-chan *Session {
	return s.sessCh
}

type HandshakeConfig struct {
	Sid          string   `json:"sid"`
	PingInterval int64    `json:"pingInterval"`
	PingTimeout  int64    `json:"pingTimeout"`
	Upgrades     []string `json:"upgrades"`
	MaxPayload   int64    `json:"maxPayload"`
}

func (s *Server) newSession(conn transport.Conn) (*Session, error) {
	sid, err := s.idGen.NextID()
	if err != nil {
		return nil, err
	}

	sess := &Session{
		id:     sid,
		server: s,
		conn:   conn,
		logger: s.logger.With("sid", sid),
		conf: &HandshakeConfig{
			Sid:          sid,
			PingInterval: s.pingInterval.Milliseconds(),
			PingTimeout:  s.pingTimeout.Milliseconds(),
			Upgrades:     s.transports.Upgradable(conn.Name()),
			MaxPayload:   s.maxPayload,
		},
		pongCh:  make(chan struct{}),
		closeCh: make(chan struct{}),
	}

	go func() {
		sess.Init()

		s.sessMap[sid] = sess
		s.sessCh <- sess

		go sess.Ping()
	}()

	return sess, nil
}

func (s *Server) closeSession(sess *Session) {
	delete(s.sessMap, sess.id)
	sess.Close()
}

func (s *Server) removeSession(sess *Session) {
	delete(s.sessMap, sess.id)
}

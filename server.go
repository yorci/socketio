package engio

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"sync"
	"time"
)

type Server struct {
	transport    Transport
	pingInterval time.Duration
	pingTimeout  time.Duration
	ßchan        chan *session
	once         sync.Once
	*sessionManager
	*eventHandlers
}

func NewServer() (*Server, error) {
	s := &Server{
		transport:      WebsocketTransport,
		pingInterval:   time.Second * 25,
		pingTimeout:    time.Second * 5,
		ßchan:          make(chan *session, 1),
		sessionManager: newSessionManager(),
		eventHandlers:  newEventHandlers(),
	}
	go func() {
		for {
			select {
			case ß, ok := <-s.ßchan:
				if !ok {
					return
				}
				s.fire(ß.Socket, EventOpen, nil)
				go func() {
					for {
						if err := ß.Socket.Handle(); err != nil {
							println(err.Error())
							return
						}
					}
				}()
			}
		}
	}()
	return s, nil
}

func (s *Server) Close() (err error) {
	s.once.Do(func() {
		close(s.ßchan)
	})
	return
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	println(query.Encode())
	transport := query.Get("transport")
	sid := query.Get("sid")

	if transport != "websocket" {
		http.Error(w, "invalid transport", http.StatusBadRequest)
		return
	}
	var ß *session
	if sid == "" {
		conn, err := s.transport.Accept(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		ß = s.NewSession(conn)
		ß.Emit(EventOpen, &Parameters{
			SID:          ß.id,
			Upgrades:     []string{},
			PingInterval: int(s.pingInterval / time.Millisecond),
			PingTimeout:  int(s.pingTimeout / time.Millisecond),
		})
		s.ßchan <- ß
	} else {
		var exists bool
		ß, exists = s.sessionManager.Get(sid)
		if !exists {
			http.Error(w, "invalid session", http.StatusBadRequest)
		}
	}
	return
}

func (s *Server) BindAndListen(srv *http.Server) error {
	if srv == nil {
		panic("nil http server")
	}
	srv.Handler = s
	return srv.ListenAndServe()
}

type session struct {
	*Socket
	id string
}

func newSession(conn Conn) *session {
	id := generateRandomKey(24)
	return &session{
		Socket: &Socket{Conn: conn, eventHandlers: newEventHandlers()},
		id:     base64.StdEncoding.EncodeToString(id),
	}
}

type sessionManager struct {
	ß map[string]*session
	sync.RWMutex
}

func newSessionManager() *sessionManager {
	return &sessionManager{
		ß: make(map[string]*session),
	}
}

func (s *sessionManager) Get(id string) (ß *session, b bool) {
	s.RLock()
	ß, b = s.ß[id]
	s.RUnlock()
	return
}

func (s *sessionManager) Remove(id string) {
	s.Lock()
	delete(s.ß, id)
	s.Unlock()
}

func (s *sessionManager) NewSession(conn Conn) *session {
	ß := newSession(conn)
	s.Lock()
	s.ß[ß.id] = ß
	s.Unlock()
	return ß
}

func generateRandomKey(length int) []byte {
	k := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return nil
	}
	return k
}
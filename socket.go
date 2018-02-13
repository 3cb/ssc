// Package ssc creates and controls pools of websocket connections --  client and server
package ssc

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

// Socket type defines a websocket connection along with configuration and channels used to run read and write goroutines
type Socket struct {
	Connection  *websocket.Conn
	ID          string
	Pool2Socket chan Message
	Quit        chan struct{}
	R2W         chan struct{}
	W2R         chan struct{}
}

// NewSocketInstance returns a new instance of a Socket
func newSocketInstance(url string) *Socket {
	s := &Socket{
		ID:          url,
		Pool2Socket: make(chan Message),
		Quit:        make(chan struct{}),
		R2W:         make(chan struct{}),
		W2R:         make(chan struct{}),
	}
	return s
}

// connectClient connects to a websocket using websocket.Upgrader.Upgrade() method and starts goroutine/s for read and write
func (s *Socket) connectClient(p *SocketPool, upgrader *websocket.Upgrader, w http.ResponseWriter, r *http.Request) (bool, error) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return false, err
	}
	s.Connection = c
	s.Connection.SetPongHandler(func(appData string) error {
		p.Pingers.mtx.Lock()
		p.Pingers.Stack[s]--
		p.Pingers.mtx.Unlock()
		return nil
	})
	s.Connection.SetCloseHandler(func(code int, text string) error {
		addr := s.Connection.RemoteAddr()
		fmt.Printf("websocket at %v has been closed: %v", addr, code)
		return nil
	})

	p.Readers.mtx.Lock()
	go s.readSocket(p.Pipes)
	p.Readers.Stack[s] = true
	p.Readers.mtx.Unlock()

	p.Writers.mtx.Lock()
	go s.writeSocket(p.Pipes)
	p.Writers.Stack[s] = true
	p.Writers.mtx.Unlock()

	if p.PingInterval > 0 {
		p.Pingers.mtx.Lock()
		p.Pingers.Stack[s] = 0
		p.Pingers.mtx.Unlock()
	}

	return true, nil
}

// connectServer connects to websocket given a url string from SocketPool.
// starts goroutines for read and write
func (s *Socket) connectServer(p *SocketPool) (bool, error) {
	c, resp, err := websocket.DefaultDialer.Dial(s.ID, nil)
	if resp.StatusCode != 101 || err != nil {
		return false, err
	}
	s.Connection = c
	s.Connection.SetPingHandler(func(appData string) error {
		s.Pool2Socket <- Message{Type: websocket.PongMessage, Payload: []byte("")}
		return nil
	})
	s.Connection.SetCloseHandler(func(code int, text string) error {
		addr := s.Connection.RemoteAddr()
		fmt.Printf("websocket at %v has been closed: %v", addr, code)
		return nil
	})

	p.Readers.mtx.Lock()
	go s.readSocket(p.Pipes)
	p.Readers.Stack[s] = true
	p.Readers.mtx.Unlock()

	p.Writers.mtx.Lock()
	go s.writeSocket(p.Pipes)
	p.Writers.Stack[s] = true
	p.Writers.mtx.Unlock()

	if p.PingInterval > 0 {
		p.Pingers.mtx.Lock()
		p.Pingers.Stack[s] = 0
		p.Pingers.mtx.Unlock()
	}

	return true, nil
}

// readSocket runs a continuous loop that reads messages from websocket and sends the []byte to the Pool controller
// It also listens for shutdown command from Pool and will close connection on command and also close connection on any errors reading from websocket
func (s *Socket) readSocket(pipes *Pipes) {
	for {
		select {
		case <-s.W2R:
			return
		case <-s.Quit:
			s.R2W <- struct{}{}
			s.closeSocket()
			pipes.Error <- ErrorMsg{s, nil}
			return
		default:
			msgType, msg, err := s.Connection.ReadMessage()
			if err != nil {
				s.R2W <- struct{}{}
				s.closeSocket()
				pipes.Error <- ErrorMsg{s, err}
				return
			}
			pipes.Socket2Pool <- Message{ID: s.ID, Type: msgType, Payload: msg}
		}
	}
}

// writeSocket runs a continuous loop that reads []byte messages from the FromPool channel and writes them to the websocket
// It also listens for shutdown command from Pool and will close connection on command and also close connection on any errors writing from websocket
func (s *Socket) writeSocket(pipes *Pipes) {
	for {
		select {
		case <-s.R2W:
			return
		case <-s.Quit:
			s.W2R <- struct{}{}
			s.closeSocket()
			pipes.Error <- ErrorMsg{s, nil}
			return
		case msg := <-s.Pool2Socket:
			err := s.Connection.WriteMessage(msg.Type, msg.Payload)
			if err != nil {
				s.W2R <- struct{}{}
				s.closeSocket()
				pipes.Error <- ErrorMsg{s, err}
				return
			}
		}
	}
}

// closeSocket closes the websocket connection
func (s *Socket) closeSocket() {
	err := s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		s.Connection.Close()
	}
}

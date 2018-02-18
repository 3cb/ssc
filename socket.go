// Package ssc creates and controls pools of websocket connections --  client and server
package ssc

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// Socket type defines a websocket connection along with configuration and channels used to run read/write goroutines
type socket struct {
	id          string
	connection  *websocket.Conn
	pool2Socket chan Message
	rQuit       chan struct{}
	wQuit       chan struct{}
	errors      []error
}

// newSocketInstance returns a new instance of a Socket
func newSocketInstance(id string) *socket {
	return &socket{
		id:          id,
		pool2Socket: make(chan Message),
		rQuit:       make(chan struct{}),
		wQuit:       make(chan struct{}),
		errors:      []error{},
	}
}

// connectClient connects to a websocket using websocket.Upgrader.Upgrade() method and starts goroutine/s for read and write
func (s *socket) connectClient(p *SocketPool, upgrader *websocket.Upgrader, w http.ResponseWriter, r *http.Request) error {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	s.connection = c
	s.connection.SetPongHandler(func(appData string) error {
		p.rw.mtx.Lock()
		p.rw.stack[s]--
		p.rw.mtx.Unlock()
		return nil
	})
	s.connection.SetCloseHandler(func(code int, text string) error {
		addr := s.connection.RemoteAddr()
		fmt.Printf("websocket at %v has been closed: %v\n", addr, code)
		return nil
	})

	p.rw.mtx.Lock()
	p.rw.stack[s] = 0
	p.rw.mtx.Unlock()

	p.ping.mtx.Lock()
	p.ping.stack[s] = 0
	p.ping.mtx.Unlock()

	go s.read(p)
	go s.write(p)

	return nil
}

// connectServer connects to websocket given a url string from SocketPool.
// starts goroutines for read and write
func (s *socket) connectServer(p *SocketPool) error {
	c, resp, err := websocket.DefaultDialer.Dial(s.id, nil)
	if resp.StatusCode != 101 || err != nil {
		return err
	}
	s.connection = c
	s.connection.SetPingHandler(func(appData string) error {
		s.pool2Socket <- Message{Type: websocket.PongMessage, Payload: []byte("")}
		return nil
	})
	s.connection.SetCloseHandler(func(code int, text string) error {
		addr := s.connection.RemoteAddr()
		fmt.Printf("websocket at %v has been closed: %v\n", addr, code)
		return nil
	})

	p.rw.mtx.Lock()
	p.rw.stack[s] = 0
	p.rw.mtx.Unlock()

	p.ping.mtx.Lock()
	p.ping.stack[s] = 0
	p.ping.mtx.Unlock()

	go s.read(p)
	go s.write(p)

	return nil
}

// read runs a continuous loop that reads messages from websocket and sends the []byte to the Pool controller
// It also listens for shutdown command from Pool and will close connection on command and also close connection on any errors reading from websocket
func (s *socket) read(p *SocketPool) {
	for {
		select {
		case <-s.rQuit:
			log.Printf("quit signal received by %v READ Goroutine. shutting down.\n", s.id)
			p.shutdown <- s
			return
		default:
			msgType, msg, err := s.connection.ReadMessage()
			if err != nil {
				s.wQuit <- struct{}{}
				s.errors = append(s.errors, err)
				p.shutdown <- s
				return
			}
			p.s2p <- Message{ID: s.id, Type: msgType, Payload: msg}
		}
	}
}

// write runs a continuous loop that reads []byte messages from the FromPool channel and writes them to the websocket
// It also listens for shutdown command from Pool and will close connection on command and also close connection on any errors writing from websocket
func (s *socket) write(p *SocketPool) {
	for {
		select {
		case <-s.wQuit:
			log.Printf("quit signal received by %v WRITE goroutine. shutting down.\n", s.id)
			p.shutdown <- s
			return
		case msg := <-s.pool2Socket:
			err := s.connection.WriteMessage(msg.Type, msg.Payload)
			if err != nil {
				s.rQuit <- struct{}{}
				s.errors = append(s.errors, err)
				p.shutdown <- s
				return
			}
		}
	}
}

// close closes the websocket connection
func (s *socket) close() bool {
	err := s.connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		s.connection.Close()
	}
	return true
}

// Package ssc creates and controls pools of websocket connections --  client and server
package ssc

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// Socket type defines a websocket connection along with configuration and channels used to run read/write goroutines
type socket struct {
	// id is either url string for server connections or string generated by caller application for client connections
	id         string
	connection *websocket.Conn

	// p2s carries massages from pool to socket
	p2s chan *Message

	// quit channels give read/write goroutines signal to return
	rQuit chan struct{}
	wQuit chan struct{}

	// done channel signals controlShutdown that read/write goroutines have stopped
	done chan struct{}

	// errors slice contains any errors generated by the websocket connection
	errors []error
}

// newSocketInstance returns a new instance of a socket
func newSocketInstance(id string) *socket {
	return &socket{
		id:     id,
		p2s:    make(chan *Message, 250),
		rQuit:  make(chan struct{}, 2),
		wQuit:  make(chan struct{}, 2),
		done:   make(chan struct{}, 2),
		errors: []error{},
	}
}

// connectClient connects to a websocket using websocket.Upgrader.Upgrade() method and starts goroutines for read and write
func (s *socket) connectClient(p *Pool, upgrader *websocket.Upgrader, w http.ResponseWriter, r *http.Request) error {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	s.connection = c
	s.connection.SetPongHandler(func(appData string) error {
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

	go s.read(p)
	go s.write(p)

	return nil
}

// connectServer connects to websocket given a url string from Pool.
// starts goroutines for read and write
func (s *socket) connectServer(p *Pool) error {
	c, resp, err := websocket.DefaultDialer.Dial(s.id, nil)
	if resp.StatusCode != 101 || err != nil {
		return err
	}
	s.connection = c
	s.connection.SetPingHandler(func(appData string) error {
		s.p2s <- &Message{Type: websocket.PongMessage, Payload: []byte("")}
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

	go s.read(p)
	go s.write(p)

	return nil
}

// read runs a continuous loop that reads messages from websocket and sends the []byte to the Pool controller
// It also listens for quit command from Pool and will close connection on command and also close connection on any errors reading from websocket
func (s *socket) read(p *Pool) {
	for {
		select {
		case <-s.rQuit:
			s.done <- struct{}{}
			p.shutdown <- s
			return
		default:
			msgType, msg, err := s.connection.ReadMessage()
			if err != nil {
				s.wQuit <- struct{}{}
				s.errors = append(s.errors, err)
				s.done <- struct{}{}
				p.shutdown <- s
				return
			}
			p.s2p <- &Message{ID: s.id, Type: msgType, Payload: msg}
		}
	}
}

// write runs a continuous loop that receives []byte messages from the Pool and writes them to the websocket
// It also listens for quit command from Pool and will close connection on command and also close connection on any errors writing from websocket
func (s *socket) write(p *Pool) {
	for {
		select {
		case <-s.wQuit:
			s.done <- struct{}{}
			p.shutdown <- s
			return
		case msg := <-s.p2s:
			err := s.connection.WriteMessage(msg.Type, msg.Payload)
			if err != nil {
				s.rQuit <- struct{}{}
				s.errors = append(s.errors, err)
				s.done <- struct{}{}
				p.shutdown <- s
				return
			}
		}
	}
}

// close closes the websocket connection
func (s *socket) close() {
	s.connection.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{})
	s.connection.Close()
}

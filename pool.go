// Package ssc creates and controls pools of websocket connections --  client and server
package ssc

import (
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Message contains type and payload as a normal websocket message
// ID is added in case user needs to identify source
type Message struct {
	ID      string
	Type    int
	Payload []byte
}

// SocketPool is a collection of websocket connections combined with
// channels which are used to send and received messages to and from
// the goroutines that control them
type SocketPool struct {
	mtx        sync.RWMutex
	isDraining bool
	rw
	ping

	// These fields are set with NewSocketPool parameters
	serverURLs   []string
	pingInterval time.Duration

	// Inbound channel carries messages from caller application to WriteControl() method
	Inbound chan *Message

	// Outbound channel carries messages from ReadControl() method to caller application
	Outbound chan *Message

	// Socket2Pool channel carries messages from Read goroutines to ReadControl() method
	s2p chan *Message

	// Stop channels are used to stop control goroutines
	stopReadControl     chan *sync.WaitGroup
	stopWriteControl    chan *sync.WaitGroup
	stopShutdownControl chan *sync.WaitGroup
	stopPingControl     chan *sync.WaitGroup

	// Shutdown channel carries messages from read/write goroutines to ControlShutdown() goroutine
	shutdown chan *socket

	// allClosed alerts pool.Drain() that all read/write goroutines have been stopped and all websockets have been disconnected
	allClosed chan struct{}
}

// RW contains map of sockets with read and write goroutines
type rw struct {
	mtx   sync.RWMutex
	stack map[*socket]int
}

// Ping holds map of sockets to keep track of unreturned pings
type ping struct {
	mtx   sync.RWMutex
	stack map[*socket]int
}

// NewSocketPool creates a new instance of SocketPool and returns a pointer to it and an error
// If slice of urls is nil or empty SocketPool will be created empty and control methods will be launched and waiting
func NewSocketPool(urls []string, pingInt time.Duration) *SocketPool {
	return &SocketPool{
		isDraining: false,
		rw:         rw{stack: make(map[*socket]int)},
		ping:       ping{stack: make(map[*socket]int)},

		serverURLs:   urls,
		pingInterval: pingInt,

		Inbound:             make(chan *Message, 200),
		Outbound:            make(chan *Message, 200),
		s2p:                 make(chan *Message, 200),
		stopReadControl:     make(chan *sync.WaitGroup),
		stopWriteControl:    make(chan *sync.WaitGroup),
		stopShutdownControl: make(chan *sync.WaitGroup),
		stopPingControl:     make(chan *sync.WaitGroup),
		shutdown:            make(chan *socket, 200),
		allClosed:           make(chan struct{}),
	}
}

// Start spins up control goroutines and connects to websockets(if server pool)
func (p *SocketPool) Start() error {
	p.Control()

	if len(p.serverURLs) > 0 {
		for _, url := range p.serverURLs {
			s := newSocketInstance(url)
			err := s.connectServer(p)
			if err != nil {
				p.Drain()
				return err
			}
			log.Printf("Connected to websocket(%v)\n", url)
		}
	}
	return nil
}

// AddClientSocket allows caller to add individual websocket connections to an existing pool of connections
// New connection will adopt existing pool configuration(SocketPool.Config)
func (p *SocketPool) AddClientSocket(id string, upgrader *websocket.Upgrader, w http.ResponseWriter, r *http.Request) error {
	if p.isDraining {
		return errors.New("pool is draining -- cannot add new websocket connection")
	}
	s := newSocketInstance(id)
	err := s.connectClient(p, upgrader, w, r)
	if err != nil {
		return err
	}
	return nil
}

// AddServerSocket allows caller to add individual websocket connections to an existing pool of connections
// New connection will adopt existing pool configuration(SocketPool.Config)
func (p *SocketPool) AddServerSocket(url string) error {
	if p.isDraining {
		return errors.New("pool is draining -- cannot add new websocket connection")
	}
	s := newSocketInstance(url)
	err := s.connectServer(p)
	if err != nil {
		return err
	}
	return nil
}

// Drain shuts down all read and write goroutines as well as all control goroutines
func (p *SocketPool) Drain() {
	p.mtx.Lock()
	p.isDraining = true
	p.mtx.Unlock()

	wg := &sync.WaitGroup{}
	if p.pingInterval > 0 {
		wg.Add(1)
		p.stopPingControl <- wg
		wg.Wait()
	}
	p.rw.mtx.RLock()
	for s := range p.rw.stack {
		s.rQuit <- struct{}{}
		s.wQuit <- struct{}{}
	}
	p.rw.mtx.RUnlock()

	<-p.allClosed

	wg.Add(3)
	p.stopReadControl <- wg
	p.stopWriteControl <- wg
	p.stopShutdownControl <- wg
	wg.Wait()
}

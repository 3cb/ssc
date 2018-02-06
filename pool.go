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

// SocketPool is a collection of websocket connections combined with
// channels which are used to send and received messages to and from
// the goroutines that control them
type SocketPool struct {
	Locked bool
	Readers
	Writers
	Pingers
	ClosedURLs
	*Pipes
	Config
}

// Readers contains map of sockets with read goroutines
type Readers struct {
	mtx   sync.RWMutex
	Stack map[*Socket]bool
}

// Writers contains map of sockets with write goroutines
type Writers struct {
	mtx   sync.RWMutex
	Stack map[*Socket]bool
}

// Pingers contains map of sockets being pinged
type Pingers struct {
	mtx   sync.RWMutex
	Stack map[*Socket]int
}

// ClosedURLs contains map of all URLs of closed server websockets
type ClosedURLs struct {
	mtx   sync.RWMutex
	Stack map[string]bool
}

// Pipes contains data communication channels:
type Pipes struct {
	// Inbound channel carries messages from caller application to WriteControl() method
	Inbound chan Message

	// Outbound channel carries messages from ReadControl() method to caller application
	Outbound chan Message

	// Socket2Pool channel carries messages from Read goroutines to ReadControl() method
	Socket2Pool chan Message

	// Stop channels are used to shutdown control goroutines
	StopReadControl     chan *sync.WaitGroup
	StopWriteControl    chan *sync.WaitGroup
	StopShutdownControl chan *sync.WaitGroup
	StopPingControl     chan *sync.WaitGroup

	// Error channels carry messages from read/write goroutines to ControlShutdown() goroutine
	ErrorRead  chan ErrorMsg
	ErrorWrite chan ErrorMsg
}

// Config is used to pass configuration settings to the Pool initialization function
type Config struct {
	ServerURLs   []string
	IsReadable   bool
	IsWritable   bool
	PingInterval time.Duration //minimum of 30 seconds
}

// NewSocketPool creates a new instance of SocketPool and returns a pointer to it and an error
// If slice of urls is nil or empty SocketPool will be created empty and control methods will be launched and waiting
func NewSocketPool(config Config) (*SocketPool, error) {
	if !config.IsReadable && !config.IsWritable {
		err := errors.New("bad input values: Sockets cannot be both unreadable and unwritable")
		return nil, err
	}

	pipes := &Pipes{}
	pipes.Inbound = make(chan Message)
	pipes.Outbound = make(chan Message)
	pipes.Socket2Pool = make(chan Message)

	pipes.StopReadControl = make(chan *sync.WaitGroup)
	pipes.StopWriteControl = make(chan *sync.WaitGroup)
	pipes.StopShutdownControl = make(chan *sync.WaitGroup)
	pipes.StopPingControl = make(chan *sync.WaitGroup)

	pipes.ErrorRead = make(chan ErrorMsg, 100)
	pipes.ErrorWrite = make(chan ErrorMsg, 100)

	pool := &SocketPool{
		Readers:    Readers{Stack: make(map[*Socket]bool)},
		Writers:    Writers{Stack: make(map[*Socket]bool)},
		Pingers:    Pingers{Stack: make(map[*Socket]int)},
		ClosedURLs: ClosedURLs{Stack: make(map[string]bool)},
		Pipes:      pipes,
		Config:     config,
	}

	pool.Control()

	if len(config.ServerURLs) > 0 {
		for _, url := range config.ServerURLs {
			s := newSocketInstance(url, config)
			success, err := s.connectServer(pool)
			if success {
				log.Printf("Connected to websocket(%v)\n", url)
			} else {
				log.Printf("Error connecting to websocket(%v): %v\n", url, err)
			}
		}
	}

	return pool, nil
}

// Drain shuts down all read and write goroutines as well as all control goroutines
func (p *SocketPool) Drain() {
	p.Locked = true
	p.Readers.mtx.RLock()
	for s, active := range p.Readers.Stack {
		if active {
			s.ShutdownRead <- struct{}{}
		}
	}
	p.Readers.mtx.RUnlock()

	wg := &sync.WaitGroup{}
	var count int
	if p.Config.PingInterval > 0 {
		count = 1
	} else {
		count = 0
	}
	wg.Add(3 + count)
	p.StopReadControl <- wg
	p.StopWriteControl <- wg
	p.StopShutdownControl <- wg
	if p.Config.PingInterval > 0 {
		p.StopPingControl <- wg
	}
	wg.Wait()
}

// AddClientSocket allows caller to add individual websocket connections to an existing pool of connections
// New connection will adopt existing pool configuration(SocketPool.Config)
func (p *SocketPool) AddClientSocket(upgrader *websocket.Upgrader, w http.ResponseWriter, r *http.Request) (*Socket, error) {
	if p.Locked {
		return nil, errors.New("pool is locked. cannot add new websocket connection")
	}
	s := newSocketInstance("", p.Config)
	success, err := s.connectClient(p, upgrader, w, r)
	if success {
		return s, nil
	}
	return nil, err
}

// AddServerSocket allows caller to add individual websocket connections to an existing pool of connections
// New connection will adopt existing pool configuration(SocketPool.Config)
func (p *SocketPool) AddServerSocket(url string) (*Socket, error) {
	if p.Locked {
		return nil, errors.New("pool is locked. cannot add new websocket connection")
	}
	s := newSocketInstance(url, p.Config)
	success, err := s.connectServer(p)
	if success {
		log.Printf("Connected to websocket(%v)\n", url)
	} else {
		log.Printf("Error connecting to websocket(%v): %v\n", url, err)
	}
	return s, nil
}

func (p *SocketPool) isPingStackEmpty() bool {
	p.Pingers.mtx.RLock()
	defer p.Pingers.mtx.RUnlock()
	if len(p.Pingers.Stack) > 0 {
		return false
	}
	return true
}

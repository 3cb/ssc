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
	Readers
	Writers
	Pingers
	ClosedURLs
	// Contains SocketPools communication channels
	Pipes *Pipes
	// Contains SocketPool's configuration object
	Config PoolConfig
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
// Error channel carries websocket error messages from goroutines back to pool controller
type Pipes struct {
	// Inbound channels carry messages from caller application to WriteControl() method
	InboundBytes chan Message
	InboundJSON  chan JSONReaderWriter

	// Outbound channels carry messages from ReadControl() method to caller application
	OutboundBytes chan Message
	OutboundJSON  chan JSONReaderWriter

	// Socket2Pool channels carry messages from Read goroutines to ReadControl() method
	Socket2PoolBytes chan Message
	Socket2PoolJSON  chan JSONReaderWriter

	// Pong carries Socket instance to ControlPong goroutine
	Pong chan *Socket

	// Stop channels are used to shutdown control goroutines
	StopReadControl     chan struct{}
	StopWriteControl    chan struct{}
	StopShutdownControl chan struct{}
	StopPingControl     chan struct{}
	StopPongControl     chan struct{}

	// Error channels carry messages from read/write goroutines to ControlShutdown() goroutine
	ErrorRead  chan ErrorMsg
	ErrorWrite chan ErrorMsg
}

// PoolConfig is used to pass configuration settings to the Pool initialization function
type PoolConfig struct {
	ServerURLs   []string
	IsReadable   bool
	IsWritable   bool
	IsJSON       bool // If false, messages will be read/written in bytes
	DataJSON     JSONReaderWriter
	PingInterval time.Duration //minimum of 30 seconds
}

// NewSocketPool creates a new instance of SocketPool and returns a pointer to it and an error
// If slice of urls is nil or empty SocketPool will be created and control methods will be launched and waiting
func NewSocketPool(config PoolConfig) (*SocketPool, error) {
	if !config.IsReadable && !config.IsWritable {
		err := errors.New("bad input values: Sockets cannot be both unreadable and unwritable")
		return nil, err
	}
	if config.IsJSON && config.DataJSON == nil {
		err := errors.New("if data type is JSON you must pass in values for DataJSON and JSON channels that implement JSONReaderWriter interface")
		return nil, err
	}
	pipes := &Pipes{}
	if config.IsJSON == true {
		pipes.InboundJSON = make(chan JSONReaderWriter)
		pipes.OutboundJSON = make(chan JSONReaderWriter)
		pipes.Socket2PoolJSON = make(chan JSONReaderWriter)
	} else {
		pipes.InboundBytes = make(chan Message)
		pipes.OutboundBytes = make(chan Message)
		pipes.Socket2PoolBytes = make(chan Message)
	}
	pipes.Pong = make(chan *Socket)

	pipes.StopReadControl = make(chan struct{})
	pipes.StopWriteControl = make(chan struct{})
	pipes.StopShutdownControl = make(chan struct{})
	pipes.StopPingControl = make(chan struct{})
	pipes.StopPongControl = make(chan struct{})

	pipes.ErrorRead = make(chan ErrorMsg)
	pipes.ErrorWrite = make(chan ErrorMsg)

	pool := &SocketPool{
		Readers:    Readers{Stack: make(map[*Socket]bool)},
		Writers:    Writers{Stack: make(map[*Socket]bool)},
		Pingers:    Pingers{Stack: make(map[*Socket]int)},
		ClosedURLs: ClosedURLs{Stack: make(map[string]bool)},
		Pipes:      pipes,
		Config:     config,
	}

	go pool.Control()

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

// AddClientSocket allows caller to add individual websocket connections to an existing pool of connections
// New connection will adopt existing pool configuration(SocketPool.Config)
func (p *SocketPool) AddClientSocket(upgrader *websocket.Upgrader, w http.ResponseWriter, r *http.Request) (*Socket, error) {
	s := newSocketInstance("", p.Config)
	success, err := s.connectClient(p, upgrader, w, r)
	if success {
		return s, nil
	}
	return s, err
}

// AddServerSocket allows caller to add individual websocket connections to an existing pool of connections
// New connection will adopt existing pool configuration(SocketPool.Config)
func (p *SocketPool) AddServerSocket(url string) {
	s := newSocketInstance(url, p.Config)
	success, err := s.connectServer(p)
	if success {
		log.Printf("Connected to websocket(%v)\n", url)
	} else {
		log.Printf("Error connecting to websocket(%v): %v\n", url, err)
	}
}

func (p *SocketPool) isPingStackEmpty() bool {
	p.Pingers.mtx.RLock()
	defer p.Pingers.mtx.RUnlock()
	if len(p.Pingers.Stack) > 0 {
		return false
	}
	return true
}

package ssc

import (
	"errors"
	"log"
	"time"
)

// SocketPool is a collection of websocket connections combined with
// channels which are used to send and received messages to and from
// the goroutines that control them
type SocketPool struct {
	OpenStack    map[string]*Socket
	ClosedStack  map[string]*Socket
	ClosingQueue map[string]string // 'read' or 'write'
	Pipes        *Pipes
	Config       PoolConfig
}

// Pipes contains data communication channels:
// Error channel carries websocket error messages from goroutines back to pool controller
type Pipes struct {
	InboundBytes        chan Data
	InboundJSON         chan JSONReaderWriter
	OutboundBytes       chan Data
	OutboundJSON        chan JSONReaderWriter
	FromSocketBytes     chan Data
	FromSocketJSON      chan JSONReaderWriter
	StopReadControl     chan struct{}
	StopWriteControl    chan struct{}
	StopShutdownControl chan struct{}
	ErrorRead           chan ErrorMsg
	ErrorWrite          chan ErrorMsg
}

// PoolConfig is used to pass configuration settings to the Pool initialization function
type PoolConfig struct {
	IsReadable bool
	IsWritable bool
	IsJSON     bool // If false, messages will be read/written in bytes
	DataJSON   JSONReaderWriter
}

// NewSocketPool creates a new instance of SocketPool and returns a pointer to it and an error
func NewSocketPool(urls []string, config PoolConfig) (*SocketPool, error) {
	if config.IsReadable == false && config.IsWritable == false {
		err := errors.New("bad input values: Sockets cannot be both unreadable and unwritable")
		return nil, err
	}
	if config.IsJSON == true && config.DataJSON == nil {
		err := errors.New("if data type is JSON you must pass in values for DataJSON and JSON channels that implement JSONReaderWriter interface")
		return nil, err
	}
	pipes := &Pipes{}
	if config.IsJSON == true {
		pipes.InboundJSON = make(chan JSONReaderWriter)
		pipes.OutboundJSON = make(chan JSONReaderWriter)
		pipes.FromSocketJSON = make(chan JSONReaderWriter)
	} else {
		pipes.InboundBytes = make(chan Data)
		pipes.OutboundBytes = make(chan Data)
		pipes.FromSocketBytes = make(chan Data)
	}
	pipes.StopReadControl = make(chan struct{})
	pipes.StopWriteControl = make(chan struct{})
	pipes.StopShutdownControl = make(chan struct{})
	pipes.ErrorRead = make(chan ErrorMsg)
	pipes.ErrorWrite = make(chan ErrorMsg)

	pool := &SocketPool{
		OpenStack:    make(map[string]*Socket, len(urls)),
		ClosedStack:  make(map[string]*Socket, len(urls)),
		ClosingQueue: make(map[string]string, len(urls)),
		Pipes:        pipes,
		Config:       config,
	}

	for _, v := range urls {
		s := newSocketInstance(v, config)
		success, err := s.connect(pipes, config)
		if success == true {
			pool.OpenStack[v] = s
			log.Printf("Connected to websocket(%v)\nAdded to Open Stack", s.URL)
		} else {
			pool.ClosedStack[v] = s
			log.Printf("Error connecting to websocket(%v): %v\nAdded to Closed Stack", s.URL, err)
		}
	}

	go pool.Control()

	return pool, nil
}

// AddSocket allows caller to add individual websocket connections to an existing pool of connections
// New connection will adopt existing pool configuration(SocketPool.Config)
func (p *SocketPool) AddSocket(url string) {
	s := newSocketInstance(url, p.Config)
	success, err := s.connect(p.Pipes, p.Config)
	if success {
		p.OpenStack[url] = s
		log.Printf("Connected to websocket(%v)\nAdded to Open Stack", url)
	} else {
		p.ClosedStack[url] = s
		log.Printf("Error connecting to websocket(%v): %v\nAdded to Closed Stack", url, err)
	}
}

// ShutdownSocket allows caller to send shutdown signal to goroutines managing reads and writes to websocket
// Goroutines will close websocket connection and then return
func (p *SocketPool) ShutdownSocket(url string) {
	_, ok := p.ClosedStack[url]
	if ok {
		return
	}

	s := p.OpenStack[url]
	closed, ok := p.ClosingQueue[url]
	if ok && closed == "read" {
		s.ShutdownWrite <- struct{}{}
	} else if ok && closed == "write" {
		s.ShutdownRead <- struct{}{}
	} else {
		s.ShutdownRead <- struct{}{}
		s.ShutdownWrite <- struct{}{}
	}
}

// RemoveSocket allows caller to remove an individual websocket connection from a SocketPool instance
// Function will send shutdown message and wait for confirmation from SocketPool.RemovalComplete channel
// Method deletes Socket connection from ClosedStack before it exits
func (p *SocketPool) RemoveSocket(url string) {
	defer func() {
		delete(p.ClosedStack, url)
		log.Printf("Connection to websocket at %v has been closed and removed from Pool.  %v\n", url, time.Now())
	}()

	p.ShutdownSocket(url)
	for {
		_, ok := p.ClosedStack[url]
		if ok {
			return
		}
	}
}

func (p *SocketPool) checkOpenStack(url string) *Socket {
	s, ok := p.OpenStack[url]
	if ok {
		return s
	}
	return nil
}

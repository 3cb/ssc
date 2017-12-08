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
	StopControlRead     chan struct{}
	StopControlWrite    chan struct{}
	StopControlShutdown chan struct{}
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
	pipes.StopControlRead = make(chan struct{})
	pipes.StopControlWrite = make(chan struct{})
	pipes.StopControlShutdown = make(chan struct{})
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

	return pool, nil
}

// Control method launches ControlShutdown(), ControlRead(), and ControlWrite()
func (p *SocketPool) Control() {
	go p.ControlShutdown()
	if p.Config.IsReadable {
		go p.ControlRead()
	}
	if p.Config.IsWritable {
		go p.ControlWrite()
	}
}

// ControlRead runs an infinite loop to take messages from websocket servers and send them to the outbound channel
func (p *SocketPool) ControlRead() {
	defer func() {
		log.Printf("ControlRead goroutine was stopped at %v.\n", time.Now())
	}()
	if p.Config.IsJSON {
		for {
			select {
			case <-p.Pipes.StopControlRead:
				return
			case v := <-p.Pipes.FromSocketJSON:
				p.Pipes.OutboundJSON <- v
			default:
				continue
			}
		}
	} else {
		for {
			select {
			case <-p.Pipes.StopControlRead:
				return
			case v := <-p.Pipes.FromSocketBytes:
				p.Pipes.OutboundBytes <- v
			default:
				continue
			}
		}
	}
}

// ControlWrite runs an infinite loop to take messages from inbound channel and send to write goroutines
func (p *SocketPool) ControlWrite() {
	defer func() {
		log.Printf("ControlWrite goroutine was stopped at %v.\n", time.Now())
	}()
	if p.Config.IsJSON {
		for {
			select {
			case <-p.Pipes.StopControlWrite:
				return
			case v := <-p.Pipes.InboundJSON:
				for _, socket := range p.OpenStack {
					socket.FromPoolJSON <- v
				}
			default:
				continue
			}
		}
	} else {
		for {
			select {
			case <-p.Pipes.StopControlWrite:
				return
			case v := <-p.Pipes.InboundBytes:
				for _, socket := range p.OpenStack {
					socket.FromPoolBytes <- v
				}
			default:
				continue
			}
		}
	}
}

// ControlShutdown method listens for Error Messages and dispatches shutdown messages
func (p *SocketPool) ControlShutdown() {
	defer func() {
		log.Printf("ControlShutdown goroutine was stopped at %v.\n", time.Now())
	}()
	for {
		select {
		case <-p.Pipes.StopControlShutdown:
			return
		case e := <-p.Pipes.ErrorRead:
			s := p.checkOpenStack(e.URL)
			if s != nil {
				rw, ok := p.ClosingQueue[e.URL]
				if ok == false {
					if s.IsWritable == true {
						p.ClosingQueue[e.URL] = "read"
						sock := p.OpenStack[e.URL]
						sock.ShutdownWrite <- struct{}{}
					} else {
						s.ClosedAt = time.Now()
						delete(p.OpenStack, e.URL)
						p.ClosedStack[e.URL] = s
					}
				} else if ok == true && rw == "write" {
					delete(p.ClosingQueue, e.URL)
					s.ClosedAt = time.Now()
					delete(p.OpenStack, e.URL)
					p.ClosedStack[e.URL] = s
				}
			}
		case e := <-p.Pipes.ErrorWrite:
			s := p.checkOpenStack(e.URL)
			if s != nil {
				rw, ok := p.ClosingQueue[e.URL]
				if ok == false {
					if s.IsReadable == true {
						p.ClosingQueue[e.URL] = "write"
						sock := p.OpenStack[e.URL]
						sock.ShutdownRead <- struct{}{}
					} else {
						s.ClosedAt = time.Now()
						delete(p.OpenStack, e.URL)
						p.ClosedStack[e.URL] = s
					}
				} else if ok == true && rw == "read" {
					delete(p.ClosingQueue, e.URL)
					s.ClosedAt = time.Now()
					delete(p.OpenStack, e.URL)
					p.ClosedStack[e.URL] = s
				}
			}
		default:
			continue
		}
	}
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

// RemoveSocket allows caller to remove an individual websocmet connection from a SocketPool instance
// Function will send shutdown message and listen for confirmation through error channel
func (p *SocketPool) RemoveSocket(url string) {
	_, ok := p.ClosedStack[url]
	if ok {
		delete(p.ClosedStack, url)
		return
	}
	s, ok := p.OpenStack[url]
	if ok {
		p.listenDeleteClosedStack(url)
		s.ShutdownRead <- struct{}{}
		s.ShutdownWrite <- struct{}{}
		return
	}
	closed, ok := p.ClosingQueue[url]
	if ok && closed == "read" {
		// ======================================================
	} else if ok && closed == "write" {
		// ======================================================
	}
}

func (p *SocketPool) checkOpenStack(url string) *Socket {
	s, ok := p.OpenStack[url]
	if ok {
		return s
	}
	return nil
}

func (p *SocketPool) listenDeleteClosedStack(url string) {
	for {
		_, ok := p.ClosedStack[url]
		if ok {
			delete(p.ClosedStack, url)
			return
		}
	}
}

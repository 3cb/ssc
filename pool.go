package ssc

import (
	"errors"
	"time"
)

// SocketPool is a collection of websocket connections combined with 4 channels which are used to send and received messages to and from the goroutines that control them
type SocketPool struct {
	OpenStack    map[string]*Socket
	ClosedStack  map[string]*Socket
	Pipes        *Pipes // {ToPool chan Data, FromPool chan Data, Shutdown chan string, Error chan ErrorMsg}
	ClosingQueue map[string]bool
	Config       PoolConfig
}

// Pipes contains data communication channels:
// ToPool and FromPool have type Data which wraps a *Socket instance, MessageType, and a []byte
// Shutdown carries shutdown command to goroutines reading/writing messages from websocket
// Error channel carries websocket error messages from goroutines back to pool controller
type Pipes struct {
	ToPool       chan Data
	FromPool     chan Data
	ToPoolJSON   chan JSONReaderWriter
	FromPoolJSON chan JSONReaderWriter
	Shutdown     chan string
	Error        chan ErrorMsg
}

// PoolConfig is used to pass configuration settings to the Pool initialization function
type PoolConfig struct {
	IsReadable bool
	IsWritable bool
	IsJSON     bool // If false messages will be read and written in bytes
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
		pipes.ToPoolJSON = make(chan JSONReaderWriter)
		pipes.FromPoolJSON = make(chan JSONReaderWriter)
	} else {
		pipes.ToPool = make(chan Data)
		pipes.FromPool = make(chan Data)
	}
	pipes.Shutdown = make(chan string)
	pipes.Error = make(chan ErrorMsg)

	pool := &SocketPool{
		OpenStack:    make(map[string]*Socket, len(urls)),
		ClosedStack:  make(map[string]*Socket, len(urls)),
		Pipes:        pipes,
		ClosingQueue: make(map[string]bool, len(urls)),
		Config:       config,
	}

	for _, v := range urls {
		count := 0
		if config.IsReadable == true {
			count++
		}
		if config.IsWritable == true {
			count++
		}
		s := &Socket{
			URL:        v,
			IsReadable: config.IsReadable,
			IsWritable: config.IsWritable,
			IsJSON:     config.IsJSON,
			RoutineCt:  count,
		}
		success := s.Connect(pipes, config)
		if success == true {
			pool.OpenStack[v] = s
		} else {
			pool.ClosedStack[v] = s
		}
	}

	return pool, nil
}

// Control method listens for Error Messages and dispatches shutdown messages
// It also routes Data messages to and from websockets
func (p *SocketPool) Control() {
	for {
		select {
		case e := <-p.Pipes.Error:
			s := p.checkOpenStack(e.URL)
			_, ok := p.ClosingQueue[s.URL]
			if s.RoutineCt == 1 {
				delete(p.OpenStack, s.URL)
				p.ClosedStack[s.URL] = s
				s.ClosedAt = time.Now()
			} else if s.RoutineCt == 2 && ok == true {
				delete(p.OpenStack, s.URL)
				delete(p.ClosingQueue, s.URL)
				p.ClosedStack[s.URL] = s
				s.ClosedAt = time.Now()
			} else if s.RoutineCt == 2 && ok == false {
				p.ClosingQueue[s.URL] = true
				p.Pipes.Shutdown <- s.URL
			}
		default:
			v := <-p.Pipes.ToPool
			p.Pipes.FromPool <- v
		}
	}
}

// ControlJSON controls flow of JSON data and manages errors and shutdown commands
func (p *SocketPool) ControlJSON() {
	for {
		select {
		case e := <-p.Pipes.Error:
			s := p.checkOpenStack(e.URL)
			_, ok := p.ClosingQueue[s.URL]
			if s.RoutineCt == 1 {
				delete(p.OpenStack, s.URL)
				p.ClosedStack[s.URL] = s
				s.ClosedAt = time.Now()
			} else if s.RoutineCt == 2 && ok == true {
				delete(p.OpenStack, s.URL)
				delete(p.ClosingQueue, s.URL)
				p.ClosedStack[s.URL] = s
				s.ClosedAt = time.Now()
			} else if s.RoutineCt == 2 && ok == false {
				p.ClosingQueue[s.URL] = true
				p.Pipes.Shutdown <- s.URL
				p.Pipes.Shutdown <- s.URL
			}
		default:
			v := <-p.Pipes.ToPoolJSON
			p.Pipes.FromPoolJSON <- v
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

// JSONReaderWriter is an interface with 2 methods. Both take a pointer to a websocket connection as a parameter:
// JSONReader reads from the websocket into a struct and sends to a channel
// JSONWriter gets a value from a channel and writes to a websocket
type JSONReaderWriter interface {
	JSONRead(s *Socket, toPoolJSON chan<- JSONReaderWriter, errorChan chan<- ErrorMsg) error
	JSONWrite(s *Socket, fromPoolJSON <-chan JSONReaderWriter, errorChan chan<- ErrorMsg) error
}

// Data wraps message type, []byte, and URL together so sender/receiver can identify the target/source respectively
type Data struct {
	URL     string
	Type    int
	Payload []byte
}

// ErrorMsg wraps an error message with Socket instance so receiver can try reconnect and/or log error
type ErrorMsg struct {
	URL   string
	Error error
}

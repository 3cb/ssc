package ssc

import (
	"errors"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// SocketPool is a collection of websocket connections combined with 4 channels which are used to send and received messages to and from the goroutines that control them
type SocketPool struct {
	Stack        map[string]*Socket
	Pipe         Pipe // {ToPool chan Data, FromPool chan Data}
	ErrorChan    chan ErrorMsg
	ShutdownChan chan string
	ClosingStack map[string]bool
}

// NewSocketPool creates a new instance of SocketPool and returns a pointer to it
func NewSocketPool(urls []string, readable bool, writable bool) (*SocketPool, error) {
	if readable == false && writable == false {
		err := errors.New("bad input values: Sockets cannot be both unreadable and unwritable")
		return nil, err
	}
	errorChan := make(chan ErrorMsg)
	shutdown := make(chan string)
	toPool := make(chan Data)
	fromPool := make(chan Data)
	pipe := Pipe{toPool, fromPool}

	pool := &SocketPool{
		Stack:        map[string]*Socket{},
		Pipe:         pipe,
		ErrorChan:    errorChan,
		ShutdownChan: shutdown,
		ClosingStack: make(map[string]bool, len(urls)),
	}

	for _, v := range urls {
		index := 0
		if readable == true {
			index++
		}
		if writable == true {
			index++
		}
		s := &Socket{
			URL:          v,
			isReadable:   readable,
			isWritable:   writable,
			ClosingIndex: index,
		}
		success := s.Connect(errorChan, shutdown, pipe)
		if success == true {
			pool.Stack[v] = s
		}
	}

	return pool, nil
}

// Control method listens for Error Messages and dispatches shutdown messages
// It also routes Data messages to and from websockets
func (p *SocketPool) Control() {
	for {
		select {
		case e := <-p.ErrorChan:
			switch {
			case e.Error != nil:
				log.Printf("Websocket error: %v\n%v: %v\n", e.Socket.URL, time.Now(), e.Error)
				if e.Socket.ClosingIndex == 1 {
					e.Socket.Connection.Close()
					e.Socket.isConnected = false
					e.Socket.ClosedAt = time.Now()
					delete(p.Stack, e.Socket.URL)
					log.Printf("Websocket at %v has been closed.", e.Socket.URL)
				} else {
					// at to closing stack send shutdown message
				}
				p.ShutdownChan <- e.Socket.URL
			case e.Error == nil && e.Socket.ClosingIndex == 1:
				e.Socket.Connection.Close()
				e.Socket.isConnected = false
				e.Socket.ClosedAt = time.Now()
				delete(p.Stack, e.Socket.URL)
				log.Printf("Websocket at %v has been closed.", e.Socket.URL)
			case e.Error == nil && e.Socket.ClosingIndex == 2:
				if p.ClosingStack[e.Socket.URL] == false {
					p.ClosingStack[e.Socket.URL] = true
				} else {
					e.Socket.Connection.Close()
					e.Socket.isConnected = false
					e.Socket.ClosedAt = time.Now()
					delete(p.ClosingStack, e.Socket.URL)
					delete(p.Stack, e.Socket.URL)
					log.Printf("Websocket at %v has been closed.", e.Socket.URL)
				}
			}
		}
	}
}

// Socket type defines a websocket connection
type Socket struct {
	URL          string
	Connection   *websocket.Conn
	isConnected  bool
	isReadable   bool
	isWritable   bool
	ClosingIndex int
	OpenedAt     time.Time
	ClosedAt     time.Time
}

func (s *Socket) Read(shutdown <-chan string, errorChan chan<- ErrorMsg, pipe Pipe) {
	defer func() {
		err := s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Printf("Closing (%v) at %v\n: %v", s.URL, time.Now(), err)
		}
		s.Connection.Close()
	}()
	for {
		select {
		case url := <-shutdown:
			if url == s.URL {
				log.Printf("Shutdown message received from controller(%v).\n", url)
				errorChan <- ErrorMsg{s, nil}
				return
			}
		default:
			readType, msg, err := s.Connection.ReadMessage()
			if err != nil {
				log.Printf("Error reading from websocket(%v): %v\n", s.URL, err)
				errorChan <- ErrorMsg{s, err}
				return
			}
			pipe.ToPool <- Data{s, readType, msg}
		}
	}
}

func (s *Socket) Write(shutdown <-chan string, errorChan chan<- ErrorMsg, pipe Pipe) {
	defer func() {
		err := s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Printf("Closing (%v) at %v\n: %v", s.URL, time.Now(), err)
		}
		s.Connection.Close()
	}()
	for {
		select {
		case url := <-shutdown:
			if url == s.URL {
				log.Printf("Shutdown message received from controller(%v).\n", url)
				errorChan <- ErrorMsg{s, nil}
				return
			}
		default:
			msg := <-pipe.FromPool
			if msg.Socket.URL == s.URL {
				err := s.Connection.WriteMessage(msg.Type, msg.Payload)
				if err != nil {
					log.Printf("Error writing to websocket(%v): %v\n", s.URL, err)
					errorChan <- ErrorMsg{s, err}
					return
				}
			}
		}
	}
}

// Pipe contains two data communication channels: inbound and outbound
type Pipe struct {
	ToPool   chan Data
	FromPool chan Data
}

// Data wraps message type, []byte, and Socket instance together so sender/receiver can identify the target/source respectively
type Data struct {
	Socket  *Socket
	Type    int
	Payload []byte
}

// ErrorMsg wraps an error message with Socket instance so receiver can try reconnect and/or log error
type ErrorMsg struct {
	Socket *Socket
	Error  error
}

// Connect connects to websocket given a url string and three channels from SocketPool type.
// Creates a goroutine to receive and send data as well as to listen for errors and calls to shutdown
func (s *Socket) Connect(errorChan chan<- ErrorMsg, shutdown <-chan string, pipe Pipe) bool {
	c, _, err := websocket.DefaultDialer.Dial(s.URL, nil)
	if err != nil {
		log.Printf("Error connecting to websocket: \n%v\n%v", s.URL, err)
		errorChan <- ErrorMsg{s, err}
		return false
	}
	s.Connection = c
	s.isConnected = true
	s.OpenedAt = time.Now()

	// Start goroutine to listen to websocket.
	// Closes connection and sends error message on error.
	// If shutdown message is received websocket error message is sent with nil error value and socket is Closed.
	go func() {
		defer func() {
			s.Connection.Close()
			s.ClosedAt = time.Now()
			s.isConnected = false
		}()

		for {
			select {
			case url := <-shutdown:
				if url == s.URL {
					log.Printf("Close Message Received from Controller. Closing websocket at: %v", s.URL)
					errorChan <- ErrorMsg{s, nil}
					return
				}
			case f := <-pipe.FromPool:
				err := f.Socket.Connection.WriteMessage(f.Type, f.Payload)
				if err != nil {
					log.Printf("Error writing to websocket: %v", f.Socket.URL)
					errorChan <- ErrorMsg{f.Socket, err}
					return
				}
			default:
				readType, msg, err := s.Connection.ReadMessage()
				if err != nil {
					log.Printf("Error reading from websocket(%v): ", err)
					errorChan <- ErrorMsg{s, err}
					return
				}
				pipe.ToPool <- Data{s, readType, msg}
			}
		}
	}()

	return true
}

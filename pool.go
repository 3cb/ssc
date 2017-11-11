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
	Pipes        *Pipes // {ToPool chan Data, FromPool chan Data, Shutdown chan string, Error chan ErrorMsg}
	ClosingStack map[string]bool
}

// Pipes contains data communication channels:
// ToPool and FromPool have type Data which wraps a *Socket instance, MessageType, and a []byte
// Shutdown carries shutdown command to goroutines reading/writing messages from websocket
// Error channel carries websocket error messages from goroutines back to pool controller
type Pipes struct {
	ToPool   chan Data
	FromPool chan Data
	Shutdown chan string
	Error    chan ErrorMsg
}

// NewSocketPool creates a new instance of SocketPool and returns a pointer to it and an error
func NewSocketPool(urls []string, readable bool, writable bool) (*SocketPool, error) {
	if readable == false && writable == false {
		err := errors.New("bad input values: Sockets cannot be both unreadable and unwritable")
		return nil, err
	}
	errorChan := make(chan ErrorMsg)
	shutdown := make(chan string)
	toPool := make(chan Data)
	fromPool := make(chan Data)
	pipes := &Pipes{toPool, fromPool, shutdown, errorChan}

	pool := &SocketPool{
		Stack:        make(map[string]*Socket, len(urls)),
		Pipes:        pipes,
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
		success := s.Connect(pipes)
		if success == true {
			pool.Stack[v] = s
		}
	}

	return pool, nil
}

// Control method listens for Error Messages and dispatches shutdown messages
// It also routes Data messages to and from websockets
func (p *SocketPool) Control() {
}

// Data wraps message type, []byte, and URL together so sender/receiver can identify the target/source respectively
type Data struct {
	URL     string
	Type    int
	Payload []byte
}

// JSONDataContainer is an interface with a GetURL() method that returns the type instances URL string and a GetPayload() method that returns and interface{} that takes shape of the callers data structure
type JSONDataContainer interface {
	GetURL() string
	GetPayload() interface{}
}

// ErrorMsg wraps an error message with Socket instance so receiver can try reconnect and/or log error
type ErrorMsg struct {
	URL   string
	Error error
}

// Connect connects to websocket given a url string and three channels from SocketPool type.
// Creates a goroutine to receive and send data as well as to listen for errors and calls to shutdown
func (s *Socket) Connect(pipes *Pipes) bool {
	c, _, err := websocket.DefaultDialer.Dial(s.URL, nil)
	if err != nil {
		log.Printf("Error connecting to websocket: \n%v\n%v", s.URL, err)
		errorChan <- ErrorMsg{s, err}
		return false
	}
	s.Connection = c
	s.isConnected = true
	s.OpenedAt = time.Now()

	return true
}

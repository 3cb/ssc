package ssc

import (
	"errors"
)

// SocketPool is a collection of websocket connections combined with 4 channels which are used to send and received messages to and from the goroutines that control them
type SocketPool struct {
	OpenStack    map[string]*Socket
	ClosedStack  map[string]*Socket
	Pipes        *Pipes // {ToPool chan Data, FromPool chan Data, Shutdown chan string, Error chan ErrorMsg}
	ClosingQueue map[string]bool
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

// PoolConfig is used to pass configuration settings to the Pool initialization function
type PoolConfig struct {
	isReadable bool
	isWritable bool
	isJSON     bool // If false messages will be read and written in bytes
	DataJSON   JSONDataContainer
	chJSON     chan JSONDataContainer
}

// NewSocketPool creates a new instance of SocketPool and returns a pointer to it and an error
func NewSocketPool(urls []string, config PoolConfig) (*SocketPool, error) {
	if config.isReadable == false && config.isWritable == false {
		err := errors.New("bad input values: Sockets cannot be both unreadable and unwritable")
		return nil, err
	}
	errorChan := make(chan ErrorMsg)
	shutdown := make(chan string)
	toPool := make(chan Data)
	fromPool := make(chan Data)
	pipes := &Pipes{toPool, fromPool, shutdown, errorChan}

	pool := &SocketPool{
		OpenStack:    make(map[string]*Socket, len(urls)),
		ClosedStack:  make(map[string]*Socket),
		Pipes:        pipes,
		ClosingQueue: make(map[string]bool, len(urls)),
	}

	for _, v := range urls {
		index := 0
		if config.isReadable == true {
			index++
		}
		if config.isWritable == true {
			index++
		}
		s := &Socket{
			URL:          v,
			isReadable:   config.isReadable,
			isWritable:   config.isWritable,
			isJSON:       config.isJSON,
			ClosingIndex: index,
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
}

// Data wraps message type, []byte, and URL together so sender/receiver can identify the target/source respectively
type Data struct {
	URL     string
	Type    int
	Payload []byte
}

// JSONDataContainer is an interface with 3 methods: GetURL(), SetURL(), and GetPayload()
// GetURL() returns the type instance's URL string
// SetURL() takes a *Socket instance URL string and sets it for the callers data type
// GetPayload() returns an interface{} that takes shape of the callers data structure
type JSONDataContainer interface {
	GetURL() string
	SetURL(string)
	GetPayload() interface{}
}

// ErrorMsg wraps an error message with Socket instance so receiver can try reconnect and/or log error
type ErrorMsg struct {
	URL   string
	Error error
}

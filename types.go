package ssc

import (
	"fmt"
)

// JSONReaderWriter is an interface with 2 methods. Both take a pointer to a websocket connection as a parameter:
// JSONReader reads from the websocket into a struct and sends to a channel
// JSONWriter gets a value from a channel and writes to a websocket
type JSONReaderWriter interface {
	JSONRead(s *Socket, Socket2PoolJSON chan<- JSONReaderWriter) error
	JSONWrite(s *Socket, Pool2SocketJSON <-chan JSONReaderWriter) error
}

// ErrorMsg wraps an error message with Socket instance so receiver can try reconnect and/or log error
type ErrorMsg struct {
	Socket *Socket
	Error  error
}

// Data wraps message type and []bytetogether so sender/receiver can identify the target/source respectively
// Implementing the JSONReaderWriter interface allows a Data instance to carry ping/pong messages
type Data struct {
	Type    int
	Payload []byte
}

// JSONRead reads pong message from websocket and sends data to SocketPool
func (d Data) JSONRead(s *Socket, Socket2PoolJSON chan<- JSONReaderWriter) error {
	err := s.Connection.ReadJSON(&d)
	if err != nil {
		return err
	}
	Socket2PoolJSON <- d
	return nil
}

// JSONWrite receives ping message from SocketPool and writes to websocket
func (d Data) JSONWrite(s *Socket, Pool2SocketJSON <-chan JSONReaderWriter) error {
	m, ok := (<-Pool2SocketJSON).(Data)
	if ok == false {
		return fmt.Errorf("wrong data type sent from Pool to websocket goroutine(%v)", s.URL)
	}
	err := s.Connection.WriteJSON(m)
	if err != nil {
		return err
	}
	return nil
}

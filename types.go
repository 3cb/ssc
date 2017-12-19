package ssc

// JSONReaderWriter is an interface with 2 methods. Both take a pointer to a websocket connection as a parameter:
// JSONReader reads from the websocket into a struct and sends to a channel
// JSONWriter gets a value from a channel and writes to a websocket
type JSONReaderWriter interface {
	JSONRead(s *Socket, Socket2PoolJSON chan<- JSONReaderWriter) error
	JSONWrite(s *Socket, Pool2SocketJSON <-chan JSONReaderWriter) error
}

// Data wraps message type and []bytetogether so sender/receiver can identify the target/source respectively
type Data struct {
	Type    int
	Payload []byte
}

// ErrorMsg wraps an error message with Socket instance so receiver can try reconnect and/or log error
type ErrorMsg struct {
	Socket *Socket
	Error  error
}

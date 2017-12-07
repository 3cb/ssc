package ssc

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

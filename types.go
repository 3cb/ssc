package ssc

// ErrorMsg wraps an error message with Socket instance so receiver can try reconnect and/or log error
type ErrorMsg struct {
	Socket *Socket
	Error  error
}

// Message wraps message type and []bytetogether so sender/receiver can identify the target/source respectively
type Message struct {
	Type    int
	Payload []byte
	URL     string
}

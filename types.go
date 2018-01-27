package ssc

// ErrorMsg wraps an error message with Socket instance so receiver can try reconnect and/or log error
type ErrorMsg struct {
	Socket *Socket
	Error  error
}

// JSONReaderWriter contains one method which checks a channel and makes a type assert
type JSONReaderWriter interface {
	ReadJSON(s *Socket, b []byte, Socket2PoolJSON chan<- JSONReaderWriter) error
	WriteJSON(s *Socket) error
}

// Message wraps message type and []bytetogether so sender/receiver can identify the target/source respectively
type Message struct {
	Type    int    `json:"type"`
	Payload []byte `json:"payload"`
}

// ReadJSON is an empty method used to satisfy the JSONReaderWriter interface
func (m Message) ReadJSON(s *Socket, b []byte, Socket2PoolJSON chan<- JSONReaderWriter) error {
	return nil
}

// WriteJSON is used to write ping messages to a JSON websocket
func (m Message) WriteJSON(s *Socket) error {
	err := s.Connection.WriteMessage(m.Type, m.Payload)
	if err != nil {
		return err
	}
	return nil
}

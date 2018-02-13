// Package ssc creates and controls pools of websocket connections --  client and server
package ssc

// ErrorMsg wraps an error message with Socket instance so receiver can try reconnect and/or log error
type ErrorMsg struct {
	Socket *Socket
	Error  error
}

// Message contains type and payload as a normal websocket message
// URL is added in case user needs to identify source (websocket server)
type Message struct {
	ID      string
	Type    int
	Payload []byte
}

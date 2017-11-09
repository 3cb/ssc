package ssc

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// SocketPool is a collection of websocket connections combined with 3 channels used to send and received message to and from the goroutines that control them
type SocketPool struct {
	Sockets      []*Socket
	FailureChan  chan FailureMSG
	shutdownChan chan *Socket
	DataChan     chan SocketData
}

// Socket type defines a websocket connection
type Socket struct {
	URL         string
	Connection  *websocket.Conn
	isConnected bool
	OpenedAt    time.Time
	ClosedAt    time.Time
}

// SocketData wraps []byte and Socket instance together so receiver can identify the source
type SocketData struct {
	Socket  *Socket
	Payload []byte
}

// FailureMSG wraps an error message with Socket instance so receiver can try reconnect and/or log error
type FailureMSG struct {
	Socket *Socket
	Error  error
}

// NewSocketPool creates a new instance of SocketPool and returns a pointer to it
func NewSocketPool(urls []string) *SocketPool {
	failure := make(chan FailureMSG)
	shutdown := make(chan *Socket)
	data := make(chan SocketData)

	sockets := []*Socket{}
	for _, v := range urls {
		p := &Socket{URL: v}
		success := p.Connect(failure, shutdown, data)
		if success == true {
			sockets = append(sockets, p)
		}
	}

	pool := &SocketPool{
		sockets,
		failure,
		shutdown,
		data,
	}
	return pool
}

// Connect connects to websocket given a url string and three channels from SocketPool type.
// Creates a goroutine to receive and send data as well as to listen for failures and calls to shutdown
func (p *Socket) Connect(failure chan<- FailureMSG, shutdown <-chan *Socket, data chan<- SocketData) bool {
	c, _, err := websocket.DefaultDialer.Dial(p.URL, nil)
	if err != nil {
		log.Printf("Error connecting to websocket: \n%v\n%v", p.URL, err)
		failure <- FailureMSG{p, err}
		return false
	}
	p.Connection = c
	p.isConnected = true
	p.OpenedAt = time.Now()

	// Start goroutine to listen to websocket.
	// Closes connection and sends failure message on error.
	// If shutdown message is received websocket failure message is sent with nil error value and socket is Closed.
	go func() {
		defer func() {
			p.Connection.Close()
			p.ClosedAt = time.Now()
			p.isConnected = false
		}()

		for {
			select {
			case v := <-shutdown:
				if v.Connection == p.Connection {
					log.Printf("Close Message Received from Controller. Closing websocket at: %v", p.URL)
					failure <- FailureMSG{p, nil}
					return
				}
			default:
				_, msg, err := p.Connection.ReadMessage()
				if err != nil {
					log.Printf("Error reading from websocket(%v): ", err)

					failure <- FailureMSG{p, err}
					return
				}
				data <- SocketData{p, msg}
			}
		}
	}()

	return true
}

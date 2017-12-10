package ssc

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// Socket type defines a websocket connection along with configuration and channels used to run read and write goroutines
type Socket struct {
	Connection       *websocket.Conn
	URL              string
	IsConnected      bool
	IsReadable       bool
	IsWritable       bool
	IsJSON           bool
	Pool2SocketBytes chan Data
	Pool2SocketJSON  chan JSONReaderWriter
	ShutdownRead     chan struct{}
	ShutdownWrite    chan struct{}
	OpenedAt         time.Time
	ClosedAt         time.Time
}

func newSocketInstance(url string, config PoolConfig) *Socket {
	var chBytes chan Data
	var chJSON chan JSONReaderWriter

	if config.IsJSON == true {
		chJSON = make(chan JSONReaderWriter)
	} else {
		chBytes = make(chan Data)
	}

	s := &Socket{
		URL:              url,
		IsReadable:       config.IsReadable,
		IsWritable:       config.IsWritable,
		IsJSON:           config.IsJSON,
		Pool2SocketBytes: chBytes,
		Pool2SocketJSON:  chJSON,
		ShutdownRead:     make(chan struct{}),
		ShutdownWrite:    make(chan struct{}),
	}
	return s
}

// Connect connects to websocket given a url string and config struct from SocketPool.
// Creates a goroutine to receive and send data as well as to listen for errors and calls to shutdown
func (s *Socket) connect(pipes *Pipes, config PoolConfig) (bool, error) {
	c, resp, err := websocket.DefaultDialer.Dial(s.URL, nil)
	if resp.StatusCode != 101 || err != nil {
		return false, err
	}
	s.Connection = c
	s.IsConnected = true
	s.OpenedAt = time.Now()

	switch s.IsJSON {
	case true:
		if s.IsReadable == true {
			go s.readSocketJSON(pipes, config.DataJSON)
		}
		if s.IsWritable == true {
			go s.writeSocketJSON(pipes, config.DataJSON)
		}
	case false:
		if s.IsReadable {
			go s.readSocketBytes(pipes)
		}
		if s.IsWritable {
			go s.writeSocketBytes(pipes)
		}
	}
	return true, nil
}

// ReadSocketBytes runs a continuous loop that reads messages from websocket and sends the []byte to the Pool controller
// It also listens for shutdown command from Pool and will close connection on command and also close connection on any errors reading from websocket
func (s *Socket) readSocketBytes(pipes *Pipes) {
	defer func() {
		err := s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		log.Printf("Closing (%v) at %v\nError: %v", s.URL, time.Now(), err)
		s.Connection.Close()
	}()
	for {
		select {
		case <-s.ShutdownRead:
			log.Printf("Shutdown message received from controller(%v).\n", s.URL)
			pipes.ErrorRead <- ErrorMsg{s.URL, nil}
			return
		default:
			readType, msg, err := s.Connection.ReadMessage()
			if err != nil {
				log.Printf("Error reading from websocket(%v): %v\n", s.URL, err)
				pipes.ErrorRead <- ErrorMsg{s.URL, err}
				return
			}
			pipes.Socket2PoolBytes <- Data{s.URL, readType, msg}
		}
	}
}

// ReadSocketJSON runs a continuous loop that reads messages from websocket and sends the JSON encoded message to the Pool controller
// It also listens for shutdown command from Pool and will close connection on command and also close connection on any errors reading from websocket
// Parameter v represents the data structure caller wants ReadJSON() methods to parse message data into
// pipes conatains a channel with v's type, passed in by caller
func (s *Socket) readSocketJSON(pipes *Pipes, data JSONReaderWriter) {
	defer func() {
		err := s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		log.Printf("Closing (%v) at %v\nError: %v", s.URL, time.Now(), err)
		s.Connection.Close()
	}()

	for {
		select {
		case <-s.ShutdownRead:
			log.Printf("Shutdown message received from controller(%v).\n", s.URL)
			pipes.ErrorRead <- ErrorMsg{s.URL, nil}
			return
		default:
			err := data.JSONRead(s, pipes.Socket2PoolJSON)
			if err != nil {
				log.Printf("Error reading message from websocket(%v): %v\n", s.URL, err)
				pipes.ErrorRead <- ErrorMsg{s.URL, err}
				return
			}
		}
	}
}

// WriteSocketBytes runs a continuous loop that reads []byte messages from the FromPool channel and writes them to the websocket
// It also listens for shutdown command from Pool and will close connection on command and also close connection on any errors writing from websocket
func (s *Socket) writeSocketBytes(pipes *Pipes) {
	defer func() {
		err := s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		log.Printf("Closing (%v) at %v\nError: %v", s.URL, time.Now(), err)
		s.Connection.Close()
	}()
	for {
		select {
		case <-s.ShutdownWrite:
			log.Printf("Shutdown message received from controller(%v).\n", s.URL)
			pipes.ErrorWrite <- ErrorMsg{s.URL, nil}
			return
		default:
			msg := <-s.Pool2SocketBytes
			err := s.Connection.WriteMessage(msg.Type, msg.Payload)
			if err != nil {
				log.Printf("Error writing to websocket(%v): %v\n", s.URL, err)
				pipes.ErrorWrite <- ErrorMsg{s.URL, err}
				return
			}
		}
	}
}

// WriteSocketJSON runs a continuous loop that reads values sent from the SocketPool controller and writes them to the websocket.
// It listens for shutdown command from the controller and will close websocket connection on any such command as well as on any error writing to the websocket
func (s *Socket) writeSocketJSON(pipes *Pipes, data JSONReaderWriter) {
	defer func() {
		err := s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		log.Printf("Closing (%v) at %v\nError: %v", s.URL, time.Now(), err)
		s.Connection.Close()
	}()
	for {
		select {
		case <-s.ShutdownWrite:
			log.Printf("Shutdown message received from controller(%v).\n", s.URL)
			pipes.ErrorWrite <- ErrorMsg{s.URL, nil}
			return
		default:
			err := data.JSONWrite(s, s.Pool2SocketJSON)
			if err != nil {
				log.Printf("Error writing to websocket(%v): %v\n", s.URL, err)
				pipes.ErrorWrite <- ErrorMsg{s.URL, err}
				return
			}
		}
	}
}

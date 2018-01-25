package ssc

import (
	"net/http"
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

// NewSocketInstance returns a new instance of a Socket
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

// ConnectClient connects to a websocket using websocket.Upgrade() method and starts goroutine/s for read and/or write
func (s *Socket) connectClient(pool *SocketPool, upgrader *websocket.Upgrader, w http.ResponseWriter, r *http.Request) (bool, error) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return false, err
	}
	s.Connection = c
	s.IsConnected = true
	s.OpenedAt = time.Now()

	switch s.IsJSON {
	case true:
		if s.IsReadable == true {
			go s.readSocketJSON(pool.Pipes, pool.Config.DataJSON)
			pool.ReadStack[s] = true
		} else {
			pool.ReadStack[s] = false
		}
		if s.IsWritable == true {
			go s.writeSocketJSON(pool.Pipes, pool.Config.DataJSON)
			pool.WriteStack[s] = true
		} else {
			pool.WriteStack[s] = false
		}
	case false:
		if s.IsReadable {
			go s.readSocketBytes(pool.Pipes)
			pool.ReadStack[s] = true
		} else {
			pool.ReadStack[s] = false
		}
		if s.IsWritable {
			go s.writeSocketBytes(pool.Pipes)
			pool.WriteStack[s] = true
		} else {
			pool.WriteStack[s] = false
		}
	}
	return true, nil
}

// ConnectServer connects to websocket given a url string and config struct from SocketPool.
// Creates a goroutine to receive and send data as well as to listen for errors and calls to shutdown
func (s *Socket) connectServer(pool *SocketPool) (bool, error) {
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
			go s.readSocketJSON(pool.Pipes, pool.Config.DataJSON)
			pool.ReadStack[s] = true
		} else {
			pool.ReadStack[s] = false
		}
		if s.IsWritable == true {
			go s.writeSocketJSON(pool.Pipes, pool.Config.DataJSON)
			pool.WriteStack[s] = true
		} else {
			pool.WriteStack[s] = false
		}
	case false:
		if s.IsReadable {
			go s.readSocketBytes(pool.Pipes)
			pool.ReadStack[s] = true
		} else {
			pool.ReadStack[s] = false
		}
		if s.IsWritable {
			go s.writeSocketBytes(pool.Pipes)
			pool.WriteStack[s] = true
		} else {
			pool.WriteStack[s] = false
		}
	}
	return true, nil
}

// ReadSocketBytes runs a continuous loop that reads messages from websocket and sends the []byte to the Pool controller
// It also listens for shutdown command from Pool and will close connection on command and also close connection on any errors reading from websocket
func (s *Socket) readSocketBytes(pipes *Pipes) {
	defer func() {
		_ = s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		s.Connection.Close()
	}()
	for {
		select {
		case <-s.ShutdownRead:
			pipes.ErrorRead <- ErrorMsg{s, nil}
			return
		default:
			readType, msg, err := s.Connection.ReadMessage()
			if err != nil {
				pipes.ErrorRead <- ErrorMsg{s, err}
				return
			}
			pipes.Socket2PoolBytes <- Data{Type: readType, Payload: msg}
		}
	}
}

// ReadSocketJSON runs a continuous loop that reads messages from websocket and sends the JSON encoded message to the Pool controller
// It also listens for shutdown command from Pool and will close connection on command and also close connection on any errors reading from websocket
// Parameter v represents the data structure caller wants ReadJSON() methods to parse message data into
// pipes conatains a channel with v's type, passed in by caller
func (s *Socket) readSocketJSON(pipes *Pipes, data JSONReaderWriter) {
	defer func() {
		_ = s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		s.Connection.Close()
	}()

	for {
		select {
		case <-s.ShutdownRead:
			pipes.ErrorRead <- ErrorMsg{s, nil}
			return
		default:
			err := data.JSONRead(s, pipes.Socket2PoolJSON)
			if err != nil {
				pipes.ErrorRead <- ErrorMsg{s, err}
				return
			}
		}
	}
}

// WriteSocketBytes runs a continuous loop that reads []byte messages from the FromPool channel and writes them to the websocket
// It also listens for shutdown command from Pool and will close connection on command and also close connection on any errors writing from websocket
func (s *Socket) writeSocketBytes(pipes *Pipes) {
	defer func() {
		_ = s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		s.Connection.Close()
	}()
	for {
		select {
		case <-s.ShutdownWrite:
			pipes.ErrorWrite <- ErrorMsg{s, nil}
			return
		case msg := <-s.Pool2SocketBytes:
			err := s.Connection.WriteMessage(msg.Type, msg.Payload)
			if err != nil {
				pipes.ErrorWrite <- ErrorMsg{s, err}
				return
			}
		}
	}
}

// WriteSocketJSON runs a continuous loop that reads values sent from the SocketPool controller and writes them to the websocket.
// It listens for shutdown command from the controller and will close websocket connection on any such command as well as on any error writing to the websocket
func (s *Socket) writeSocketJSON(pipes *Pipes, data JSONReaderWriter) {
	defer func() {
		_ = s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		s.Connection.Close()
	}()
	for {
		select {
		case <-s.ShutdownWrite:
			pipes.ErrorWrite <- ErrorMsg{s, nil}
			return
		default:
			err := data.JSONWrite(s, s.Pool2SocketJSON)
			if err != nil {
				pipes.ErrorWrite <- ErrorMsg{s, err}
				return
			}
		}
	}
}

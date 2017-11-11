package ssc

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// Socket type defines a websocket connection
type Socket struct {
	URL          string
	Connection   *websocket.Conn
	isConnected  bool
	isReadable   bool
	isWritable   bool
	isJSON       bool
	ClosingIndex int
	OpenedAt     time.Time
	ClosedAt     time.Time
}

// Connect connects to websocket given a url string and three channels from SocketPool type.
// Creates a goroutine to receive and send data as well as to listen for errors and calls to shutdown
func (s *Socket) Connect(pipes *Pipes, config PoolConfig) bool {
	c, resp, err := websocket.DefaultDialer.Dial(s.URL, nil)
	if resp.StatusCode != 101 || err != nil {
		log.Printf("Error connecting to websocket(%v): %v\n", s.URL, err)
		pipes.Error <- ErrorMsg{s.URL, err}
		return false
	}
	s.Connection = c
	s.isConnected = true
	s.OpenedAt = time.Now()

	switch s.isJSON {
	case true:
		if s.isReadable == true {
			go s.ReadSocketJSON(pipes, config.DataJSON, config.chJSON)
		}
		if s.isWritable == true {
			go s.WriteSocketJSON(pipes, config.DataJSON, config.chJSON)
		}
	case false:
		if s.isReadable {
			go s.ReadSocketBytes(pipes)
		}
		if s.isWritable {
			go s.WriteSocketBytes(pipes)
		}
	}

	return true
}

// ReadSocketBytes runs a continuous loop that reads messages from websocket and sends the []byte to the Pool controller
// It also listens for shutdown command from Pool and will close connection on command and also close connection on any errors reading from websocket
func (s *Socket) ReadSocketBytes(pipes *Pipes) {
	defer func() {
		err := s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Printf("Closing (%v) at %v\n: %v", s.URL, time.Now(), err)
		}
		s.Connection.Close()
	}()
	for {
		select {
		case url := <-pipes.Shutdown:
			if url == s.URL {
				log.Printf("Shutdown message received from controller(%v).\n", url)
				pipes.Error <- ErrorMsg{s.URL, nil}
				return
			}
		default:
			readType, msg, err := s.Connection.ReadMessage()
			if err != nil {
				log.Printf("Error reading from websocket(%v): %v\n", s.URL, err)
				pipes.Error <- ErrorMsg{s.URL, err}
				return
			}
			pipes.ToPool <- Data{s.URL, readType, msg}
		}
	}
}

// ReadSocketJSON runs a continuous loop that reads messages from websocket and sends the JSON encoded message to the Pool controller
// It also listens for shutdown command from Pool and will close connection on command and also close connection on any errors reading from websocket
// Parameter v represents the data structure caller wants ReadJSON() methods to parse message data into
// Parameter (ch <-chan JSONDataContainer) is a channel with v's type passed in by caller
func (s *Socket) ReadSocketJSON(pipes *Pipes, msg JSONDataContainer, ch chan<- JSONDataContainer) {
	defer func() {
		err := s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Printf("Closing (%v) at %v\n: %v", s.URL, time.Now(), err)
		}
		s.Connection.Close()
	}()
	for {
		select {
		case url := <-pipes.Shutdown:
			if url == s.URL {
				log.Printf("Shutdown message received from controller(%v).\n", url)
				pipes.Error <- ErrorMsg{s.URL, nil}
				return
			}
		default:
			err := s.Connection.ReadJSON(&msg)
			if err != nil {
				log.Printf("Error reading message from websocket(%v): %v\n", s.URL, err)
				pipes.Error <- ErrorMsg{s.URL, err}
				return
			}
			msg.SetURL(s.URL)
			ch <- msg
		}
	}
}

// WriteSocketBytes runs a continuous loop that reads []byte messages from the FromPool channel and writes them to the websocket
// It also listens for shutdown command from Pool and will close connection on command and also close connection on any errors writing from websocket
func (s *Socket) WriteSocketBytes(pipes *Pipes) {
	defer func() {
		err := s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Printf("Closing (%v) at %v\n: %v", s.URL, time.Now(), err)
		}
		s.Connection.Close()
	}()
	for {
		select {
		case url := <-pipes.Shutdown:
			if url == s.URL {
				log.Printf("Shutdown message received from controller(%v).\n", url)
				pipes.Error <- ErrorMsg{s.URL, nil}
				return
			}
		default:
			msg := <-pipes.FromPool
			if msg.URL == s.URL {
				err := s.Connection.WriteMessage(msg.Type, msg.Payload)
				if err != nil {
					log.Printf("Error writing to websocket(%v): %v\n", s.URL, err)
					pipes.Error <- ErrorMsg{s.URL, err}
					return
				}
			}
		}
	}
}

// WriteSocketJSON runs a continuous loop that reads msg values from the ch channel and writes them to the websocket.
// It listen for shutdown command from the controller and will close websocket connection on any such command as well as any error writing to the websocket
func (s *Socket) WriteSocketJSON(pipes *Pipes, msg JSONDataContainer, ch <-chan JSONDataContainer) {
	defer func() {
		err := s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Printf("Closing (%v) at %v\n: %v", s.URL, time.Now(), err)
		}
		s.Connection.Close()
	}()
	for {
		select {
		case url := <-pipes.Shutdown:
			if url == s.URL {
				log.Printf("Shutdown message received from controller(%v).\n", url)
				pipes.Error <- ErrorMsg{s.URL, nil}
				return
			}
		default:
			msg := <-ch
			if msg.GetURL() == s.URL {
				err := s.Connection.WriteJSON(msg.GetPayload())
				if err != nil {
					log.Printf("Error writing to websocket(%v): %v\n", s.URL, err)
					pipes.Error <- ErrorMsg{s.URL, err}
					return
				}
			}
		}
	}
}

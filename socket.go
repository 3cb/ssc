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
	ClosingIndex int
	OpenedAt     time.Time
	ClosedAt     time.Time
}

func (s *Socket) Read(shutdown <-chan string, errorChan chan<- ErrorMsg, pipe Pipe) {
	defer func() {
		err := s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Printf("Closing (%v) at %v\n: %v", s.URL, time.Now(), err)
		}
		s.Connection.Close()
	}()
	for {
		select {
		case url := <-shutdown:
			if url == s.URL {
				log.Printf("Shutdown message received from controller(%v).\n", url)
				errorChan <- ErrorMsg{s, nil}
				return
			}
		default:
			readType, msg, err := s.Connection.ReadMessage()
			if err != nil {
				log.Printf("Error reading from websocket(%v): %v\n", s.URL, err)
				errorChan <- ErrorMsg{s, err}
				return
			}
			pipe.ToPool <- Data{s, readType, msg}
		}
	}
}

func (s *Socket) Write(shutdown <-chan string, errorChan chan<- ErrorMsg, pipe Pipe) {
	defer func() {
		err := s.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Printf("Closing (%v) at %v\n: %v", s.URL, time.Now(), err)
		}
		s.Connection.Close()
	}()
	for {
		select {
		case url := <-shutdown:
			if url == s.URL {
				log.Printf("Shutdown message received from controller(%v).\n", url)
				errorChan <- ErrorMsg{s, nil}
				return
			}
		default:
			msg := <-pipe.FromPool
			if msg.Socket.URL == s.URL {
				err := s.Connection.WriteMessage(msg.Type, msg.Payload)
				if err != nil {
					log.Printf("Error writing to websocket(%v): %v\n", s.URL, err)
					errorChan <- ErrorMsg{s, err}
					return
				}
			}
		}
	}
}

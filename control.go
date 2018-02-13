// Package ssc creates and controls pools of websocket connections --  client and server
package ssc

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Control method launches ControlShutdown(), ControlRead(), ControlWrite(), and ControlPing()
func (p *SocketPool) Control() {
	go p.controlShutdown()
	go p.controlRead()
	go p.controlWrite()
	if p.PingInterval > 0 {
		go p.controlPing()
	}
}

// controlRead runs an infinite loop to take messages from websocket servers and send them to the outbound channel
func (p *SocketPool) controlRead() {
	wg := &sync.WaitGroup{}
	defer func() {
		log.Printf("ControlRead goroutine was stopped at %v.", time.Now())
		wg.Done()
	}()
	log.Printf("ControlRead started.")

	for {
		select {
		case wg = <-p.Pipes.StopReadControl:
			return
		case msg := <-p.Pipes.Socket2Pool:
			p.Pipes.Outbound <- msg
		}
	}
}

// controlWrite runs an infinite loop to take messages from inbound channel and send to write goroutines
func (p *SocketPool) controlWrite() {
	wg := &sync.WaitGroup{}
	defer func() {
		log.Printf("ControlWrite goroutine was stopped at %v.", time.Now())
		wg.Done()
	}()
	log.Printf("ControlWrite started.")

	for {
		select {
		case wg = <-p.Pipes.StopWriteControl:
			return
		case msg := <-p.Pipes.Inbound:
			p.Writers.mtx.RLock()
			for socket, open := range p.Writers.Stack {
				if open {
					socket.Pool2Socket <- msg
				}
			}
			p.Writers.mtx.RUnlock()
		}
	}
}

// controlShutdown method listens for Error Messages and dispatches shutdown messages
func (p *SocketPool) controlShutdown() {
	wg := &sync.WaitGroup{}
	defer func() {
		log.Printf("ControlShutdown goroutine was stopped at %v.", time.Now())
		wg.Done()
	}()
	log.Printf("ControlShutdown started.")

	for {
		select {
		case wg = <-p.Pipes.StopShutdownControl:
			return
		case e := <-p.Pipes.Error:
			s := e.Socket

			p.Readers.mtx.Lock()
			delete(p.Readers.Stack, s)
			p.Readers.mtx.Unlock()

			p.Writers.mtx.Lock()
			delete(p.Writers.Stack, s)
			p.Writers.mtx.Unlock()

			p.Pingers.mtx.Lock()
			delete(p.Pingers.Stack, s)
			p.Pingers.mtx.Unlock()

			if e.Error != nil {
				fmt.Printf("\nSocket ID: %v\nClosed due to error: %s\n", s.ID, e.Error)
			} else {
				fmt.Printf("\nSocket ID: %v\nClosed due to quit quit signal.\n", s.ID)
			}
		}
	}
}

// controlPing runs an infinite loop to send ping messages to websocket write goroutines at an interval defined in Config
func (p *SocketPool) controlPing() {
	wg := &sync.WaitGroup{}
	defer func() {
		log.Printf("ControlPing goroutine was stopped at %v.", time.Now())
		wg.Done()
	}()
	log.Printf("ControlPing started.")

	var t time.Duration
	if p.PingInterval < (time.Second * 30) {
		t = time.Second * 30
	} else {
		t = p.PingInterval
	}
	ticker := time.NewTicker(t)

	for {
		select {
		case wg = <-p.Pipes.StopPingControl:
			return
		case <-ticker.C:
			if !p.isPingStackEmpty() {
				p.Pingers.mtx.Lock()
				fmt.Printf("%v active websocket connections", len(p.Pingers.Stack))
				for s, missed := range p.Pingers.Stack {
					if missed < 2 {
						p.Pingers.Stack[s]++
						s.Pool2Socket <- Message{Type: websocket.PingMessage}
					} else {
						s.Quit <- struct{}{}
					}
				}
				p.Pingers.mtx.Unlock()
			}
		}
	}
}

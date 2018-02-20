// Package ssc creates and controls pools of websocket connections --  client and server
package ssc

import (
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// control method launches ControlShutdown(), ControlRead(), ControlWrite(), and ControlPing()
func (p *Pool) control() {
	go p.controlShutdown()
	go p.controlRead()
	go p.controlWrite()
}

// controlRead runs an infinite loop to take messages from websocket servers and send them to the outbound channel
func (p *Pool) controlRead() {
	log.Printf("ControlRead started at %v\n", time.Now())

	for {
		select {
		case wg := <-p.stopReadControl:
			log.Printf("ControlRead goroutine was stopped at %v\n", time.Now())
			wg.Done()
			return
		case msg := <-p.s2p:
			p.Outbound <- msg
		}
	}
}

// controlWrite runs an infinite loop to take messages from inbound channel and send to ALL write goroutines
func (p *Pool) controlWrite() {
	log.Printf("ControlWrite started at %v\n", time.Now())
	ticker := time.NewTicker(p.pingInterval)

	for {
		select {
		case wg := <-p.stopWriteControl:
			log.Printf("ControlWrite goroutine was stopped at %v\n", time.Now())
			wg.Done()
			return
		case <-ticker.C:
			p.rw.mtx.RLock()
			for socket := range p.rw.stack {
				socket.p2s <- &Message{ID: socket.id, Type: websocket.PingMessage}
			}
			p.rw.mtx.RUnlock()
		case msg := <-p.Inbound:
			p.rw.mtx.RLock()
			for socket := range p.rw.stack {
				socket.p2s <- msg
			}
			p.rw.mtx.RUnlock()
		}
	}
}

// controlShutdown method listens for Error Messages and closes socket connections and removes them from stack
// is Stop() method has been called, will signal that all read/write goroutines have been stoppped so that control goroutines and then be shut down
func (p *Pool) controlShutdown() {
	log.Printf("ControlShutdown started at %v\n", time.Now())

	for {
		select {
		case wg := <-p.stopShutdownControl:
			log.Printf("ControlShutdown goroutine was stopped at %v\n", time.Now())
			wg.Done()
			return
		case s := <-p.remove:
			s.rQuit <- struct{}{}
			s.wQuit <- struct{}{}
		case s := <-p.shutdown:
			closed := false
			p.rw.mtx.Lock()
			if p.rw.stack[s] > 0 {
				s.close()
				delete(p.rw.stack, s)
				closed = true
			}
			p.rw.mtx.Unlock()

			if closed {
				if len(s.errors) > 0 {
					fmt.Printf("\nSocket ID: %v\nAddr: %v\nClosed due to error: %s\nTime: %v\n", s.id, s.connection.RemoteAddr(), s.errors, time.Now())
				} else {
					fmt.Printf("\nSocket ID: %v\nAddr: %v\nClosed due to quit signal\nTime: %v\n", s.id, s.connection.RemoteAddr(), time.Now())
				}
			}

			if p.isDraining {
				p.rw.mtx.RLock()
				if len(p.rw.stack) == 0 {
					p.allClosed <- struct{}{}
				}
				p.rw.mtx.RUnlock()
			}
		}
	}
}

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
	if p.pingInterval > 0 {
		go p.controlPing()
	}
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

	for {
		select {
		case wg := <-p.stopWriteControl:
			log.Printf("ControlWrite goroutine was stopped at %v\n", time.Now())
			wg.Done()
			return
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
		case s := <-p.shutdown:
			closed := false
			p.rw.mtx.Lock()
			if p.rw.stack[s] > 0 {
				closed = s.close()
				delete(p.rw.stack, s)
			} else {
				p.rw.stack[s]++
				delete(p.ping.stack, s)
			}
			p.rw.mtx.Unlock()

			if closed {
				if len(s.errors) > 0 {
					fmt.Printf("\nSocket ID: %v\nAddr: %vClosed due to error: %s\nTime: %v\n", s.id, s.connection.RemoteAddr(), s.errors, time.Now())
				} else {
					fmt.Printf("\nSocket ID: %v\nAddr: %vClosed due to quit signal\nTime: %v\n", s.id, s.connection.RemoteAddr(), time.Now())
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

// controlPing runs an infinite loop to send ping messages to websocket write goroutines at an interval defined with NewPool()
func (p *Pool) controlPing() {
	log.Printf("ControlPing started at %v\n", time.Now())

	var t time.Duration
	if p.pingInterval < (time.Second * 30) {
		t = time.Second * 30
	} else {
		t = p.pingInterval
	}
	ticker := time.NewTicker(t)

	for {
		select {
		case wg := <-p.stopPingControl:
			log.Printf("ControlPing goroutine was stopped at %v\n", time.Now())
			wg.Done()
			return
		case <-ticker.C:
			p.ping.mtx.Lock()
			fmt.Printf("%v active websocket connections", len(p.ping.stack))
			for s, missed := range p.ping.stack {
				if missed < 2 {
					p.ping.stack[s]++
					s.p2s <- &Message{Type: websocket.PingMessage}
				} else {
					s.rQuit <- struct{}{}
					s.wQuit <- struct{}{}
				}
			}
			p.ping.mtx.Unlock()
		}
	}
}

package ssc

import (
	"log"
	"time"
)

// Control method launches ControlShutdown(), ControlRead(), and ControlWrite()
func (p *SocketPool) Control() {
	go p.ControlShutdown()
	if p.Config.IsReadable {
		go p.ControlRead()
	}
	if p.Config.IsWritable {
		go p.ControlWrite()
	}
	if p.Config.IsReadable && p.Config.IsWritable {
		go p.ControlPing(p.Config.PingInterval)
		go p.ControlPong()
	}
}

// ControlRead runs an infinite loop to take messages from websocket servers and send them to the outbound channel
func (p *SocketPool) ControlRead() {
	defer func() {
		log.Printf("ControlRead goroutine was stopped at %v.\n\n", time.Now())
	}()
	log.Printf("ControlRead started.")
	if p.Config.IsJSON {
		for {
			select {
			case <-p.Pipes.StopReadControl:
				return
			case v := <-p.Pipes.Socket2PoolJSON:
				p.Pipes.OutboundJSON <- v
			}
		}
	} else {
		for {
			select {
			case <-p.Pipes.StopReadControl:
				return
			case v := <-p.Pipes.Socket2PoolBytes:
				p.Pipes.OutboundBytes <- v
			}
		}
	}
}

// ControlWrite runs an infinite loop to take messages from inbound channel and send to write goroutines
func (p *SocketPool) ControlWrite() {
	defer func() {
		log.Printf("ControlWrite goroutine was stopped at %v.\n\n", time.Now())
	}()
	log.Printf("ControlWrite started.")
	if p.Config.IsJSON {
		for {
			select {
			case <-p.Pipes.StopWriteControl:
				return
			case v := <-p.Pipes.InboundJSON:
				for socket, open := range p.WriteStack {
					if open {
						socket.Pool2SocketJSON <- v
					}
				}
			}
		}
	} else {
		for {
			select {
			case <-p.Pipes.StopWriteControl:
				return
			case v := <-p.Pipes.InboundBytes:
				for socket, open := range p.WriteStack {
					if open {
						socket.Pool2SocketBytes <- v
					}
				}
			}
		}
	}
}

// ControlShutdown method listens for Error Messages and dispatches shutdown messages
func (p *SocketPool) ControlShutdown() {
	defer func() {
		log.Printf("ControlShutdown goroutine was stopped at %v.\n\n", time.Now())
	}()
	log.Printf("ControlShutdown started.")
	for {
		select {
		case <-p.Pipes.StopShutdownControl:
			return
		case e := <-p.Pipes.ErrorRead:
			s := e.Socket
			switch {
			case p.ReadStack[s] && p.WriteStack[s]:
				p.ReadStack[s] = false
				s.ShutdownWrite <- struct{}{}
			case p.ReadStack[s] && !p.WriteStack[s]:
				p.ReadStack[s] = false
				if len(s.URL) > 0 {
					p.ClosedURLs[s.URL] = true
				}
				s.ClosedAt = time.Now()
				delete(p.ReadStack, s)
				delete(p.WriteStack, s)
			case !p.ReadStack[s] && p.WriteStack[s]:
				s.ShutdownWrite <- struct{}{}
			case !p.ReadStack[s] && !p.WriteStack[s]:
				if len(s.URL) > 0 {
					p.ClosedURLs[s.URL] = true
				}
				s.ClosedAt = time.Now()
				delete(p.ReadStack, s)
				delete(p.WriteStack, s)
			}
		case e := <-p.Pipes.ErrorWrite:
			s := e.Socket
			switch {
			case p.ReadStack[s] && p.WriteStack[s]:
				p.WriteStack[s] = false
				s.ShutdownRead <- struct{}{}
			case p.ReadStack[s] && !p.WriteStack[s]:
				s.ShutdownRead <- struct{}{}
			case !p.ReadStack[s] && p.WriteStack[s]:
				p.WriteStack[s] = false
				if len(s.URL) > 0 {
					p.ClosedURLs[s.URL] = true
				}
				s.ClosedAt = time.Now()
				delete(p.ReadStack, s)
				delete(p.WriteStack, s)
			case !p.ReadStack[s] && !p.WriteStack[s]:
				if len(s.URL) > 0 {
					p.ClosedURLs[s.URL] = true
				}
				s.ClosedAt = time.Now()
				delete(p.ReadStack, s)
				delete(p.WriteStack, s)
			}
		}
	}
}

// ControlPing runs an infinite loop to send pings messages to websocket write goroutine at interval t
func (p *SocketPool) ControlPing(t time.Duration) {
	defer func() {
		log.Printf("ControlPing goroutine was stopped at %v.\n\n", time.Now())
	}()
	log.Printf("ControlPing started.")
	ticker := time.NewTicker(t)

	for {
		select {
		case <-p.Pipes.StopPingControl:
			return
		case <-ticker.C:
			for s, missed := range p.PingStack {
				if missed < 2 {
					p.mtx.Lock()
					p.PingStack[s]++
					p.mtx.Unlock()
					if p.Config.IsJSON {
						s.Pool2SocketJSON <- Message{Type: 9}
					} else {
						s.Pool2SocketBytes <- Message{Type: 9}
					}
				} else {
					p.mtx.Lock()
					delete(p.PingStack, s)
					p.mtx.Unlock()
					if s.IsReadable {
						s.ShutdownRead <- struct{}{}
					}
					if s.IsWritable {
						s.ShutdownWrite <- struct{}{}
					}
				}
			}
		}
	}
}

// ControlPong runs an infinite loop to receive pong messages and register responses
func (p *SocketPool) ControlPong() {
	defer func() {
		log.Printf("ControlPong goroutine was stopped at %v.\n\n", time.Now())
	}()
	log.Printf("ControlPong started.")

	for {
		select {
		case <-p.Pipes.StopPongControl:
			return
		case s := <-p.Pipes.Pong:
			p.mtx.Lock()
			p.PingStack[s]--
			p.mtx.Unlock()
		}
	}
}

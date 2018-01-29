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

	if p.Config.PingInterval > 0 && p.Config.IsReadable && p.Config.IsWritable {
		go p.ControlPing()
		go p.ControlPong()
	}
	if p.Config.PingInterval > 0 && !p.Config.IsReadable {
		log.Print("Unable to start Ping/Pong control -- Cannot receive Pong messages from websockets that are not readable.")
	}
	if p.Config.PingInterval > 0 && !p.Config.IsWritable {
		log.Print("Unable to start Ping/Pong control -- Cannot Ping websockets that are not writable.")
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
				p.Writers.mtx.RLock()
				for socket, open := range p.Writers.Stack {
					if open {
						socket.Pool2SocketJSON <- v
					}
				}
				p.Writers.mtx.RUnlock()
			}
		}
	} else {
		for {
			select {
			case <-p.Pipes.StopWriteControl:
				return
			case v := <-p.Pipes.InboundBytes:
				p.Writers.mtx.RLock()
				for socket, open := range p.Writers.Stack {
					if open {
						socket.Pool2SocketBytes <- v
					}
				}
				p.Writers.mtx.RUnlock()
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
			p.Readers.mtx.Lock()
			p.Writers.mtx.Lock()
			switch {
			case p.Readers.Stack[s] && p.Writers.Stack[s]:
				p.Readers.Stack[s] = false
				s.ShutdownWrite <- struct{}{}
			case p.Readers.Stack[s] && !p.Writers.Stack[s]:
				p.Readers.Stack[s] = false
				if len(s.URL) > 0 {
					p.ClosedURLs.mtx.Lock()
					p.ClosedURLs.Stack[s.URL] = true
					p.ClosedURLs.mtx.Unlock()
				}
				delete(p.Readers.Stack, s)
				delete(p.Writers.Stack, s)
			case !p.Readers.Stack[s] && p.Writers.Stack[s]:
				s.ShutdownWrite <- struct{}{}
			case !p.Readers.Stack[s] && !p.Writers.Stack[s]:
				if len(s.URL) > 0 {
					p.ClosedURLs.mtx.Lock()
					p.ClosedURLs.Stack[s.URL] = true
					p.ClosedURLs.mtx.Unlock()
				}
				delete(p.Readers.Stack, s)
				delete(p.Writers.Stack, s)
			}
			p.Readers.mtx.Unlock()
			p.Writers.mtx.Unlock()
		case e := <-p.Pipes.ErrorWrite:
			s := e.Socket
			p.Readers.mtx.Lock()
			p.Writers.mtx.Lock()
			switch {
			case p.Readers.Stack[s] && p.Writers.Stack[s]:
				p.Writers.Stack[s] = false
				s.ShutdownRead <- struct{}{}
			case p.Readers.Stack[s] && !p.Writers.Stack[s]:
				s.ShutdownRead <- struct{}{}
			case !p.Readers.Stack[s] && p.Writers.Stack[s]:
				p.Writers.Stack[s] = false
				if len(s.URL) > 0 {
					p.ClosedURLs.mtx.Lock()
					p.ClosedURLs.Stack[s.URL] = true
					p.ClosedURLs.mtx.Unlock()
				}
				delete(p.Readers.Stack, s)
				delete(p.Writers.Stack, s)
			case !p.Readers.Stack[s] && !p.Writers.Stack[s]:
				if len(s.URL) > 0 {
					p.ClosedURLs.mtx.Lock()
					p.ClosedURLs.Stack[s.URL] = true
					p.ClosedURLs.mtx.Unlock()
				}
				delete(p.Readers.Stack, s)
				delete(p.Writers.Stack, s)
			}
			p.Readers.mtx.Unlock()
			p.Writers.mtx.Unlock()
		}
	}
}

// ControlPing runs an infinite loop to send pings messages to websocket write goroutines at an interval defined by PoolConfig
func (p *SocketPool) ControlPing() {
	defer func() {
		log.Printf("ControlPing goroutine was stopped at %v.\n\n", time.Now())
	}()
	log.Printf("ControlPing started.")

	var t time.Duration
	if p.Config.PingInterval < (time.Second * 30) {
		t = time.Second * 30
	} else {
		t = p.Config.PingInterval
	}
	ticker := time.NewTicker(t)

	for {
		select {
		case <-p.Pipes.StopPingControl:
			return
		case <-ticker.C:
			if p.isPingStackEmpty() {
				p.Pingers.mtx.Lock()
				for s, missed := range p.Pingers.Stack {
					if missed < 2 {
						p.Pingers.Stack[s]++
						if p.Config.IsJSON {
							s.Pool2SocketJSON <- Message{Type: 9}
						} else {
							s.Pool2SocketBytes <- Message{Type: 9}
						}
					} else {
						delete(p.Pingers.Stack, s)
						if s.IsReadable {
							s.ShutdownRead <- struct{}{}
						}
						if s.IsWritable {
							s.ShutdownWrite <- struct{}{}
						}
					}
				}
				p.Pingers.mtx.Unlock()
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
			p.Pingers.mtx.Lock()
			p.Pingers.Stack[s]--
			p.Pingers.mtx.Unlock()
		}
	}
}

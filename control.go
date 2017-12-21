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
			default:
				continue
			}
		}
	} else {
		for {
			select {
			case <-p.Pipes.StopReadControl:
				return
			case v := <-p.Pipes.Socket2PoolBytes:
				p.Pipes.OutboundBytes <- v
			default:
				continue
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
			default:
				continue
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
			default:
				continue
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
		default:
			continue
		}
	}
}

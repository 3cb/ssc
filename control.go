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
		log.Printf("ControlRead goroutine was stopped at %v.\n", time.Now())
	}()
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
		log.Printf("ControlWrite goroutine was stopped at %v.\n", time.Now())
	}()
	if p.Config.IsJSON {
		for {
			select {
			case <-p.Pipes.StopWriteControl:
				return
			case v := <-p.Pipes.InboundJSON:
				for _, socket := range p.OpenStack {
					socket.Pool2SocketJSON <- v
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
				for _, socket := range p.OpenStack {
					socket.Pool2SocketBytes <- v
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
		log.Printf("ControlShutdown goroutine was stopped at %v.\n", time.Now())
	}()
	for {
		select {
		case <-p.Pipes.StopShutdownControl:
			return
		case e := <-p.Pipes.ErrorRead:
			s := p.checkOpenStack(e.URL)
			if s != nil {
				rw, ok := p.ClosingQueue[e.URL]
				if ok == false {
					if s.IsWritable == true {
						p.ClosingQueue[e.URL] = "read"
						sock := p.OpenStack[e.URL]
						sock.ShutdownWrite <- struct{}{}
					} else {
						s.ClosedAt = time.Now()
						delete(p.OpenStack, e.URL)
						p.ClosedStack[e.URL] = newSocketInstance(e.URL, p.Config)
					}
				} else if ok == true && rw == "write" {
					delete(p.ClosingQueue, e.URL)
					s.ClosedAt = time.Now()
					delete(p.OpenStack, e.URL)
					p.ClosedStack[e.URL] = newSocketInstance(e.URL, p.Config)
				}
			}
		case e := <-p.Pipes.ErrorWrite:
			s := p.checkOpenStack(e.URL)
			if s != nil {
				rw, ok := p.ClosingQueue[e.URL]
				if ok == false {
					if s.IsReadable == true {
						p.ClosingQueue[e.URL] = "write"
						sock := p.OpenStack[e.URL]
						sock.ShutdownRead <- struct{}{}
					} else {
						s.ClosedAt = time.Now()
						delete(p.OpenStack, e.URL)
						p.ClosedStack[e.URL] = newSocketInstance(e.URL, p.Config)
					}
				} else if ok == true && rw == "read" {
					delete(p.ClosingQueue, e.URL)
					s.ClosedAt = time.Now()
					delete(p.OpenStack, e.URL)
					p.ClosedStack[e.URL] = newSocketInstance(e.URL, p.Config)
				}
			}
		default:
			continue
		}
	}
}

package ssc

import (
	"time"

	"github.com/gorilla/websocket"
)

type ListenerPool struct {
	Listeners []Listener
}

type Listener struct {
	Connection  *websocket.Conn
	isConnected bool
	OpenedAt    time.Time
	ClosedAt    time.Time
	Ponged      bool
}

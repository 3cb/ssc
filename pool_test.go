// Package ssc creates and controls pools of websocket connections --  client and server
package ssc

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestPoolServer(t *testing.T) {
	upgrader := websocket.Upgrader{}
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("failed to start test server1: %v", err)
		}
		for {
			msgType, msg, _ := conn.ReadMessage()
			if msgType == websocket.CloseMessage {
				break
			}
			err = conn.WriteMessage(msgType, msg)
			if err != nil {
				t.Fatalf("error writing to socket: %s", err)
			}
			_ = conn.WriteMessage(websocket.PingMessage, []byte(""))
		}

	}))
	u1, _ := url.Parse(srv1.URL)
	u1.Scheme = "ws"

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("failed to start test server1: %v", err)
		}
		for {
			msgType, msg, _ := conn.ReadMessage()
			if msgType == websocket.CloseMessage {
				break
			}
			err = conn.WriteMessage(msgType, msg)
			if err != nil {
				t.Fatalf("error writing to socket: %s", err)
			}
		}
	}))
	u2, _ := url.Parse(srv2.URL)
	u2.Scheme = "ws"

	srv3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("failed to start test server1: %v", err)
		}
		for {
			msgType, msg, _ := conn.ReadMessage()
			if msgType == websocket.CloseMessage {
				break
			}
			err = conn.WriteMessage(msgType, msg)
			if err != nil {
				t.Fatalf("error writing to socket: %s", err)
			}
		}
	}))
	u3, _ := url.Parse(srv3.URL)
	u3.Scheme = "ws"

	// slice of websocket servers
	urls := []string{u1.String(), u2.String()}

	// start empty server pool
	pool := NewPool(urls, time.Second*30)

	t.Run("test Start", func(t *testing.T) {
		err := pool.Start()
		if err != nil {
			t.Fatalf("unable to start socket pool: %s", err)
		}
		got := pool.Count()
		if got != len(urls) {
			t.Errorf("not all sockets connected: expected %v; got %v", len(urls), got)
		}

	})

	t.Run("test AddServerSocket", func(t *testing.T) {
		before := pool.Count()
		err := pool.AddServerSocket(u3.String())
		if err != nil {
			t.Errorf("unable to connect to ws server: %s", err)
		}
		after := pool.Count()
		if after != before+1 {
			t.Errorf("not all sockets connected: expected %v; got %v", before+1, after)
		}
	})

	// t.Run("test RemoveSocket", func(t *testing.T) {
	// 	before := pool.Count()
	// 	err := pool.RemoveSocket(u3.String())
	// 	if err != nil {
	// 		t.Errorf("unable to remove socket from stack")
	// 	}
	// 	time.Sleep(time.Second * 10)
	// 	after := pool.Count()
	// 	if after != before-1 {
	// 		t.Errorf("socket was not removed from stack: expected length %v; got length %v", before-1, after)
	// 	}
	// })

	t.Run("test WriteAll", func(t *testing.T) {
		msg := &Message{
			ID:      "",
			Type:    websocket.TextMessage,
			Payload: []byte("test message!"),
		}
		count := pool.Count()
		pool.WriteAll(msg)
		for i := 0; i < count; i++ {
			got := <-pool.Outbound
			if got.Type != msg.Type {
				t.Errorf("wrong message type: expected %v; got %v", msg.Type, got.Type)
			}
			if string(got.Payload) != string(msg.Payload) {
				t.Errorf("wrong payload: expected %v; got %v", string(msg.Payload), string(got.Payload))
			}
		}
	})

	t.Run("test Write", func(t *testing.T) {
		// test correct message format
		for _, url := range urls {
			msg := &Message{
				ID:      url,
				Type:    websocket.TextMessage,
				Payload: []byte("test message!"),
			}
			err := pool.Write(msg)
			if err != nil {
				t.Fatalf("unable to write to pool socket: %s", err)
			}
			got := <-pool.Outbound
			if got.ID != msg.ID {
				t.Errorf("wrong socket ID: expected %v; got %v", msg.ID, got.ID)
			}
			if got.Type != msg.Type {
				t.Errorf("wrong message type: expected %v; got %v", msg.Type, got.Type)
			}
			if string(got.Payload) != string(msg.Payload) {
				t.Errorf("wrong payload: expected %v; got %v", string(msg.Payload), string(got.Payload))
			}
		}

		// test bad message formats(empty string and id not in stack)
		for _ = range urls {
			err := pool.Write(&Message{
				ID:      "",
				Type:    websocket.TextMessage,
				Payload: []byte("test message!"),
			})
			if err == nil {
				t.Errorf("Write method did not return error for empty ID string")
			}
			err = pool.Write(&Message{
				ID:      "non-existent ID",
				Type:    websocket.TextMessage,
				Payload: []byte("test message!"),
			})
			if err == nil {
				t.Errorf("Write method did not return error for ID string not in stack")
			}
		}
	})

	// t.Run("test Stop", func(t *testing.T) {
	// 	before := pool.Count()
	// 	pool.Stop()
	// 	after := pool.Count()

	// 	if before == after || after != 0 {
	// 		t.Errorf("Stop method did not remove all sockets from stack")
	// 	}
	// })
}

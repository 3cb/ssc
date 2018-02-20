// Package ssc creates and controls pools of websocket connections --  client and server
package ssc

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
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

	t.Run("test List", func(t *testing.T) {
		expected := append(urls, u3.String())
		got := pool.List()
		if len(got) != len(expected) {
			t.Errorf("expected %v; got %v", expected, got)
		}
		for _, url := range expected {
			count := 0
			for _, id := range got {
				if id == url {
					break
				} else {
					count++
				}
			}
			if count == 3 {
				t.Errorf("list does not contain correct ids: expected %v; got %v", expected, got)
			}
		}
	})

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

}

func TestPoolClient(t *testing.T) {
	pool := NewPool([]string{}, time.Second*5)
	pool.Start()

	upgrader := &websocket.Upgrader{}
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := pool.AddClientSocket("client1", upgrader, w, r)
		if err != nil {
			t.Fatalf("unable to connect new client socket: %s", err)
		}
		pool.Write(&Message{
			ID:      "client1",
			Type:    websocket.TextMessage,
			Payload: []byte("Hi client1!"),
		})
	}))
	u1, _ := url.Parse(srv1.URL)
	u1.Scheme = "ws"

	t.Run("test AddClientSocket", func(t *testing.T) {
		expected := pool.Count() + 1
		conn, _, err := websocket.DefaultDialer.Dial(u1.String(), nil)
		if err != nil {
			t.Fatalf("unable to connect to socket server: %s", err)
		}
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("error reading message from pool to client: %s", err)
		}
		if msgType != websocket.TextMessage {
			t.Errorf("wrong message type: expected %v; got %v", websocket.TextMessage, msgType)
		}
		if string(msg) != "Hi client1!" {
			t.Errorf("wrong payload: expected %v; got %v", "Hi client1!", string(msg))
		}
		got := pool.Count()
		if got != expected {
			t.Errorf("socket not added to stack: expected %v; got %v", expected, got)
		}
	})

	// test writeControl with Ping
	t.Run("test Ping", func(t *testing.T) {
		time.Sleep(time.Second * 8)
	})

	t.Run("test StopControl", func(t *testing.T) {
		wg := &sync.WaitGroup{}
		wg.Add(3)
		pool.stopReadControl <- wg
		pool.stopWriteControl <- wg
		pool.stopShutdownControl <- wg
		wg.Wait()
	})
}

func TestWriteControlNoPing(t *testing.T) {
	pool := NewPool([]string{}, time.Second*0)
	pool.Start()

	upgrader := &websocket.Upgrader{}
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := pool.AddClientSocket("client1", upgrader, w, r)
		if err != nil {
			t.Fatalf("unable to connect new client socket: %s", err)
		}
		pool.WriteAll(&Message{
			Type:    websocket.TextMessage,
			Payload: []byte("Hello!"),
		})
	}))
	u1, _ := url.Parse(srv1.URL)
	u1.Scheme = "ws"

	conn, _, err := websocket.DefaultDialer.Dial(u1.String(), nil)
	if err != nil {
		t.Fatalf("unable to connect to socket server: %s", err)
	}
	msgType, msg, err := conn.ReadMessage()
	if err != nil {
		t.Errorf("error reading message from pool to client: %s", err)
	}
	if msgType != websocket.TextMessage {
		t.Errorf("wrong message type: expected %v; got %v", websocket.TextMessage, msgType)
	}
	if string(msg) != "Hello!" {
		t.Errorf("wrong message received: expected %v; got %v", "Hello!", string(msg))
	}

	// test stopWriteControl without Ping
	t.Run("test StopControl", func(t *testing.T) {
		wg := &sync.WaitGroup{}
		wg.Add(3)
		pool.stopReadControl <- wg
		pool.stopWriteControl <- wg
		pool.stopShutdownControl <- wg
		wg.Wait()
	})
}

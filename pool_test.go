// Package ssc creates and controls pools of websocket connections --  client and server
package ssc

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNewSocketPool(t *testing.T) {
	upgrader := websocket.Upgrader{}
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("failed to start test server1: %v", err)
		}
	}))
	u1, _ := url.Parse(srv1.URL)
	u1.Scheme = "ws"
	_, _, err := websocket.DefaultDialer.Dial(u1.String(), nil)
	if err != nil {
		t.Fatalf("unable to make websocket connection: %v", err)
	}

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("failed to start test server1: %v", err)
		}
	}))
	u2, _ := url.Parse(srv2.URL)
	u2.Scheme = "ws"
	_, _, err = websocket.DefaultDialer.Dial(u2.String(), nil)
	if err != nil {
		t.Fatalf("unable to make websocket connection: %v", err)
	}

	urls := []string{fmt.Sprint(u1), fmt.Sprint(u2)}

	type expected struct {
		locked bool
		reader bool
		writer bool
		pinger int
	}

	tc := []struct {
		name string
		Config
		expected
	}{
		{
			"read-only no ping",
			Config{urls, true, false, time.Second * 0},
			expected{false, true, false, 0},
		},
		{
			"read-only ping",
			Config{urls, true, false, time.Second * 20},
			expected{false, true, false, 0},
		},
		{
			"write-only no ping",
			Config{urls, false, true, time.Second * 0},
			expected{false, false, true, 0},
		},
		{
			"write-only ping",
			Config{urls, false, true, time.Second * 50},
			expected{false, false, true, 0},
		},
		{
			"read-write no ping",
			Config{urls, true, true, time.Second * 0},
			expected{false, true, true, 0},
		},
		{
			"read-write ping",
			Config{urls, true, true, time.Second * 15},
			expected{false, true, true, 0},
		},
	}

	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			pool, err := NewSocketPool(c.Config)
			if err != nil {
				t.Fatalf("unable to create SocketPool: %v", err)
			}

			if pool.Locked != c.expected.locked {
				t.Errorf("expected Locked to be 'false'; got true")
			}
			for socket, v := range pool.Readers.Stack {
				// test reader stack value
				if v != c.expected.reader {
					t.Errorf("expected reader to be %v; got %v", c.expected.reader, v)
				}
				// test writer stack value
				if pool.Writers.Stack[socket] != c.expected.writer {
					t.Errorf("expected writer to be %v; got %v", c.expected.writer, pool.Writers.Stack[socket])
				}
				// text pinger stack value
				if pool.Pingers.Stack[socket] != c.expected.pinger {
					t.Errorf("expected pinger to be %v; got %v", c.expected.pinger, pool.Pingers.Stack[socket])
				}

			}
		})

	}
}

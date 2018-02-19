**This README is NOT up to date.  Will fix soon**

# Simple Socket Controller
Built on top of gorilla toolkit's websocket package, this library allows caller to create and control a pool of websockets

NOTE: This package is still under development.

To import the package into your application:

```bash
import "github.com/3cb/ssc"
```

To install the package on your system:

```bash
go get "github.com/3cb/ssc"
```
## Pool Layout
Each WebSocket has its own readSocket and writeSocket goroutine but there is only one instance of each control goroutine per Pool:


![Diagram](https://images2.imgbox.com/6c/f0/Z1ax6br1_o.png?download=true)

## Example Usage

First create an instance of `ssc.Pool` by calling `ssc.NewPool()` which takes a configuration object as a parameter:
```go
sockets := []string{
    "wss://api.example.com/ws1",
    "wss://api.example.com/ws2",
    "wss://api.example.com/ws3",
}

pool, err := ssc.NewPool(sockets, time.Second*45)
if err != nil {
    log.Fatalf("Error starting new Socket Pool.")
	return
}
```
The above example starts a pool of websocket server connections.  In order to start an empty pool which is ready for client websockets to connect, send an empty slice for the first parameter:
```go

pool, err := ssc.NewPool([]string{}, time.Second*45])
if err != nil {
    log.Fatalf("Error starting new Socket Pool.")
	return
}
```
To add individual client connections call `pool.AddClientSocket()` from a route handler:

```go
func main() {
	r := mux.NewRouter()	// gorilla mux package

	upgrader := &websocket.Upgrader{}	// gorilla websocket package
	r.Handle("/". wsHandler(pool, upgrader))

	log.Fatal(http.ListenAndServe(":3000", r))
}

func wsHandler(pool *ssc.Pool, upgrader *websocket.Upgrader) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := pool.AddClientSocket("clientID", upgrader, w, r)
		if err != nil {
			// handle error
		}
		// do other stuff
	})
}
```
Shutdown Pool and cleanup all running goroutines by using `pool.Stop()`.  This method will shutdown all read/write goroutines and then shut down all control goroutines that are running.

Usage for server WebSockets here: https://github.com/3cb/gemini_clone/tree/go_stream

Usage for client WebSockets here: https://github.com/3cb/seattle911 and here: https://github.com/3cb/melbourne_parking

## Work Left To Do

-> TESTS!
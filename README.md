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
## SocketPool Layout
Each WebSocket has its own readSocket and writeSocket goroutine but there is only one instance of each control goroutine per SocketPool:


![Diagram](https://images2.imgbox.com/c6/d5/RbGZhCNw_o.png?download=true)

## Example Usage

First create an instance of `ssc.SocketPool` by calling `ssc.NewSocketPool()` which takes a configuration object as a parameter:
```go
sockets := []string{
    "wss://api.example.com/ws1",
    "wss://api.example.com/ws2",
    "wss://api.example.com/ws3",
}

config := ssc.Config{
        ServerURLs: sockets,
	IsReadable: true,
	IsWritable: true,
	PingInterval: time.Second*45,  //minimum of 30 seconds - if 0, pool will NOT ping sockets!!
}

pool, err := ssc.NewSocketPool(config)
if err != nil {
    log.Printf("Error starting new Socket Pool.")
	return
}
```
The above example starts a pool of websocket server connections.  In order to start an empty pool which is ready for client websockets to connect, use a config object without a slice of url strings:
```go
config := ssc.Config{
	IsReadable: true,
	IsWritable: true,
	PingInterval: time.Second*45,  //minimum of 30 seconds - if 0, pool will NOT ping sockets!!
}

pool, err := ssc.NewSocketPool(config)
if err != nil {
    log.Printf("Error starting new Socket Pool.")
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

func wsHandler(pool *ssc.SocketPool, upgrader *websocket.Upgrader) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		socket, err := pool.AddClientSocket(upgrader, w, r)	// can usually ignore socket with an _
		if err != nil {
			// handle error
		}
		// do other stuff
	})
}
```
Shutdown Socket Pool and cleanup all running goroutines by using `pool.Drain()`.  This method will shutdown all read/write goroutines and then shut down all control goroutines that are running.

Usage for server WebSockets here: https://github.com/3cb/gemini_clone/tree/go_stream

Usage for client WebSockets here: https://github.com/3cb/seattle911 and here: https://github.com/3cb/melbourne_parking

## Work Left To Do

-> TESTS!
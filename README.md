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
![Diagram](https://github.com/3cb/ssc/blob/master/diagram.png)

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
The above example starts a pool of websocket server connections.  In order to start and empty pool ready for client websockets to connect, use config object without a slice of url strings:
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
Shutdown Socket Pool and cleanup all running goroutines by using `pool.Drain()`.


## Work Left To Do

-> TESTS!
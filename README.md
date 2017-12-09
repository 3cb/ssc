# Simple Socket Controller
Built on top of gorilla toolkit's websocket package this library allows caller to create and control a pool of websockets

NOTE: This package is still under development.

To import the package into your application:

```
import "github.com/3cb/ssc"
```

To install the package on your system:

```
go get "github.com/3cb/ssc"
```


## Example Usage

First create an instance of `ssc.SocketPool` by calling `ssc.NewSocketPool()` which takes a slice of url strings and a configuration object:
```
sockets := []string{
    wss://api.example.com/ws1,
    wss://api.example.com/ws2,
    wss://api.example.com/ws3,
}

config := ssc.PoolConfig{
    IsReadable: true,
    IsWritable: true,
    IsJSON: false,
}

pool, err := ssc.NewSocketPool(sockets, config)
if err != nil {
    log.Printf("Error starting new Socket Pool.")
	return
}
```

The above example will create goroutines that read and write in bytes.  In order to read and write with JSON the caller has to set the `IsJSON` field of the config object to `true` and set the `DataJSON` field of the config object to an empty instance of a data structure that implements the `ssc.JSONReaderWriter` interface:

```
type JSONReaderWriter interface {
	JSONRead(s *Socket, toPoolJSON chan<- JSONReaderWriter, errorChan chan<- ErrorMsg) error
	JSONWrite(s *Socket, fromPoolJSON <-chan JSONReaderWriter, errorChan chan<- ErrorMsg) error
}
```

An example of this can be seen in the "go_stream" branch here: https://github.com/3cb/gemini_clone/tree/go_stream

## Work Left To Do

-> ssc.ShutdownSocket() method will close websocket connection but will leave it in the pool and merely move from OpenStack to ClosedStack

-> Replace infinite for loop with Removal channel functionality -- RemoveSocket()???

-> Make closing websocket conditional in deferred funcs for read/write groutines

-> Add option to create empty SocketPool

-> Make read/write methods for sockets unexportable

-> Make Control methods unexportable

-> Remove checkOpenStack() (value, ok := p.OpenStack[url])

-> Make websocket connection initialization concurrent -- add mtx to prevent data race

-> Add auto reconnect feature -- requires sync.Mutex field in SocketPool type to prevent data race

-> TESTS!
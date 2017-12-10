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
    "wss://api.example.com/ws1",
    "wss://api.example.com/ws2",
    "wss://api.example.com/ws3",
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
	JSONRead(s *Socket, Socket2PoolJSON chan<- JSONReaderWriter) error
	JSONWrite(s *Socket, Pool2SocketJSON <-chan JSONReaderWriter) error
}
```

An example of this can be seen in the "go_stream" branch.

Data type with methods that implement JSONReaderWriter interface:
https://github.com/3cb/gemini_clone/blob/go_stream/types/types.go

Config object with zero value instance of Data type in config object:
https://github.com/3cb/gemini_clone/blob/go_stream/handlers/handlers.go

## Work Left To Do

-> Make closing websocket conditional in deferred funcs for read/write groutines

-> Remove checkOpenStack() (value, ok := p.OpenStack[url])

-> TESTS!
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


## Example Usage

First create an instance of `ssc.SocketPool` by calling `ssc.NewSocketPool()` which takes a configuration object as a parameter:
```go
sockets := []string{
    "wss://api.example.com/ws1",
    "wss://api.example.com/ws2",
    "wss://api.example.com/ws3",
}

config := ssc.PoolConfig{
        ServerURLs: sockets,
	IsReadable: true,
	IsWritable: true,
	IsJSON: false,    // If false, messages will be read/written in bytes
	DataJSON: DataType{},
	PingInterval: time.Second*45  //minimum of 30 seconds - if 0, pool will NOT ping sockets!!
}

pool, err := ssc.NewSocketPool(sockets, config)
if err != nil {
    log.Printf("Error starting new Socket Pool.")
	return
}
```

The above example will create goroutines that read and write in bytes.  In order to read and write with JSON the caller has to set the `IsJSON` field of the config object to `true` and set the `DataJSON` field of the config object to an empty instance of a data structure that satisfies the `ssc.JSONReaderWriter` interface:

```go
type JSONReaderWriter interface {
	ReadJSON(s *Socket, b []byte, Socket2PoolJSON chan<- JSONReaderWriter) error
	WriteJSON(s *Socket) error
}
```

An example of this can be seen in the "go_stream" branch:
https://github.com/3cb/gemini_clone/blob/go_stream/types/types.go

Data type with methods that implement JSONReaderWriter interface:
```go
type Message struct {
	Type      string  `json:"type"`
	Product   string  `json:"product"`
	EventID   int     `json:"eventId"`
	Sequence  int     `json:"socket_sequence"`
	Events    []Event `json:"events"`
	Timestamp int     `json:"timestampms"`
}

type Event struct {
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Price     string `json:"price"`
	Delta     string `json:"delta"`
	Remaining string `json:"remaining"`
	Side      string `json:"side"`

	TID       int64  `json:"tid"`
	Amount    string `json:"amount"`
	MakerSide string `json:"makerSide"`
}

func (m Message) WriteJSON(s *ssc.Socket) error {
	err := s.Connection.WriteJSON(m)
	if err != nil {
		return err
	}
	return nil
}

func (m Message) ReadJSON(s *ssc.Socket, b []byte, Socket2PoolJSON chan<- ssc.JSONReaderWriter) error {
	err := json.Unmarshal(b, &m)
	if err != nil {
		return err
	}

	slice := strings.Split(s.URL, "/")
	m.Product = slice[len(slice)-1]

	Socket2PoolJSON <- m

	return nil
}
```


## Work Left To Do

-> Update comments

-> Rewrite Shutdown and Remove methods for SocketPool

-> Update this README!

-> TESTS!
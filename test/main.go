package main

import (
	"log"
	"time"

	"github.com/3cb/ssc"
)

func main() {
	sockets := []string{
		"wss://api.gemini.com/v1/marketdata/btcusd",
		"wss://api.gemini.com/v1/marketdata/ethusd",
		"wss://api.gemini.com/v1/marketdata/ethbtc",
	}

	config := ssc.Config{
		ServerURLs:   sockets,
		IsReadable:   true,
		IsWritable:   true,
		PingInterval: time.Second * 30,
	}

	pool, err := ssc.NewSocketPool(config)
	if err != nil {
		log.Fatalf("Unable to start server socket pool: %v", err)
	}

	ticker := time.NewTicker(time.Second * 30)
outer:
	for {
		select {
		case <-ticker.C:
			break outer
		case <-pool.Outbound:
			// fmt.Printf("%v\n\n", v)
		}
	}
	println("===== pre pool.Drain() =====")
	pool.Drain()
	<-ticker.C
}

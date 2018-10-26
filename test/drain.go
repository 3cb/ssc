package main

import (
	"log"
	"time"

	"github.com/3cb/ssc"
	"github.com/pkg/profile"
)

func main() {
	defer profile.Start().Stop()
	sockets := []string{
		"wss://api.gemini.com/v1/marketdata/btcusd",
		"wss://api.gemini.com/v1/marketdata/ethusd",
		"wss://api.gemini.com/v1/marketdata/ethbtc",
	}

	pool := ssc.NewPool(sockets, time.Second*0)

	err := pool.Start()
	if err != nil {
		log.Fatalf("Unable to start server socket pool: %v", err)
	}
	println(pool.Count())

	ticker := time.NewTicker(time.Second * 10)
outer:
	for {
		select {
		case <-ticker.C:
			break outer
		case _ = <-pool.Outbound:
			// println(v.ID)
		}
	}
	pool.Stop()
	// list := pool.List()
	// for _, id := range list {
	// 	pool.RemoveSocket(id)
	// }
	// time.Sleep(time.Second * 5)
	println(pool.Count())
}

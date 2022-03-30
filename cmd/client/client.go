package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/xyproto/enet"
)

func pingOnConnect(host enet.Host, ep string, reason int) {
	if reason == 0 {
		host.Write(ep, 0, []byte("hello enet"))
	}
}

func pingOnReliable(host enet.Host, ep string, chid uint8, data []byte) {
	s := string([]byte(data))

	fmt.Println(s)
	host.Disconnect(ep)
}

func installSignal() chan os.Signal {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	return c
}

func panicIfError(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	endpoint := "127.0.0.1:19091"
	c := installSignal()
	host, err := enet.NewHost("")
	panicIfError(err)
	host.SetDataHandler(pingOnReliable)
	host.SetConnectionHandler(pingOnConnect)
	host.SetDisconnectionHandler(func(enet.Host, string, int) {
		host.Stop()
	})
	host.Connect(endpoint)
	host.Run(c)
}

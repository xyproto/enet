package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/xyproto/enet"
)

func onPeerConnected(host enet.Host, peer string, ret int) {
	fmt.Printf("%v conn\n", peer)
}

func onPeerDisconnected(host enet.Host, peer string, ret int) {
	fmt.Printf("%v disconn\n", peer)
}

func installSignal() chan os.Signal {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	return c
}

func pong(host enet.Host, ep string, chanid uint8, payload []byte) {
	host.Write(ep, chanid, payload)

	fmt.Printf("dat pong %v\n", ep)
}

func panicIfError(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	endpoint := "127.0.0.1:19091"
	c := installSignal()
	host, err := enet.NewHost(endpoint)
	panicIfError(err)
	host.SetDataHandler(pong)
	host.SetConnectionHandler(onPeerConnected)
	host.SetDisconnectionHandler(onPeerDisconnected)
	host.Run(c)
}

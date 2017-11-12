package main

import (
	"flag"
	"fmt"
	"strings"

	"./tools"
	"./GUI"
)

type Peer struct {
	webServer *GUI.WebServer
	gossiper  *tools.Gossiper
}

func main() {

	// Parsing inputs
	UIPort := flag.String("UIPort", "10000", "UIPort")
	gossipPort := flag.String("gossipAddr", "localhost:5000", "gossipAddr")
	nodeName := flag.String("name", "nodeA", "nodeName")
	peers := flag.String("peers", "", "peers")
	rtimer := flag.Uint("rtimer", 60, "rtimer")
	guiAddr := flag.String("guiAddr", "localhost:8080", "Address of the GUI")
	flag.Parse()

	// Avoid :0 of being a peers if no intial peers are specified
	var peerAddrs []string
	if *peers != "" {
		peerAddrs = strings.Split(*peers, "_")
	} else {
		peerAddrs = make([]string, 0)
	}

	// Creating peer
	peer := Peer{}

	// Creating a new gossiper
	var err error
	peer.gossiper, err = tools.NewGossiper(*UIPort, *gossipPort, *nodeName, peerAddrs, *rtimer)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Creating WebServer
	peer.webServer = GUI.NewWebServer(*guiAddr, peer.sendMsg, &peer.gossiper.MessagesReceived)
	fmt.Println("Peer: server addr=", peer.webServer.Addr)

	go peer.gossiper.Run()
	peer.webServer.Run()
}

func (p *Peer) sendMsg(msg string) {
	p.gossiper.AcceptRumorMessage(tools.RumorMessage{Text:msg}, *p.webServer.Addr, true)
}
package main

import (
	"flag"
	"fmt"
	"net"

	"github.com/dedis/protobuf"
	"github.com/gjolly/Gossiper/tools/Messages"
)

func main() {
	port := flag.String("UIPort", "10000", "UIPort")
	msg := flag.String("msg", "hello", "Message")
	dest := flag.String("Dest", "", "Specify a destination for a private message")
	file := flag.String("file", "", "File to share")
	flag.Parse()

	udpAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:" + *port)
	if err != nil {
		fmt.Println(err)
		return
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		fmt.Println(err)
		return
	}

	var mess Messages.GossipMessage
	if *file == "" && *dest == "" {
		rmess := Messages.RumorMessage{Text: *msg}
		mess = Messages.GossipMessage{Rumor: &rmess}
	} else if *dest != "" {
		pmess := Messages.PrivateMessage{Text: *msg, Dest: *dest}
		mess = Messages.GossipMessage{PrivateMessage: &pmess}
	} else if *file != "" {
		mess = Messages.GossipMessage{ShareFile: &Messages.ShareFile{*file}}
	}
	buf, err := protobuf.Encode(&mess)

	if err != nil {
		fmt.Println(err)
		return
	}

	_, err = conn.WriteToUDP(buf, udpAddr)
	if err != nil {
		fmt.Println(err)
		return
	}
}

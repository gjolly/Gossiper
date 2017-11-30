package main

import (
	"flag"
	"fmt"
	"net"

	"github.com/gjolly/Gossiper/tools/Messages"
	"encoding/hex"
	"log"
)

func main() {
	port := flag.String("UIPort", "10000", "UIPort")
	msg := flag.String("msg", "hello", "Message")
	dest := flag.String("Dest", "", "Specify a destination for a private message")
	file := flag.String("file", "", "File to share")
	hash := flag.String("request", "", "File to download")
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
	defer conn.Close()

	var mess Messages.GossipMessage
	if *file == "" && *dest == "" {
		rmess := Messages.RumorMessage{Text: *msg}
		mess = Messages.GossipMessage{Rumor: &rmess}
	} else if *dest != "" && *file == "" {
		pmess := Messages.PrivateMessage{Text: *msg, Dest: *dest}
		mess = Messages.GossipMessage{PrivateMessage: &pmess}
	} else if *file != "" && *hash == "" {
		mess = Messages.GossipMessage{ShareFile: &Messages.ShareFile{*file}}
	} else if *hash != "" && *dest != "" && *file != "" {
		byteHash, err := hex.DecodeString(*hash)
		if err != nil {
			log.Println(err)
			return
		}
		mess = Messages.GossipMessage{Download: &Messages.DownloadFile{*file, byteHash, *dest}}
	}

	mess.Send(conn, *udpAddr)

}

package Messages

import (
	"net"
	"fmt"
)

type DownloadFile struct {
	FileName    string
	HashValue   []byte
	Destination string
}

func (df DownloadFile) Send(conn *net.UDPConn, addr net.UDPAddr) error {
	gossipMessage := GossipMessage{Download: &df}
	err := gossipMessage.Send(conn,addr)
	if err != nil {
		fmt.Println("error protobuf")
		return err
	}
	return nil
}
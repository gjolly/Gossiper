package Messages

import (
	"net"
	"fmt"
)

type ShareFile struct {
	Path string
}

func (sf ShareFile) Send(conn *net.UDPConn, addr net.UDPAddr) error {
	gossipMessage := GossipMessage{ShareFile: &sf}
	err := gossipMessage.Send(conn,addr)
	if err != nil {
		fmt.Println("error protobuf")
		return err
	}
	return nil
}

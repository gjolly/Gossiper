package Messages

import (
	"net"
)

type StatusMessage struct {
	Want []PeerStatus
}

func (sm StatusMessage) Send(conn *net.UDPConn, addr net.UDPAddr) error {
	gossipMessage := GossipMessage{Status: &sm}
	err := gossipMessage.Send(conn, addr)
	if err != nil {
		return err
	}
	return nil
}

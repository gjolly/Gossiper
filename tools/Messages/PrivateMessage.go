package Messages

import (
	"net"
)

type PrivateMessage struct {
	Origin   string
	ID       uint32
	Text     string
	Dest     string
	HopLimit uint32
}

func (pr *PrivateMessage) DecHopLimit() {
	pr.HopLimit -= 1
}

func (pr PrivateMessage) GetHopLimit() (uint32) {
	return pr.HopLimit
}

func (pr PrivateMessage) GetDest() (string) {
	return pr.Dest
}

func (pm PrivateMessage) Send(conn *net.UDPConn, addr net.UDPAddr) error {
	gossipMessage := GossipMessage{PrivateMessage: &pm}
	err := gossipMessage.Send(conn, addr)
	if err != nil {
		return err
	}
	return nil
}

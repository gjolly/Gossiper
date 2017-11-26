package Messages

import (
	"net"
)

type RumorMessage struct {
	Origin   string
	ID       uint32
	Text     string
}

func (m RumorMessage) String() string {
	return m.Text
}

func (rm RumorMessage) Send(conn *net.UDPConn, addr net.UDPAddr) error {
	gossipMessage := GossipMessage{Rumor: &rm}
	err := gossipMessage.Send(conn, addr)
	if err != nil {
		return err
	}
	return nil
}

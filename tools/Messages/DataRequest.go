package Messages

import (
	"net"
)

type DataRequest struct {
	Origin      string
	Destination string
	HopLimit    uint32
	FileName    string
	HashValue   []byte
}

func (dr *DataRequest) DecHopLimit() {
	dr.HopLimit -= dr.HopLimit
}

func (dr DataRequest) GetHopLimit() (uint32) {
	return dr.HopLimit
}

func (dr DataRequest) GetDest() (string) {
	return dr.Destination
}

func (dr DataRequest) Send(conn *net.UDPConn, addr net.UDPAddr) error {
	gossipMessage := GossipMessage{DataRequest: &dr}
	err := gossipMessage.Send(conn, addr)
	if err != nil {
		return err
	}
	return nil
}

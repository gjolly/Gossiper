package Messages

import (
	"net"
)

type DataReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	FileName    string
	HashValue   []byte
	Data        []byte
}

func (dr *DataReply) DecHopLimit() {
	dr.HopLimit -= 1
}

func (dr DataReply) GetHopLimit() (uint32) {
	return dr.HopLimit
}

func (dr DataReply) GetDest() (string) {
	return dr.Destination
}

func (dr DataReply) Send(conn *net.UDPConn, addr net.UDPAddr) error {
	gossipMessage := GossipMessage{DataReply: &dr}
	err := gossipMessage.Send(conn, addr)
	if err != nil {
		return err
	}
	return nil
}

func (dr *DataReply) Write(buf []byte) (int, error) {
	if dr.Data == nil {
		dr.Data = make([]byte, 0)
	}
	dr.Data = append(dr.Data, buf...)
	return len(buf), nil
}

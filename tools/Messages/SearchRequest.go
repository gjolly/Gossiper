package Messages

import "net"

type SearchRequest struct {
	Origin   string
	Budget   uint64
	Keywords []string
}

func (sr SearchRequest) Send(conn *net.UDPConn, addr net.UDPAddr) error {
	g := GossipMessage{SearchRequest: &sr}
	return g.Send(conn, addr)
}
package Messages

import "net"

type SearchReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	Results     []*SearchResult
}

type SearchResult struct {
	FileName     string
	MetafileHash []byte
	ChunkMap     []uint64
}

func (sr SearchReply) Send(conn *net.UDPConn, addr net.UDPAddr) error {
	g := GossipMessage{SearchReply: &sr}
	err := g.Send(conn, addr)
	return err
}

func (sr *SearchReply) DecHopLimit() {
	sr.HopLimit -= 1
}

func (sr SearchReply) GetHopLimit() (uint32) {
	return sr.HopLimit
}

func (sr SearchReply) GetDest() (string) {
	return sr.Destination
}
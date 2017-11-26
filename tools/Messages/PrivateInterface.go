package Messages

import "net"

type Private interface {
	DecHopLimit()
	GetHopLimit() uint32
	GetDest() string
	Send(conn *net.UDPConn, addr net.UDPAddr) error
}

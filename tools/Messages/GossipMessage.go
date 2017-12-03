package Messages

import (
	"net"
	"github.com/dedis/protobuf"
	"log"
)

type GossipMessage struct {
	Rumor          *RumorMessage
	Status         *StatusMessage
	ShareFile      *ShareFile
	DataRequest    *DataRequest
	DataReply      *DataReply
	PrivateMessage *PrivateMessage
	Download       *DownloadFile
	SearchReply    *SearchReply
	SearchRequest  *SearchRequest
}

func (g GossipMessage) String() string {
	var str string
	if g.Rumor != nil {
		str = "Rumor message: " + g.Rumor.String()
	} else if g.Status != nil {
		str = "Status Message"
	}

	return str
}

func (gm GossipMessage) Send(conn *net.UDPConn, addr net.UDPAddr) error {
	messEncode, err := protobuf.Encode(&gm)
	if err != nil {
		log.Println(err)
		return err
	}
	_, err = conn.WriteToUDP(messEncode, &addr)
	if err != nil {
		return err
	}

	return nil
}

package message

import (

	"net"
)
/* Struct definition */
type Message struct {

	// TODO: Figure out why need ptr here
	Text string
	Destination *string
	File *string
	Request *[]byte
	Keywords []string
	Budget uint64
}

type SimpleMessage struct {

	OriginalName string
	RelayPeerAddr string
	Contents string
}

type RumorMessage struct {

	Origin string
	ID uint32
	Text string
}

type PeerStatus struct {

	Identifier string
	NextID uint32
}

type StatusPacket struct {

	Want []PeerStatus
}

type  PrivateMessage struct {

	Origin string
	ID uint32
	Text string
	Destination string
	HopLimit uint32
}

type DataRequest struct {

	Origin string
	Destination string
	HopLimit uint32
	HashValue []byte
}

type DataReply struct {

	Origin string
	Destination string
	HopLimit uint32
	HashValue []byte
	Data []byte
}

type SearchRequest struct {

	Origin string
	Budget uint64
	Keywords []string
}

type SearchRequestRelayer struct {

	SearchRequest *SearchRequest
	Relayer string
}
type SearchReply struct {

	Origin string
	Destination string
	HopLimit uint32
	Results []*SearchResult
}

type SearchResult struct {

	FileName string
	MetafileHash []byte
	ChunkMap []uint64
	ChunkCount uint64
}
type GossipPacket struct {

	Simple *SimpleMessage
	Rumor *RumorMessage
	Status *StatusPacket
	Private *PrivateMessage
	DataRequest *DataRequest
	DataReply *DataReply
	SearchRequest *SearchRequest
	SearchReply *SearchReply
}

type Gossiper struct {

	address *net.UDPAddr
	conn *net.UDPConn
	Name string
}

type PacketToSend struct {

	Packet *GossipPacket
	Addr string
	Timeout chan struct{}
}

type PacketIncome struct {

	Packet *GossipPacket
	Sender string
}

type ClientMsgIncome struct {

	Msg *Message
	Sender string
}

type StatusMap map[string]uint32 

/* Convert a status packet to map */
func (status *StatusPacket) ToMap() (statusMap StatusMap) {

	statusMap = make(StatusMap)

	for _, peer_status := range status.Want {

		statusMap[peer_status.Identifier] = peer_status.NextID
	}

	return
}
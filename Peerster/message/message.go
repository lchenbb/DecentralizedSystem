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

type TxPublish struct {
	Name string
	Size int64
	MetafileHash []byte
}

type BlockPublish struct {
	PrevHash [32]byte
	Transaction TxPublish
}

type TLCMessage struct {
	Origin string
	ID uint32
	Confirmed bool
	TxBlock BlockPublish
	VectorClock *StatusPacket
	Fitness float32
}

type WrappedRumorTLCMessage struct {
	RumorMessage *RumorMessage
	TLCMessage *TLCMessage
}

func (m *WrappedRumorTLCMessage) GetOrigin() (origin string) {
	if m.RumorMessage != nil {
		origin = m.RumorMessage.Origin
	} else {
		origin = m.TLCMessage.Origin
	}
	return
}

func (m *WrappedRumorTLCMessage) GetID() (ID uint32) {
	if m.RumorMessage != nil {
		ID = m.RumorMessage.ID
	} else {
		ID = m.TLCMessage.ID
	}
	return
}
type TLCAck PrivateMessage

type GossipPacket struct {

	Simple *SimpleMessage
	Rumor *RumorMessage
	Status *StatusPacket
	Private *PrivateMessage
	DataRequest *DataRequest
	DataReply *DataReply
	SearchRequest *SearchRequest
	SearchReply *SearchReply
	TLCMessage *TLCMessage
	ACK *TLCAck
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
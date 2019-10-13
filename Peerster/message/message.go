package message

import (

	"net"
)
/* Struct definition */
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

type GossipPacket struct {

	Simple *SimpleMessage
	Rumor *RumorMessage
	Status *StatusPacket
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

type StatusMap map[string]uint32 

/* Convert a status packet to map */
func (status *StatusPacket) ToMap() (statusMap StatusMap) {

	statusMap = make(StatusMap)

	for _, peer_status := range status.Want {

		statusMap[peer_status.Identifier] = peer_status.NextID
	}

	return
}
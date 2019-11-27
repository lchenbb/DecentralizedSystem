package gossiper

import (
	"fmt"
	"encoding/hex"
	"time"
	"math"
	"strings"
	"github.com/LiangweiCHEN/Peerster/message"
	"github.com/LiangweiCHEN/Peerster/routing"
)

func (g *Gossiper) HandleTLCMessage(wrapped_pkt *message.PacketIncome) {
	/*	Step 1. Update records if the TLC message is unseen
		Step 2. Send back ACK if unseen and not confirmed
		Step 3. Trigger same process as normal rumor message 
	*/
	
	/* Step 1 */
	// Decode wrapped pkt
	sender, tlc := wrapped_pkt.Sender, wrapped_pkt.Packet.TLCMessage

	// Update status and rumor buffer
	updated := g.Update(&message.WrappedRumorTLCMessage{
		TLCMessage : tlc,
	}, sender)

	// Defer sending status back
	defer g.N.Send(&message.GossipPacket{
			Status : g.StatusBuffer.ToStatusPacket(),
			}, sender)

	if updated {
		/* Step 2 */
		if !tlc.Confirmed {
			outputStr := fmt.Sprintf("UNCONFIRMED GOSSIP origin %s ID %d file name %s size %d metahash %s\n",
			tlc.Origin,
			tlc.ID,
			tlc.TxBlock.Transaction.Name,
			tlc.TxBlock.Transaction.Size,
			hex.EncodeToString(tlc.TxBlock.Transaction.MetafileHash),
			)
			fmt.Printf(outputStr)
			g.ACK(tlc.ID, tlc.Origin)
		} else {
			outputStr := fmt.Sprintf("CONFIRMED GOSSIP origin %s ID %d file name %s size %d metahash %s\n",
				tlc.Origin,
				tlc.ID,
				tlc.TxBlock.Transaction.Name,
				tlc.TxBlock.Transaction.Size,
				hex.EncodeToString(tlc.TxBlock.Transaction.MetafileHash),
				)
				fmt.Printf(outputStr)
		}
		/* Step 3 */
		// Triger update routing
		heartbeat := false
		g.Dsdv.Ch<- &routing.OriginRelayer{
			Origin : tlc.Origin,
			Relayer : sender,
			HeartBeat : heartbeat,
		}

		// Output rumor content only if it is not heartbeat rumor
	
		wrappedMessage := &message.WrappedRumorTLCMessage {
			TLCMessage : tlc,
		}
		g.MongerRumor(wrappedMessage, "", []string{sender})
	}
}

func (g *Gossiper) ACK(ID uint32, destination string) {
	// Step 1. Get next hop of destination
	// Step 2. Construct ack msg
	// Step 3. Send ACK back to destination

	fmt.Printf("SENDING ACK origin %s ID %d\n", destination, ID)
	/* Step 1 */
	g.Dsdv.Mux.Lock()
	nextHop := g.Dsdv.Map[destination]
	g.Dsdv.Mux.Unlock()

	/* Step 2 */
	ack := &message.TLCAck{
		Origin : g.Name,
		ID : ID,
		Text : "",
		Destination : destination,
		HopLimit : g.HopLimit,
	}

	/* Step 3 */
	g.N.Send(&message.GossipPacket{
		ACK : ack,
	}, nextHop)
}

func (g *Gossiper) SendTLC(tx message.TxPublish) {
	// Step 1. Create TLCMessage
	// Step 2. Register unconfirmed TLC message
	// Step 3. Periodically monger TLC message till 
	// receive terminating signal
	// Step 4. Broadcast confirmation to all peers after receiving majority ack

	/* Step 1 */
	
	g.RumorBuffer.Mux.Lock()
	ID := uint32(len(g.RumorBuffer.Rumors[g.Name]) + 1)
	outputStr := fmt.Sprintf("UNCONFIRMED GOSSIP origin %s ID %d file name %s size %d metahash %s\n",
					g.Name,
					ID,
					tx.Name,
					tx.Size,
					hex.EncodeToString(tx.MetafileHash))

	tlc := &message.TLCMessage{
		Origin : g.Name,
		ID : ID,
		Confirmed : false,
		TxBlock : message.BlockPublish{
			Transaction : tx,
		},
		VectorClock : nil,
		Fitness : 0,
	}
	wrappedMessage := &message.WrappedRumorTLCMessage{
		TLCMessage : tlc,
	}

	// Store new tlc message into rumor buffer
	g.RumorBuffer.Rumors[g.Name] = append(g.RumorBuffer.Rumors[g.Name], &message.WrappedRumorTLCMessage{
		TLCMessage : tlc,
	})
	g.RumorBuffer.Mux.Unlock()

	// Update status
	g.StatusBuffer.Mux.Lock()
	if _, ok := g.StatusBuffer.Status[g.Name]; !ok {
		g.StatusBuffer.Status[g.Name] = 2
	} else {
		g.StatusBuffer.Status[g.Name] += 1
	}
	g.StatusBuffer.Mux.Unlock()

	// Directly confirm if less than three peers in the system
	if g.NumPeers > 3 {

		/* Step 2 */
		terminateCh := make(chan []string)
		g.TLCAckChs.Mux.Lock()
		g.TLCAckChs.Chs[ID] = terminateCh
		g.TLCAckChs.Mux.Unlock()

		/* Step 3 */
		// Trigger initial broadcast of tlc
		g.MongerRumor(wrappedMessage, "", []string{})
		fmt.Printf(outputStr)
		ticker := time.NewTicker(time.Duration(g.StubbornTimeout) * time.Second)

		consensus := false
		var witnesses []string
		for {
			select {
			case <-ticker.C:
				// Timeout before receiving enough ack
				// Trigger mongering again
				g.MongerRumor(wrappedMessage, "", []string{})
				fmt.Printf("RE-BROADCAST ID %d WITNESSES %s\n", ID, strings.Join(witnesses, ","))
			case witnesses = <-terminateCh:
				// Receive ack from majority
				// Stop periodically sending 

				if (len(witnesses) >= int(math.Ceil(float64(g.NumPeers) / 2))) {
					ticker.Stop()
					consensus = true
					break
				}
			}
			if consensus {
				break
			}
		}
	}

	/* Step 4 */
	g.RumorBuffer.Mux.Lock()
	tlc.Confirmed = true
	g.RumorBuffer.Mux.Unlock()
	fmt.Printf("RECEIVE MAJORITY ACK FOR %d th PROPOSAL\n", ID)
	g.MongerRumor(wrappedMessage, "", []string{})
}


func (g *Gossiper) HandleTLCAck() {
	// Step 0. Initialize map holding ack info
	// Step 1. Get ack from TLCAckChs
	// Step 2. Update local ack map
	// Step 3. Trigger termination if some proposal obtain majority ack

	/* Step 0 */
	ackMap := make(map[uint32]map[string]bool)
	finishMap := make(map[uint32]bool)

	/* Step 1 */
	for wrappedPkt := range g.TLCAckCh {
		
		peer := wrappedPkt.Packet.ACK.Origin
		ID := wrappedPkt.Packet.ACK.ID

		// Stop handling if this round has already finished
		if _, ok := finishMap[ID]; ok {
			continue
		}

		fmt.Printf("RECEIVE ACK OF %d FROM %s\n", ID, peer)
		/* Step 2 */
		if _, ok := ackMap[ID]; !ok {
			ackMap[ID] = make(map[string]bool)

			// ACK SELF
			ackMap[ID][g.Name] = true
		}

		if _, ok := ackMap[ID][peer]; !ok {
			ackMap[ID][peer] = true
		}

		/* Step 3 */
		if len(ackMap[ID]) >= int(math.Ceil(float64(g.NumPeers) / 2)) {
			
			g.TLCAckChs.Mux.Lock()
			terminateCh := g.TLCAckChs.Chs[ID]
			delete(g.TLCAckChs.Chs, ID)
			g.TLCAckChs.Mux.Unlock()

			// Put the witness to the channel
			witnesses := make([]string, 0)
			for k, _ := range ackMap[ID] {
				witnesses = append(witnesses, k)
			}
			terminateCh<- witnesses
			close(terminateCh)
			finishMap[ID] = true
		}
	}
}
package network

import (

	"net"
	"fmt"
	"sync"
	"strconv"
	"time"
	"github.com/dedis/protobuf"
	"github.com/DecentralizedSystem/Peerster/message"
)

type NetworkHandler struct {

	Conn *net.UDPConn
	Client_conn *net.UDPConn
	Addr *net.UDPAddr
	Send_ch chan *message.PacketToSend
	Listen_ch chan *message.PacketIncome
	Done_chs *Done_chs
	RumorTimeoutCh chan *message.PacketToSend
}

type Done_chs struct {

	Chs map[string]chan struct{}
	Mux sync.Mutex
}

func (n *NetworkHandler) Send(pkt *message.GossipPacket, dst string) {

	// Build pkt to send
	pkt_to_send := message.PacketToSend{

		Packet: pkt,
		Addr: dst,
	}

	// Pass pkt to send to send_ch
	// fmt.Printf("%p\n", pkt_to_send)
	n.Send_ch <- &pkt_to_send

	return
}


func (n *NetworkHandler) Start_sending() {
	
	// Create waitgroup obj to wait for finish of sending
	var wg sync.WaitGroup
	// Get pkt to send from send_ch and 
	// create go routine to send it

	count := 0
	for pkt_to_send := range n.Send_ch {

		count += 1
		fmt.Printf("Sending %dth packet\n", count)
		// Localize pkt_to_send
		pkt_to_send := pkt_to_send
		//fmt.Println(pkt_to_send.Packet.Rumor.Text)
		
		/*
		switch {
		case pkt_to_send.Packet.Rumor != nil:
			fmt.Print("Sending rumor\n")
		case pkt_to_send.Packet.Status != nil:
			fmt.Print("Sending status\n")
		}
		*/
		// Increment waitgroup
		wg.Add(1)
		// Localize pkt and addr
		pkt, err := protobuf.Encode(pkt_to_send.Packet)

		if err != nil {

			fmt.Print(err)
			return
		}

		addr, err := net.ResolveUDPAddr("udp", pkt_to_send.Addr)

		if err != nil {

			fmt.Print(err)
			return
		} 

		// Start go routine to send the pkt
		go func() {
			defer wg.Done()

			// Construct done channel only for rumor mongering msg
			if pkt_to_send.Packet.Rumor != nil {

				// Construct a channel to handle stop sending current packet
				done_ch := make(chan struct{})

				/* Put the channel into Done_chs map */
				n.Done_chs.Mux.Lock()
				
				// Build the key to Done_chs
				key := pkt_to_send.Addr + pkt_to_send.Packet.Rumor.Origin + strconv.Itoa(int(pkt_to_send.Packet.Rumor.ID))

				n.Done_chs.Chs[key] = done_ch
				n.Done_chs.Mux.Unlock()

				// Set up time out and ticker
				timeout := time.After(10 * time.Second)
				ticker := time.NewTicker(9 * time.Second)

				// Send msg regularly when timeout has not trigger
				for t := range ticker.C {

					
					// Drop the control signal
					_ = t

					select {

					case <-timeout:
						fmt.Println("Timeout!!!")
						// Pass pkt to Network's RumorTimeoutCh
						n.RumorTimeoutCh<- pkt_to_send
						return
					case <-done_ch:

						// Update Done_chs
						/*
						n.Done_chs.Mux.Lock()
						delete(n.Done_chs.Chs, pkt_to_send.Addr + pkt_to_send.Packet.Rumor.Origin + strconv.Itoa(int(pkt_to_send.Packet.Rumor.ID)))
						fmt.Println("Deleting " + pkt_to_send.Addr + pkt_to_send.Packet.Rumor.Origin + strconv.Itoa(int(pkt_to_send.Packet.Rumor.ID)))
						n.Done_chs.Mux.Unlock()
						*/
						//fmt.Print("Receving return signal, stop rumoring to %s\n", key)
						return

					default:
						// Send pkt in default case
						n.Conn.WriteToUDP(pkt, addr)
					}
				}
			} else {
				// Handle status pkt

				// Set up timeout and ticker
				timeout := time.After(10 * time.Second)
				ticker := time.NewTicker(9 * time.Second)

				// Send status pkt regularly when timeout has not triggered
				for t := range ticker.C {

					_ = t

					select {
					case <-timeout:
						// fmt.Println("TIMEOUT: Stop sending status")
						return
					default:
						n.Conn.WriteToUDP(pkt, addr)
					}
				}

			}
		}()
		
	}

	// Wait f all msg finish sending
	wg.Wait()
	
}


func (n *NetworkHandler) Start_listening() {

	// Create buffer and pkt container
	buffer := make([]byte, 1024)

	// Listen
	for {

		packet := new(message.GossipPacket)
		// fmt.Println("Listening")
		// Try to collect encoded pkt
		size, addr, err := n.Conn.ReadFromUDP(buffer)

		// fmt.Println("Receiving " + strconv.Itoa(size))
		if err != nil {

			fmt.Print(err)
			return
		}

		// Decode pkt
		protobuf.Decode(buffer[: size], packet)

		// Output packet for testing
		//fmt.Printf("CLIENT MESSAGE %s\n", packet.Rumor.Text)

		// Put pkt into listen channel
		n.Listen_ch <- &message.PacketIncome{
			Packet : packet,
			Sender : addr.String(),
		}
	}

	fmt.Println("Finish listening")
}


func (n *NetworkHandler) Start_listening_client() {
	// Create buffer and pkt container
	buffer := make([]byte, 1024)

	// Listen
	for {

		packet := new(message.GossipPacket)
		// fmt.Println("Listening")
		// Try to collect encoded pkt
		size, addr, err := n.Client_conn.ReadFromUDP(buffer)

		fmt.Println("Receive info from client")
		// fmt.Println("Receiving " + strconv.Itoa(size))
		if err != nil {

			fmt.Print(err)
			return
		}

		// Decode pkt
		protobuf.Decode(buffer[: size], packet)

		// Output packet for testing
		// fmt.Printf("CLIENT MESSAGE %s\n", packet.Simple.Contents)

		// Put pkt into listen channel
		n.Listen_ch <- &message.PacketIncome{
			Packet : packet,
			Sender : addr.String(),
		}
		fmt.Println("Successfully put client msg into channel")
	}

	fmt.Println("Finish listening")

}
func (n *NetworkHandler) Start_working() {

	go n.Start_listening()
	go n.Start_listening_client()
	go n.Start_sending()
}
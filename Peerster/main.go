package main 

import (
	"fmt"
	"flag"
	"net"
	"strings"
	"github.com/dedis/protobuf"
)

/** Global variable **/
var UIPort, gossipAddr, name string
var peers []string
var simple bool

/* Struct definition */
type SimpleMessage struct {

	OriginalName string
	RelayPeerAddr string
	Contents string
}

type GossipPacket struct {

	Simple *SimpleMessage
}

type Gossiper struct {

	address *net.UDPAddr
	conn *net.UDPConn
	Name string
}


func input() (UIPort string, gossipAddr string, name string, peers []string, simple bool) {

	// Set flag value containers
	flag.StringVar(&UIPort, "UIPort", "8080", "UI port num")

	flag.StringVar(&gossipAddr, "gossipAddr", "127.0.0.1:5000",
					"gossip addr")

	flag.StringVar(&name, "name", "", "name of gossiper")

	var peers_str string

	flag.StringVar(&peers_str, "peers", "", "list of peers")

	flag.BoolVar(&simple, "simple", true, "Simple broadcast or not")

	// Conduct parameter retreival
	flag.Parse()

	// Convert peers to slice
	peers = strings.Split(peers_str, ",")

	return
}

type Message struct {

	Text string
}

/** Create a new Gossiper with address and name **/
func NewGossiper(address, name string) *Gossiper {

	// Set up the connection
	udpAddr, err := net.ResolveUDPAddr("udp4", address)
	conn, err := net.ListenUDP("udp4", udpAddr)

	if err != nil {
		fmt.Println("Error nil")
		return nil
	}
	return &Gossiper{
		address: udpAddr,
		conn: conn,
		Name: name,
	}
}


/** Create a listener for msg from client **/
func NewListener(UIPort string) *net.UDPConn {

	// Build the udp address obj
	udpAddr, _ := net.ResolveUDPAddr("udp4", ":" + UIPort)

	// Set up the connection
	conn, err := net.ListenUDP("udp4", udpAddr)

	if err != nil {

		fmt.Println(err)
	}

	return conn
}


/** Broadcast method of gossiper struct **/
func (gossiper *Gossiper) Broadcast(pkt *SimpleMessage) {

	// Change the original name to self's name
	pkt.OriginalName = gossiper.Name

	// Set relay address to self's address
	pkt.RelayPeerAddr = gossiper.address.String()

	// Create encoded pkt for sending
	encoded_pkt, _ := protobuf.Encode(pkt)

	fmt.Println("broadcast pkt with size", len(encoded_pkt))
	// Send the msg to all other peers
	for i := 0; i < len(peers); i += 1 {

		fmt.Println("Broadcasting ", pkt.Contents, " to ", peers[i])

		// Get udpAddr of peer
		peer_addr, _ := net.ResolveUDPAddr("udp", peers[i])
		gossiper.conn.WriteToUDP(encoded_pkt, peer_addr)
	}
}


/** Forward method of gossiper struct **/
func (gossiper *Gossiper) Forward(pkt *SimpleMessage) {

	// Store precendence of pkt
	precedence := pkt.RelayPeerAddr

	// Set relay address to self's address
	pkt.RelayPeerAddr = gossiper.address.String()

	// Create encoded pkt for sending
	encoded_pkt, _ := protobuf.Encode(pkt)

	// Send pkt to peers apart from precendence
	for i := 0; i < len(peers); i += 1 {

		if peers[i] != precedence {

			fmt.Println("Forwarding ", pkt.Contents, " to ", peers[i])

			// Get udpAddr of peer
			peer_addr, _ := net.ResolveUDPAddr("udp", peers[i])
			gossiper.conn.WriteToUDP(encoded_pkt, peer_addr)
		}
	}

}


/** Handle forwarding **/
func HandleForward(gossiper *Gossiper, ch chan int) {

		// Create buffer
		buffer := make([]byte, 1024)

		// Create pkt container
		packet := new(SimpleMessage)

		for {
			// Read from gossiper connection
			n, _, err := gossiper.conn.ReadFromUDP(buffer)

			if err != nil { 	

				fmt.Println(err)
			}

			// Decode the received bytes
			protobuf.Decode(buffer[: n], packet)

			// Insert the address into peers for the msg comes from 
			// an unknown peer
			if packet.RelayPeerAddr != gossiper.address.String(){

				for i := 0; i < len(peers); i += 1 {

					if peers[i] == packet.RelayPeerAddr {

						continue
					} else {

						peers = append(peers, packet.RelayPeerAddr)

						break
					}
				}
			}

			// Output pkt info for testing
			fmt.Printf("SIMPLE MESSAGE origin %s from %s contents %s\n",
						packet.OriginalName, packet.RelayPeerAddr, packet.Contents)

			for i := 0; i < len(peers); i += 1 {

				fmt.Printf("%s", peers[i])

				if i < len(peers) - 1 {

					fmt.Printf(",")
				} else {

					fmt.Printf("\n")
				}
			}

			// Forward pkt
			gossiper.Forward(packet)
		}

		ch <-1
}

/** Handle broadcasting **/
func HandleBroadcast(listener *net.UDPConn, gossiper *Gossiper, ch chan int) {

	// Create buffer and pkt container
	buffer := make([]byte, 1024)
	packet := new(SimpleMessage)
	
	for {

		// Try to collect encoded pkt
		n, _, err := listener.ReadFromUDP(buffer)

		if err != nil {

			fmt.Println(err)
		}

		// Convert the encoded pkt to pkt
		protobuf.Decode(buffer[: n], packet)

		// Output pkt info for testing
		fmt.Printf("CLIENT MESSAGE %s", packet.Contents)


		for i := 0; i < len(peers); i += 1 {

			fmt.Printf("%s", peers[i])
			if i < len(peers) - 1 {

				fmt.Printf(",")
			} else {

				fmt.Printf("\n")
			}
		}

		// Broadcast the pkt
		gossiper.Broadcast(packet)
	}

	ch <- 2
}
func main() {

	// Get input parameters
	UIPort, gossipAddr, name, peers, simple = input()

	// Establish UDP gossiper at specified ip_addr: port
	gossiper := NewGossiper(gossipAddr, name)

	// Establish UDP listener for msg from client
	listener := NewListener(UIPort)

	// Handle closing channel
	defer gossiper.conn.Close()
	defer listener.Close()

	// Construct a channel between main and listener
	ch := make(chan int)

	// Handle communication between peers
	go HandleForward(gossiper, ch)

	// Handle broadcasting pkt
	go HandleBroadcast(listener, gossiper, ch)
	
	<-ch
}
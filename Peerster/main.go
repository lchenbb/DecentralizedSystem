package main

import (
	"fmt"
	"flag"
	"strings"
	"net"
	"strconv"
	"sync"
	"time"
	"log"
	"math/rand"
	"net/http"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/LiangweiCHEN/Peerster/network"
	"github.com/LiangweiCHEN/Peerster/message"
)

type Gossiper struct {

	Address string
	Conn *net.UDPConn
	Name string
	UIPort string
	GuiPort string
	Peers *PeersBuffer
	Simple bool
	N *network.NetworkHandler
	RumorBuffer *RumorBuffer
	StatusBuffer *StatusBuffer
	Ack_chs *Ack_chs
	PeerStatuses *PeerStatuses
	AntiEntropyPeriod int
}

type PeerStatus struct {

	Identifier string
	NextID uint32
}

type RumorBuffer struct {

	Rumors map[string][]*message.RumorMessage
	Mux sync.Mutex
}

type StatusBuffer struct {

	Status message.StatusMap
	Mux sync.Mutex
}

type PeersBuffer struct {

	Peers []string
	Mux sync.Mutex
}

type Ack_chs struct {

	Chs map[string]chan *message.PeerStatus
	Mux sync.Mutex
}

type PeerStatuses struct {

	Map map[string]map[string]uint32
	Mux sync.Mutex
}
/** Global variable **/
var UIPort, gossipAddr, name string
var peers []string
var simple bool
var antiEntropy int

func input() (UIPort string, gossipAddr string, name string, peers []string, simple bool, antiEntropy int) {

	// Set flag value containers
	flag.StringVar(&UIPort, "UIPort", "8080", "UI port num")

	flag.StringVar(&gossipAddr, "gossipAddr", "127.0.0.1:5000",
					"gossip addr")

	flag.StringVar(&name, "name", "", "name of gossiper")

	var peers_str string

	flag.StringVar(&peers_str, "peers", "", "list of peers")

	flag.BoolVar(&simple, "simple", false, "Simple broadcast or not")

	flag.IntVar(&antiEntropy, "antiEntropy", 1, "antiEntroypy trigger period")
	// Conduct parameter retreival
	flag.Parse()

	// Convert peers to slice
	peers = strings.Split(peers_str, ",")

	return
}

func InitGossiper(UIPort, gossipAddr, name string, simple bool, peers []string, antiEntropy int) (g *Gossiper) {

	// Establish gossiper addr and conn
	addr, _ := net.ResolveUDPAddr("udp", gossipAddr)
	conn, _ := net.ListenUDP("udp", addr)

	// Establish client addr and conn
	client_addr, _ := net.ResolveUDPAddr("udp", ":" + UIPort)
	client_conn, _ := net.ListenUDP("udp", client_addr)

	// Set up GuiPort
	GuiPort, _ := strconv.Atoi(UIPort) 
	offset, _ := strconv.Atoi(strings.Split(gossipAddr, ":")[1])
	GuiPort += offset
	GuiPortStr := strconv.Itoa(GuiPort)

	// Create gossiper
	g = &Gossiper{
		Address : gossipAddr,
		Conn : conn,
		Name : name,
		UIPort : UIPort,
		GuiPort : GuiPortStr,
		Peers : &PeersBuffer{

			Peers : peers,
		},
		Simple : simple,
		N : &network.NetworkHandler{

			Conn : conn,
			Addr : addr,
			Client_conn : client_conn,
			Send_ch : make(chan *message.PacketToSend),
			Listen_ch : make(chan *message.PacketIncome),
			Done_chs : &network.Done_chs{

				Chs : make(map[string]chan struct{}),
			},
			RumorTimeoutCh : make(chan *message.PacketToSend),
		},
		RumorBuffer : &RumorBuffer{

			Rumors : make(map[string][]*message.RumorMessage),
		},
		StatusBuffer : &StatusBuffer{

			Status : make(message.StatusMap),
		},
		Ack_chs : &Ack_chs{
			Chs: make(map[string]chan *message.PeerStatus),
		},
		PeerStatuses : &PeerStatuses {
			Map : make(map[string]map[string]uint32),
		},
		AntiEntropyPeriod : antiEntropy,
	}

	return 
}

func main() {

	// Get input parameters
	UIPort, gossipAddr, name, peers, simple, antiEntropy = input()

	// Set up gossiper
	g := InitGossiper(UIPort, gossipAddr, name, simple, peers, antiEntropy)
	
	// Start gossiper's work
	g.Start_working()

	// Send sth using network
	
	// TODO: Set terminating condition
	for {
		time.Sleep(10 * time.Second)
	}
	return
}


// Gossiper start working 
func (gossiper *Gossiper) Start_working() {

	// Start network working
	gossiper.N.Start_working()

	// Start Receiving
	gossiper.Start_handling()

	// Start antiEntropy sending
	gossiper.Start_antiEntropy()

	// Start catching timeout rumor
	// gossiper.HandleRumorMongeringTimeout()
	gossiper.HandleGUI()
}


// Handle receiving msg
func (gossiper *Gossiper) Start_handling() {

	go func() {

		for pkt := range gossiper.N.Listen_ch {

			// Start handling packet content
			switch {

			case pkt.Packet.Simple != nil:
				if gossiper.Simple && pkt.Packet.Simple.OriginalName != "client" {
					gossiper.UpdatePeers(pkt.Packet.Simple.RelayPeerAddr)
				}
				gossiper.HandleSimple(pkt)

			case pkt.Packet.Rumor != nil:
				// Update peers
				gossiper.UpdatePeers(pkt.Sender)

				// Print peers
				gossiper.PrintPeers()

				go gossiper.HandleRumor(pkt)

			case pkt.Packet.Status != nil:
				// Update peers
				gossiper.UpdatePeers(pkt.Sender)

				// Print peers
				fmt.Print("PEERS ")
				for i, s := range gossiper.Peers.Peers {

					fmt.Print(s)
					if i < len(gossiper.Peers.Peers) - 1 {
						fmt.Print(",")
					}
				}
				fmt.Println()
				go gossiper.HandleStatus(pkt)
			}
		}
	}()
}


// Begin antiEntropy to ensure the delivery of packets
func (gossiper *Gossiper) Start_antiEntropy() {

	go func() {

		// Set up ticker
		ticker := time.NewTicker(time.Duration(gossiper.AntiEntropyPeriod) * time.Second)

		for t := range ticker.C {

			// Drop useless tick
			_ = t

			// Conduct antiEntropy job
			gossiper.Peers.Mux.Lock()
			rand_peer := gossiper.Peers.Peers[rand.Intn(len(gossiper.Peers.Peers))]
			gossiper.Peers.Mux.Unlock()

			// Send status to selected peer
			gossiper.N.Send(&message.GossipPacket{
				Status : gossiper.StatusBuffer.ToStatusPacket(),
			}, rand_peer)
		}
	}()
}


// Update peer with given address
func (gossiper *Gossiper) UpdatePeers(peer_addr string) {

	gossiper.Peers.Mux.Lock()
	defer gossiper.Peers.Mux.Unlock()

	// Try to find peer addr in self's buffer
	for _, addr := range gossiper.Peers.Peers {

		if peer_addr == addr {
			return
		}
	}

	// Put it in self's buffer if it is absent
	gossiper.Peers.Peers = append(gossiper.Peers.Peers, peer_addr)
}


// Handle Rumor msg
func (g *Gossiper) HandleRumor(wrapped_pkt *message.PacketIncome) {

	// Decode wrapped pkt
	sender, rumor := wrapped_pkt.Sender, wrapped_pkt.Packet.Rumor

	// Defer sending status back
	defer g.N.Send(&message.GossipPacket{
			Status : g.StatusBuffer.ToStatusPacket(),
			}, sender)

	// Update status and rumor buffer
	updated := g.Update(rumor, sender)

	// Trigger rumor mongering if it is new
	if updated {

		// Output rumor content
		fmt.Printf("RUMOR origin %s from %s ID %s contents %s\n", rumor.Origin, sender, strconv.Itoa(int(rumor.ID)), rumor.Text)
		g.MongerRumor(rumor, "", []string{sender})
	}

}

func (g *Gossiper) Update(rumor *message.RumorMessage, sender string) (updated bool) {

	/** Compare origin and seq_id with that in self.status **/

	// Lock Status
	g.StatusBuffer.Mux.Lock()
	defer g.StatusBuffer.Mux.Unlock()

	// Initialize flag showing whether peer is known
	known_peer := false

	for origin, nextID := range g.StatusBuffer.Status {

		// Found rumor origin in statusbuffer
		if origin == rumor.Origin {

			known_peer = true

			// Update rumor buffer if current rumor is needed
			if nextID == rumor.ID {
				g.RumorBuffer.Mux.Lock()
				g.RumorBuffer.Rumors[origin] = append(g.RumorBuffer.Rumors[origin], rumor)
				g.RumorBuffer.Mux.Unlock()

				// Update StatusBuffer
				g.StatusBuffer.Status[origin] += 1

				updated = true

				fmt.Println("Receive rumor originated from " + rumor.Origin + " with ID " +
				 strconv.Itoa(int(rumor.ID)) + " relayed by " + sender)
				return
			}
		}
	}

	// Handle rumor originated from a new peer
	if rumor.ID == 1 && !known_peer {

		// Put entry for origin into self's statusBuffer
		g.StatusBuffer.Status[rumor.Origin] = 2
		// Buffer current rumor
		g.RumorBuffer.Mux.Lock()
		g.RumorBuffer.Rumors[rumor.Origin] = []*message.RumorMessage{rumor}
		g.RumorBuffer.Mux.Unlock()
		updated = true

		fmt.Println("Receive rumor originated from " + rumor.Origin + " with ID " + strconv.Itoa(int(rumor.ID)) +
		  " relayed by " + sender)
		return
	}

	// Fail to update, either out of date or too advanced
	updated = false

	return
}

func (g *Gossiper) MongerRumor(rumor *message.RumorMessage, target string, excluded []string) {

	// 1. Select a random peer if no target specified
	// 2. Create spoofing ack and timeout channel
	// 3. Send the rumor
	// 4. FlipCoinMonger if rumor is needed or timeout

	// Select a random peer to continue monger

	// Step 1
	var peer_addr string

	if target == "" {
		var ok bool
		peer_addr, ok = g.SelectRandomPeer(excluded)

		if !ok {
			return
		}
	} else {

		peer_addr = target
	}

	// Step 2 & 4
	go func() {
		
		timeout := time.After(10 * time.Second)
		// Generate and store ack_ch
		ack_ch := make(chan *message.PeerStatus)
		key := peer_addr + rumor.Origin + strconv.Itoa(int(rumor.ID))
		g.Ack_chs.Mux.Lock()
		g.Ack_chs.Chs[key] = ack_ch
		g.Ack_chs.Mux.Unlock()
		// Generate timeout ch

		// Start waiting 
		for {

			select {

			case peerStatus := <-ack_ch:

				g.Ack_chs.Mux.Lock()
				delete(g.Ack_chs.Chs, key)
				close(ack_ch)
				g.Ack_chs.Mux.Unlock()

				if peerStatus.NextID == rumor.ID {

					g.FlipCoinMonger(rumor, []string{peer_addr})
				}
				return
			case <-timeout:
				g.Ack_chs.Mux.Lock()
				delete(g.Ack_chs.Chs, key)
				close(ack_ch)
				g.Ack_chs.Mux.Unlock()
				
				g.FlipCoinMonger(rumor, []string{peer_addr})
				return
			}
		}
	}()

	// Step 3 
	// Output mongering msg
	fmt.Printf("MONGERING with %s\n", peer_addr)

	// Monger Rumor to selected peer
	g.N.Send(&message.GossipPacket{Rumor : rumor,}, peer_addr)
}


func (g *Gossiper) SelectRandomPeer(excluded []string) (rand_peer_addr string, ok bool) {

	g.Peers.Mux.Lock()
	defer g.Peers.Mux.Unlock()

	available_peers := make([]string, 0)

	// Get slice of availble peers
	for _, peer := range g.Peers.Peers {

		is_excluded := false
		for _, ex_peer := range excluded {

			if peer == ex_peer {

				is_excluded = true
				break
			}
		}

		if !is_excluded {

			available_peers = append(available_peers, peer)
		}
	}

	// Return if no available peer exists
	if len(available_peers) == 0 {
		ok = false
		return
	}

	// Get a rand peer addr
	rand_peer_addr = available_peers[rand.Intn(len(available_peers))]
	ok = true
	return
}

// Handle status pkt 
func (g *Gossiper) HandleStatus(wrapped_pkt *message.PacketIncome) {

	// 1. Convert peer_status to map
	// 2. Ack rumors sended to peer
	// 3. Check whether need to provide or request for mongering

	// Step 1. Decode sender and pkt
	sender, peer_status := wrapped_pkt.Sender, wrapped_pkt.Packet.Status
	peer_status_map := peer_status.ToMap()

	// Ouput peer status information
	fmt.Printf("STATUS from %s ", sender)
	for k, v := range peer_status_map {

		fmt.Printf("peer %s nextID %s ", k, strconv.Itoa(int(v)))
	}
	fmt.Println()
	
	// Step 2. Ack all pkts received or not needed by peer
	go g.Ack(peer_status_map, sender)

	// Step 3. Provide new mongering or Request mongering
	moreUpdated := g.MoreUpdated(peer_status_map)

	switch moreUpdated {

	case 1:
		g.ProvideMongering(peer_status_map, sender)
	case -1:
		g.RequestMongering(peer_status_map, sender)
	default:
		// Already handled in Ack
		fmt.Printf("IN SYNC WITH %s\n", sender)
	}
}


// Ack rumor mongering
func (g *Gossiper) Ack(peer_status_map message.StatusMap, sender string) {

	// Check all possible pkts being sent to peer
	// Ack them if necessary

	// Step 1. Ack necessary pkts
	// Step 2. Update PeerStatuses

	g.PeerStatuses.Mux.Lock()
	if _, ok := g.PeerStatuses.Map[sender]; !ok {

		g.PeerStatuses.Map[sender] = make(map[string]uint32)
	}

	// TODO: Decide whether need to use lock on Ack_chs
	g.Ack_chs.Mux.Lock()

	// Loop through all origin to ack
	for origin, nextid := range peer_status_map {

		if _, ok := g.PeerStatuses.Map[sender][origin]; !ok {

			g.PeerStatuses.Map[sender][origin] = uint32(1)
		}

		// Construct newest peerstatus for current origin
		peerStatus := &message.PeerStatus{

			Identifier : origin,
			NextID : nextid,
		}

		// Attempt to ack rumors sent to peer from current origin
		for id := g.PeerStatuses.Map[sender][origin]; id <= nextid; id += 1 {

			if ack_ch, ok := g.Ack_chs.Chs[sender + origin + strconv.Itoa(int(nextid))]; ok {

				ack_ch<- peerStatus
			}
		}

		// Update PeerStasus for current origin
		g.PeerStatuses.Map[sender][origin] = nextid
	}

	g.Ack_chs.Mux.Unlock()
	g.PeerStatuses.Mux.Unlock()
}


func (g *Gossiper) MoreUpdated(peer_status message.StatusMap) (moreUpdated int) {

	// Check which of peer and self is more updated

	g.StatusBuffer.Mux.Lock()
	defer g.StatusBuffer.Mux.Unlock()

	// Loop through self status first
	for k, v := range g.StatusBuffer.Status {

		peer_v, ok := peer_status[k]
		// Return Self more updated if not ok or peer_v < v
		if !ok || peer_v < v {

			moreUpdated = 1
			return
		}
	}

	// Loop through peer to check if peer more update
	for k, v := range peer_status {

		self_v, ok := g.StatusBuffer.Status[k]

		// Return peer more updated if not ok or self_v < v
		if !ok || self_v < v {

			moreUpdated = -1
			return
		}
	}

	// Return zero if in sync state
	moreUpdated = 0
	return
}

func (rb *RumorBuffer) get(origin string, ID uint32) (rumor *message.RumorMessage) {

	rb.Mux.Lock()
	rumor = rb.Rumors[origin][ID - 1]
	rb.Mux.Unlock()
	return
}


/* Construct Status Packet from local Status Buffer */
func (sb *StatusBuffer) ToStatusPacket() (st *message.StatusPacket) {

	Want := make([]message.PeerStatus, 0)

	// fmt.Println("Requesting status buffer lock")
	sb.Mux.Lock()
	defer sb.Mux.Unlock()

	// defer fmt.Println("Releasing status buffer lock")
	for k, v := range sb.Status {

		Want = append(Want, message.PeerStatus{
			Identifier : k,
			NextID : v,
		})
	}

	st = &message.StatusPacket{
		Want : Want,
	}

	return
}


func (g *Gossiper) ProvideMongering(peer_status message.StatusMap, sender string) {

	//fmt.Println("Provide Mongering")
	// Lock status buffer for comparison
	g.StatusBuffer.Mux.Lock()
	defer g.StatusBuffer.Mux.Unlock()

	// Find the missing rumor with least ID to monger
	for k, v := range g.StatusBuffer.Status {

		switch peer_v, ok := peer_status[k]; {

		// Send the first rumor from current origin if peer have not heard
		// from it
		case !ok:
			g.MongerRumor(g.RumorBuffer.get(k, 1), sender, []string{})
			return
		case peer_v < v:
			g.MongerRumor(g.RumorBuffer.get(k, peer_v), sender, []string{})
			// fmt.Printf("Mongering to %s with rumor originates from %s of seq id %d\n", sender, k, peer_v)
			return
		}
	}
}


func (g *Gossiper) RequestMongering(peer_status message.StatusMap, sender string) {

	g.N.Send(&message.GossipPacket{
		Status : g.StatusBuffer.ToStatusPacket(),
	}, sender)
}


// Handle Simple Message
func (g *Gossiper) HandleSimple(wrapped_pkt *message.PacketIncome) {
	// 0. Handle simple flag case
	// 1. Construct and store rumor
	// 2. Update self's status
	// 3. Trigger rumor mongering

	// Step 0. Handle simple flag case
	packet := wrapped_pkt.Packet

	if g.Simple {

		if packet.Simple.OriginalName == "client" {

			// Output msg content
			fmt.Printf("CLIENT MESSAGE %s\n", packet.Simple.Contents)

			g.PrintPeers()
			// Broadcast
			packet.Simple.OriginalName = g.Name
			packet.Simple.RelayPeerAddr = g.Address

			g.Peers.Mux.Lock()
			for _, peer_addr := range g.Peers.Peers {

				g.N.Send(packet, peer_addr)
			}
			g.Peers.Mux.Unlock()
		} else {

			// Output msg content
			fmt.Printf("SIMPLE MESSAGE origin %s from %s contents %s\n",
						packet.Simple.OriginalName,
						packet.Simple.RelayPeerAddr,
						packet.Simple.Contents)
			g.PrintPeers()
			// Broadcast pkt to all peers apart from relayers
			g.Peers.Mux.Lock()
			packet.Simple.RelayPeerAddr = g.Address
			for _, peer_addr := range g.Peers.Peers {

				if peer_addr != packet.Simple.RelayPeerAddr {

					g.N.Send(packet, peer_addr)
				}
			}
			g.Peers.Mux.Unlock()
		}
		return
	}

	// Step 1. Construct rumor message
	// Output msg content
	fmt.Printf("CLIENT MESSAGE %s\n", packet.Simple.Contents)

	g.RumorBuffer.Mux.Lock()
	defer g.RumorBuffer.Mux.Unlock()

	rumor := &message.RumorMessage{
				Origin : g.Name,
				ID : uint32(len(g.RumorBuffer.Rumors[g.Name]) + 1),
				Text : packet.Simple.Contents,
			}

	// Store rumor
	g.RumorBuffer.Rumors[g.Name] = append(g.RumorBuffer.Rumors[g.Name], rumor)

	// Step 2. Update status
	g.StatusBuffer.Mux.Lock()
	defer g.StatusBuffer.Mux.Unlock()

	if _, ok := g.StatusBuffer.Status[g.Name]; !ok {

		g.StatusBuffer.Status[g.Name] = 2
	} else {

		g.StatusBuffer.Status[g.Name] += 1
	}

	// Step 3. Trigger rumor mongering
	g.MongerRumor(rumor, "", []string{})
}


// Handle timeout resend
func (g *Gossiper) HandleRumorlsMongeringTimeout() {

	go func() {
		for timeout_pkt := range g.N.RumorTimeoutCh {

			g.FlipCoinMonger(timeout_pkt.Packet.Rumor, []string{timeout_pkt.Addr})
		}
	} ()

}


// Handle flip coin mongering
func (g *Gossiper) FlipCoinMonger(rumor *message.RumorMessage, excluded []string) {

	// Flip a coin to decide whether to continue mongering
	continue_monger := rand.Int() % 2

	if continue_monger == 0 {

		return
	} else {

		if peer_addr, ok := g.SelectRandomPeer(excluded); !ok {

			return
		} else {

			fmt.Printf("FLIPPED COIN sending rumor to %s\n", peer_addr)
			g.MongerRumor(rumor, peer_addr, []string{})
		}
	}
}


func (g *Gossiper) PrintPeers() {
	fmt.Print("PEERS ")
	for i, s := range g.Peers.Peers {

		fmt.Print(s)
		if i < len(g.Peers.Peers) - 1 {
			fmt.Print(",")
		}
	}
	fmt.Println()
}


func (g *Gossiper) HandleGUI() {

	// Register router
	r := mux.NewRouter()

	// Register handlers
	r.HandleFunc("/message", g.MessageGetHandler).
		Methods("GET", "OPTIONS")
	r.HandleFunc("/node",  g.NodeGetHandler).
		Methods("GET", "OPTIONS")
	r.HandleFunc("/message", g.MessagePostHandler).
		Methods("POST", "OPTIONS")
	r.HandleFunc("/node", g.NodePostHandler).
		Methods("POST", "OPTIONS")
	r.HandleFunc("/id", g.IDGetHandler).
		Methods("GET", "OPTIONS")


	fmt.Printf("Starting webapp on address http://127.0.0.1:%s\n", g.GuiPort)

	srv := &http.Server{

		Handler : r,
		Addr : fmt.Sprintf("127.0.0.1:%s", g.GuiPort),
		WriteTimeout: 15 * time.Second,
		ReadTimeout: 15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
}

func (g *Gossiper) MessageGetHandler(w http.ResponseWriter, r *http.Request) {

	enableCors(&w)

	var messages struct {
		Messages []message.RumorMessage `json:"messages"`
	}

	messages.Messages = g.GetMessages()

	json.NewEncoder(w).Encode(messages)
}

func (g *Gossiper) GetMessages() ([]message.RumorMessage){
	// Return all rumors

	buffer := make([]message.RumorMessage, 0)

	for _, list := range g.RumorBuffer.Rumors {

		for _, rumor := range list {

			buffer = append(buffer, *rumor)
		}
	}

	return buffer
}

func (g *Gossiper) MessagePostHandler(w http.ResponseWriter, r *http.Request) {

	enableCors(&w)
	var message struct {

		Text string `json:"text"`
	}

	json.NewDecoder(r.Body).Decode(&message)

	fmt.Printf("Receive new msg from GUI%v",message)
	g.PostNewMessage(message.Text)

	g.AckPost(true, w)
}

func (g *Gossiper) PostNewMessage(text string) {

	// Create Simple msg
	wrapped_pkt := &message.PacketIncome{
					Packet: &message.GossipPacket{
								Simple : &message.SimpleMessage{
									OriginalName : "client",
									RelayPeerAddr : "",
									Contents : text,
									},
								},
					Sender : "",
					}

	// Trigger handle simple msg
	go g.HandleSimple(wrapped_pkt)
}

func (g *Gossiper) NodeGetHandler(w http.ResponseWriter, r *http.Request) {

	enableCors(&w)

	var peers struct {
		Nodes []string `json:"nodes"`
	}

	peers.Nodes = g.GetPeers()
    
	json.NewEncoder(w).Encode(peers)
}

func (g *Gossiper) GetPeers() ([]string) {
     
	// TODO: Decide whether need to lock
	return g.Peers.Peers
}

func (g *Gossiper) NodePostHandler(w http.ResponseWriter, r *http.Request) {

	enableCors(&w)
	var peer struct {
		Addr string `json:"addr"`
	}

	json.NewDecoder(r.Body).Decode(&peer)

	g.AddNewNode(peer.Addr)

	g.AckPost(true, w)
}

func (g *Gossiper) AddNewNode(addr string) {

	g.Peers.Mux.Lock()
	g.Peers.Peers = append(g.Peers.Peers, addr)
	g.Peers.Mux.Unlock()
}

func (g *Gossiper) IDGetHandler(w http.ResponseWriter, r *http.Request) {

	enableCors(&w)
	var ID struct {

		ID string `json:"id"`
	}

	ID.ID = g.GetPeerID()

	json.NewEncoder(w).Encode(ID)
}

func (g *Gossiper) GetPeerID() (ID string) {

	ID = g.Name
	return
}

func (g *Gossiper) AckPost(success bool, w http.ResponseWriter) {

	enableCors(&w)
	var response struct {
		Success bool `json:"success"`
	}
	response.Success = success 
	json.NewEncoder(w).Encode(response)
}
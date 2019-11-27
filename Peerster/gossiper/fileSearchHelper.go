package gossiper

import (
	"fmt"
	"strings"
	"github.com/LiangweiCHEN/Peerster/message"
)

func (g *Gossiper) TriggerSearch() {
	// Send file search request to others when self is the origin
	go func() {

		for request := range g.FileSharer.Searcher.SendCh {
			target_addr, ok := g.SelectRandomPeer([]string{}, 1)
			if !ok { return }
			request.Origin = g.Name
			g.N.Send(&message.GossipPacket{
				SearchRequest: request,
			}, target_addr[0])
			fmt.Printf("SENDING REQUEST FOR %s with budget %d\n", strings.Join(request.Keywords, ","), request.Budget)
		}
	}()
}


func (g *Gossiper) DistributeSearch() {
	// Distribute the request to neighbours aprt from replayer evenly

	for requestRelayer := range g.SearchDistributeCh {

		rand_peer_slice, ok := g.SelectRandomPeer([]string{requestRelayer.Relayer},
			int(requestRelayer.SearchRequest.Budget))
		if !ok {
			fmt.Println("CANNOT FIND A RANDOM PEER TO RELAY!!!")
			continue
		}
		// Trigger search with budget 1 if len(rand_peer_slice) == request.Budget
		budget := int(requestRelayer.SearchRequest.Budget)
		if len(rand_peer_slice) == budget {

			for _, peer := range rand_peer_slice {
				fmt.Printf("RELAYING SEARCH REQUEST FOR %s TO %s WITH BUDGET %d\n",
					strings.Join(requestRelayer.SearchRequest.Keywords, ","),
					peer,
					1)
				g.N.Send(&message.GossipPacket{
					SearchRequest: &message.SearchRequest{
						Origin:   requestRelayer.SearchRequest.Origin,
						Budget:   uint64(1),
						Keywords: requestRelayer.SearchRequest.Keywords,
					},
				}, peer)
			}
		} else {
			// Trigger send with most average budget if len(rand_per_slice) < request.Budget
			low_budget := budget / len(rand_peer_slice)
			high_budget_peer := budget % len(rand_peer_slice)
			for _, peer := range rand_peer_slice {
				var budget_to_send int
				if high_budget_peer > 0 {
					budget_to_send = low_budget + 1
				} else {
					budget_to_send = low_budget
				}
				fmt.Printf("RELAYING SEARCH REQUEST FOR %s TO %s WITH BUDGET %d\n",
					strings.Join(requestRelayer.SearchRequest.Keywords, ","),
					peer,
					budget_to_send)
				g.N.Send(&message.GossipPacket{
					SearchRequest: &message.SearchRequest{
						Origin:   requestRelayer.SearchRequest.Origin,
						Budget:   uint64(budget_to_send),
						Keywords: requestRelayer.SearchRequest.Keywords,
					},
				}, peer)
			}
		}
	}
}
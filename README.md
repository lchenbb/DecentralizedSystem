# DecentralizedSystem

## Components: Clients, Gossipers

## Operations

1. Client
  * Gossip msg
  
  ```$GOPATH/bin/client -UIPort XXXX -msg YOUR_MSG```
  * Send private msg
  
  ```$GOPATH/bin/client -UIPort XXXX -msg YOUR_MSG -dest Name_of_receiving_gossiper```
  * Set file to be sharable
  
  ```$GOPATH/bin/client -UIPort XXXX -file YOUR_FILE_ADDR```
  * Request for file from some gossip peer
  
  ```$GOPATH/bin/client -UIPort XXXX -file LOCAL_FILE_NAME -request METAHASH_OF_REQUESTED_FILE```
  
2. Gossiper
  * Broadcast msg
  
  ```$GOPATH/bin/Peerster -UIPort XXXX -name X -gossipAddr xxx.xxx.xxx.xxx:xxxx -peers List_of_gossip_peer_addresses -simple```
  
  * Monger Rumor with Anti-entropy
  
   ```$GOPATH/bin/Peerster -UIPort XXXX -name X -gossipAddr xxx.xxx.xxx.xxx:xxxx -peers List_of_gossip_peer_addresses```
  
## Properties
* Peer completion: Automated finding new peers during communication
* Reliable gossiping: Guarantee delivering rumor in RumorMongering mode
* Routing: Automated updating routing table during communication

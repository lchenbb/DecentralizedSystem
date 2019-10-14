package main

import (
	"fmt"
	"flag"
	"net"
	"github.com/dedis/protobuf"
	"github.com/LiangweiCHEN/Peerster/message"
)


/* Struct definition */
func input() (UIPort string, msg string) {

	// Set cmd flag value containers
	flag.StringVar(&UIPort, "UIPort", "8080", "UI port number")

	flag.StringVar(&msg, "msg", "", "Msg to be sent")

	// Parse cmd values
	flag.Parse()

	return
}

func main() {

	UIPort, msg := input()

	// Create dst address
	dst_addr, _ := net.ResolveUDPAddr("udp4", ":" + UIPort)

	// Create UDP 'connection'
	conn, _ := net.DialUDP("udp4", nil, dst_addr)

	defer conn.Close()

	// Create a gossiper msg
	pkt := &message.Message{
		Text : msg,
	}

	// Encode the msg
	msg_bytes, err := protobuf.Encode(pkt)

	if err != nil {

		fmt.Println(err)
	}

	// Send the msg to the server
	fmt.Println("Sending to gossiper")
	conn.Write(msg_bytes)

	return
}
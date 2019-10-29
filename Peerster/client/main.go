package main

import (
	"fmt"
	"flag"
	"net"
	
	"encoding/base64"
	"github.com/dedis/protobuf"
	"github.com/LiangweiCHEN/Peerster/message"
)


/* Struct definition */
func input() (UIPort string, msg string, dest string, file, request string) {

	// Set cmd flag value containers
	flag.StringVar(&UIPort, "UIPort", "8080", "UI port number")

	flag.StringVar(&msg, "msg", "", "Msg to be sent")

	flag.StringVar(&dest, "dest", "", "Private Msg Destination")

	flag.StringVar(&file, "file", "", "File to be indexed")

	flag.StringVar(&request, "request", "", "metahash of the file to be requested")

	// Parse cmd values
	flag.Parse()

	return
}

func main() {

	UIPort, msg, dest, file, request := input()

	// Create dst address
	dst_addr, _ := net.ResolveUDPAddr("udp4", ":" + UIPort)

	// Create UDP 'connection'
	conn, _ := net.DialUDP("udp4", nil, dst_addr)

	defer conn.Close()

	// Create a gossiper msg
	var destPtr, filePtr *string
	var requestPtr *[]byte
	if dest == ""{
		destPtr = nil
	} else {
		destPtr = &dest
	}

	if file == "" {
		filePtr = nil
	} else {
		filePtr = &file
	}

	requestBytes := make([]byte, 32)
	fmt.Println(request)
	requestBytes, err := base64.URLEncoding.DecodeString(request)
	if err != nil {
		fmt.Println(err)
		
	}
	fmt.Println(len(requestBytes))
	if request == "" {
		requestPtr = nil
	} else {
		requestPtr = &requestBytes
	}
	pkt := &message.Message{
		Text : msg,
		Destination : destPtr,
		File : filePtr,
		Request : requestPtr,
	}

	//fmt.Println(pkt.Request)
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
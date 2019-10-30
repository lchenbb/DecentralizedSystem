package fileSharing

import (
	"os"
	"io"
	"fmt"
	"sync"
	"time"
	"bytes"
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"github.com/LiangweiCHEN/Peerster/network"
	"github.com/LiangweiCHEN/Peerster/message"
	"github.com/LiangweiCHEN/Peerster/routing"
)

type IndexFile struct {

	FileName string
	Size int
	MetaFileName string
	MetaFileHash []byte
}


type FileIndexer struct {

	SharedFolder string
}

type RequestReplyChMap struct {

	Map map[string]chan *message.DataReply
	Mux sync.Mutex
}

type HashValueMap struct {

	Map map[[32]byte][]byte
	Mux sync.Mutex
}

type ChunkHashMap struct {

	Map map[string]bool
	Mux sync.Mutex
}

type FileSharer struct {

	N *network.NetworkHandler
	Indexer *FileIndexer
	RequestReplyChMap *RequestReplyChMap
	HashValueMap *HashValueMap
	HopLimit uint32
	Origin string
	RequestTimeout int
	IndexFileMap map[string]*IndexFile
	ChunkHashMap *ChunkHashMap
	Dsdv *routing.DSDV
}

func (sharer *FileSharer) Request(hashPtr *[]byte, dest string, ch chan *message.DataReply, tmp *[]byte) {
	// 1. Register requestReplyChannel and ticker
	// 2. Send request to dest
	// 3. If timeout: Resend
	// 4. If receive reply from requestReplyChannel and not empty: trigger requestChunk
	// 5. Return failure in request

	
		temp := *hashPtr
		hash := make([]byte, 0)
		for _, v := range temp {
			hash = append(hash, v)
		}

		// Step 1
		request := &message.DataRequest{

			Origin : sharer.Origin,
			Destination : dest,
			HopLimit : sharer.HopLimit,
			HashValue : hash,
		}
		gossipPacket := &message.GossipPacket{

			DataRequest : request,
		}

		replyCh := make(chan *message.DataReply)
		sharer.RequestReplyChMap.Mux.Lock()
		sharer.RequestReplyChMap.Map[dest + string(hash)] = replyCh
		sharer.RequestReplyChMap.Mux.Unlock()

		ticker := time.NewTicker(time.Duration(sharer.RequestTimeout) * time.Second)

		// Step 2

		sharer.N.Send(gossipPacket, sharer.Dsdv.Map[dest])

		// fmt.Printf("Sending file request to %s\n", sharer.Dsdv.Map[dest])
		for {

			if tmp != nil {
				//fmt.Printf("CHECKPOINT 3.5 chunk 2 hash is %s", base64.URLEncoding.EncodeToString((*tmp)[32: ]))
			}

			select {

			case <-ticker.C:
				// Step 3: Timeout -> resend
				sharer.N.Send(gossipPacket, sharer.Dsdv.Map[dest])

			case reply := <-replyCh:

				// Step 4: Break and return if empty reply
				if len(reply.Data) == 0 {

					fmt.Println(request.HashValue)
					fmt.Printf("Peer %s does not contain value for hash %s\n", request.Origin, base64.URLEncoding.EncodeToString(request.HashValue))
					ch<- nil
					return
				}

				// Step 5. Trigger requestChunks if reply is valid and non-empty
				hashValueArray := sha256.Sum256(reply.Data)
				if bytes.Equal(hashValueArray[:], reply.HashValue) {

					//fmt.Println("Server's reply is valid, returning")
					ch<- reply

					if tmp != nil {
						//fmt.Printf("CHECKPOINT 5 chunk 2 hash is %s", base64.URLEncoding.EncodeToString((*tmp)[32: ]))
					}
					return
				}
			}
		}
	
}

func (sharer *FileSharer) requestMetaFile(metahash []byte, dest string) ([]byte) {

	ch := make(chan *message.DataReply, 1)
	defer close(ch)
	sharer.Request(&metahash, dest, ch, nil)
	reply := <-ch

	fmt.Println("reply", reply)
	if reply == nil {
		return nil
	} else {
		return reply.Data
	}
}

func (sharer *FileSharer) RequestFile(fileNamePtr *string, metahashPtr *[]byte, destPtr *string) {

	go func() {
		// Localize the variables 
		fileName := *fileNamePtr
		metahash := *metahashPtr
		dest := *destPtr

		fmt.Printf("DOWNLOADING metafile of %s from %s\n", fileName, dest)
		chunkHashes := sharer.requestMetaFile(metahash, dest)
		backupChunkHashes := make([]byte, 0)
		for _, v := range chunkHashes {
			backupChunkHashes = append(backupChunkHashes, v)
		}

		if chunkHashes != nil {

			// TODO: Modify request chunks to parallel version
			

			
				var wg sync.WaitGroup
				contentCh := make(chan []byte, len(chunkHashes) / 32)

				// Request chunks
				for i := 0; i < len(chunkHashes); i += 32 {

					wg.Add(1)
					// Localize chunkhash
					chunkHash := make([]byte, 0)
					for _, v := range chunkHashes[i : i + 32] {
						chunkHash = append(chunkHash, v)
					}
					// fmt.Printf("DOWNLOADING %s chunk %d from %s with hash %s\n", fileName, i + 1, dest, base64.URLEncoding.EncodeToString(chunkHash))
					
					// fmt.Printf("BEFORE chunk 2 is %s\n", base64.URLEncoding.EncodeToString(chunkHashes[32 : 64]))
					sharer.requestChunk(&chunkHash, dest, contentCh, &wg, &chunkHashes)

					// Renew chunkHashes
					copy(chunkHashes, backupChunkHashes)
				}

				wg.Wait()

				// Merge chunks, index them and store the complete obj
				content := make([]byte, 0)
				for i := 0; i < len(chunkHashes) / 32; i += 1 {

					chunk := <-contentCh
					
					fmt.Printf("Retrived chunk is")
					fmt.Println(chunk)
					
					content = append(content, chunk...)
				}
				close(contentCh)

				// fmt.Println(content)
				file, err := os.Create("_Downloads/" + fileName)
				if err != nil{
					fmt.Println(err)
					fmt.Println("invalid address")
					return
				}
				_, err = file.Write(content)
				defer file.Close()
				if err != nil {

					fmt.Println(err)
					return
				}
				fmt.Printf("RECONSTRUCTED file %s", fileName)
				// TODO: Index retrived obj
		
				byteSlice := []byte{167,20,93,6,234,62,159,34,77,40,7,251,104,235,244,29,151,98,150,23,126,129,72,111,209,173,33,62,115,140,34,8,34,237,248,179,12,16,72,213,73,81,44,33,65,117,175,100,144,188,145,118,196,118,87,155,152,100,236,203,217,14}

				fmt.Println(base64.URLEncoding.EncodeToString(byteSlice))
		}
	}()
}

func (sharer *FileSharer) requestChunk(chunkHashPtr *[]byte, dest string,
										 contentCh chan []byte,
										wg *sync.WaitGroup,
										tmp *[]byte) {

	chunkHash := *chunkHashPtr

	// fmt.Printf("IN REQUEST CHUNK, the hash is %s\n", base64.URLEncoding.EncodeToString(chunkHash))
	// fmt.Printf("CHECK POINT 2 chunk 2 is %s", base64.URLEncoding.EncodeToString((*tmp)[32 : 64]))
	ch := make(chan *message.DataReply, 1)
	defer close(ch)
	sharer.Request(chunkHashPtr, dest, ch, tmp)
	reply := <-ch

	if reply == nil {

		wg.Done()
		fmt.Printf("Fail to request chunk with hash %s from %s\n",
					base64.URLEncoding.EncodeToString(chunkHash),
					dest)
		os.Exit(-1)
		return
	} else {

		// Push data into channel
		chunk := make([]byte, len(reply.Data))
		copy(chunk, reply.Data)
		contentCh<- chunk

		// fmt.Printf("CHECKPOINT 6 chunk 2 hash is %s", base64.URLEncoding.EncodeToString(chunkHash[32: ]))
		wg.Done()
		return
	}
}


func (sharer *FileSharer) HandleReply(wrapped_pkt *message.PacketIncome) {
	// 1. Notify the requesting routine if it still exists
	// 2. Close the requestReply channel

	// Step 1
	dataReply := wrapped_pkt.Packet.DataReply
	key := dataReply.Origin + string(dataReply.HashValue)
	sharer.RequestReplyChMap.Mux.Lock()
	if ch, ok := sharer.RequestReplyChMap.Map[key]; ok {

		ch<- dataReply

		// Step 2
		close(ch)
		delete(sharer.RequestReplyChMap.Map, key)
	}
	sharer.RequestReplyChMap.Mux.Unlock()

	// fmt.Printf("Receive %v from server\n", dataReply.Data)
	return
}


func (sharer *FileSharer) HandleRequest(wrapped_pkt *message.PacketIncome) {
	// 1. If find hash in metahashes, store chunks on disk and put chunk hashes
	// in chunkHashList if not done yet, return metafile
	// 2. If find chunks in chunkHashList, return chunk

	// Step 1
	fmt.Printf("REQUEST HASH IS %s\n", base64.URLEncoding.EncodeToString(wrapped_pkt.Packet.DataRequest.HashValue))
	dataRequest := wrapped_pkt.Packet.DataRequest
	hash := dataRequest.HashValue
	key := string(hash)
	// key := base64.URLEncoding.EncodeToString(hash)

	if indexFile, ok := sharer.IndexFileMap[key + "_meta"]; ok{
		// Handle the case where a metaFile exists locally is requested

		// Open file and metafile
		fmt.Printf("Opening %s\n", sharer.Indexer.SharedFolder + "/" + indexFile.MetaFileName)
		metaFile, err := os.Open(sharer.Indexer.SharedFolder + "/" + indexFile.MetaFileName)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer metaFile.Close()

		// Store chunks in chunkHashMap
		chunkHash := make([]byte, 32)
		metafile := make([]byte, 0)

		// TODO: Prevent duplicated put chunk hash into map
		for {

			// Read current chunk hash and put it in chunkHashMap
			bytesread, err := metaFile.Read(chunkHash)
			if err != nil {
				if err != io.EOF {
					fmt.Println(err)
					return
				}

				// End reading if EOF
				break
			}

			if bytesread != 32 {
				fmt.Println("Invalid SHA256 hash")
				return
			}

			// fmt.Printf("Reading chunk hash %s", string(chunkHash))
			metafile = append(metafile, chunkHash...)
			// TODO: check whether there is need for lock
			sharer.ChunkHashMap.Mux.Lock()
			sharer.ChunkHashMap.Map[string(chunkHash)] = true
			sharer.ChunkHashMap.Mux.Unlock()
		}

		// fmt.Println("Finished obtaining metafile")
		// Send back metaFile
		sharer.N.Send(&message.GossipPacket{

			DataReply : &message.DataReply{

				Origin : sharer.Origin,
				Destination : dataRequest.Origin,
				HopLimit : sharer.HopLimit,
				HashValue : dataRequest.HashValue,
				Data : metafile,
			},
		}, sharer.Dsdv.Map[dataRequest.Origin])
	} else if _, ok := sharer.ChunkHashMap.Map[key]; ok {
		// Handle the case where a chunk exists locally is requested

		// Read chunk from storage
		chunkFile, err := os.Open(sharer.Indexer.SharedFolder + "/" + base64.URLEncoding.EncodeToString(hash))
		fmt.Printf("Openning %s\n", sharer.Indexer.SharedFolder + "/" + base64.URLEncoding.EncodeToString(hash))
		if err != nil {
			fmt.Println(err)
			return
		}
		defer chunkFile.Close()
		const bufferSize = 1024 * 8
		buffer := make([]byte, bufferSize + 1)
		bytesread, err := chunkFile.Read(buffer)
		if err != nil {
			fmt.Println(err)
			return
		}

		// Send chunk back
		dataReply := &message.DataReply{

			Origin : sharer.Origin,
			Destination : dataRequest.Origin,
			HopLimit : sharer.HopLimit,
			HashValue : dataRequest.HashValue,
			Data : buffer[: bytesread],
		}

		fmt.Printf("SENDING FILE PART with length %d\n", len(buffer))
		fmt.Println(dataReply.Data)

		sharer.N.Send(&message.GossipPacket{

			DataReply : dataReply,
		}, sharer.Dsdv.Map[dataReply.Destination])
	} else {
		// The requested stuff does not exist locally, send empty reply back

		dataReply := &message.DataReply{

			Origin : sharer.Origin,
			Destination : dataRequest.Origin,
			HopLimit : sharer.HopLimit,
			HashValue : dataRequest.HashValue,
			Data : make([]byte, 0),
		}

		sharer.N.Send(&message.GossipPacket{

			DataReply : dataReply,
		}, sharer.Dsdv.Map[dataReply.Destination])
	}
}

func (sharer *FileSharer) CreateIndexFile(fileNamePtr *string) (err error) {

	indexFile, err := sharer.Indexer.CreateIndexFile(fileNamePtr)
	sharer.IndexFileMap[string(indexFile.MetaFileHash) + "_meta"] = indexFile
	fmt.Printf("Create index file for %s with value %v named %s\n", indexFile.FileName,
																 indexFile.MetaFileHash,
																  string(indexFile.MetaFileHash))
	return															  
}

func (indexer *FileIndexer) CreateIndexFile(fileNamePtr *string) (indexFile *IndexFile, err error) {
	// 1. Read chunks and compute hashes
	// 2. Compute metahash

	// Open file
	fileName := *fileNamePtr
	fileName = indexer.SharedFolder + "/" + fileName
	const bufferSize = 1024 * 8

	file, err := os.Open(fileName)

	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	// Read chunks
	buffer := make([]byte, bufferSize)
	metafile := make([]byte, 0)
	totalSize := 0

	for {

		// Read current chunk
		bytesread, err := file.Read(buffer)
		if err != nil {
			if err != io.EOF {
				fmt.Println(err)
				return nil, err
			}
			break
		}
		totalSize += bytesread

		// Compute and store hash of current chunk
		hashArray := sha256.Sum256(buffer[: bytesread])
		metafile = append(metafile, hashArray[:]...)
		fmt.Println("The hash array has length", len(hashArray))

		// Store chunk locally
		hashName := base64.URLEncoding.EncodeToString(hashArray[:])
		chunkObj, err := os.Create(indexer.SharedFolder + "/" + hashName)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		_, err = chunkObj.Write(buffer[: bytesread])
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		chunkObj.Close()

	}

	// Compute metahash
	metaHashArray := sha256.Sum256(metafile)
	metahash := metaHashArray[:]

	// Store metafile
	metaHashName := base64.URLEncoding.EncodeToString(metahash)
	metafileObj, err := os.Create(indexer.SharedFolder + "/" + metaHashName + "_meta")
	defer metafileObj.Close()

	if err != nil {
		fmt.Println("Fail to open")
		fmt.Println(err)
		return
	}

	n2, err := metafileObj.Write(metafile)
	if err != nil {
		fmt.Println("Fail to write")
		fmt.Println(err)
		return
	}

	fmt.Printf("Write metafile with %d bytes\n", n2)

	// Build indexFile obj
	indexFile = &IndexFile{

		FileName : fileName,
		Size : totalSize,
		MetaFileName : metaHashName + "_meta",
		MetaFileHash : metahash,
	}

	return
}


func main() {

	indexer := FileIndexer{

		SharedFolder : "_SharedFiles",
	}

	tmp := "trivial"
	indexFile, err := indexer.CreateIndexFile(&tmp)

	f, err := os.Create(indexer.SharedFolder + "/trivial_meta")
	if err != nil {
		return
	}

	w := bufio.NewWriter(f)
	fmt.Println(indexFile.MetaFileHash)
	fmt.Println(len(base64.URLEncoding.EncodeToString(indexFile.MetaFileHash)))
	w.WriteString(base64.URLEncoding.EncodeToString(indexFile.MetaFileHash))
	w.Flush()
	f.Close()
}
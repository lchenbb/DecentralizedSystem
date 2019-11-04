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
	"encoding/hex"
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
	Downloading *Downloading 
}

type Downloading struct {
	Map map[string]chan *message.DataReply
	Mux sync.Mutex
}
func (sharer *FileSharer) Request(hashPtr *[]byte, dest string, ch chan *message.DataReply, notification string) {
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
		fmt.Printf(notification)
		sharer.N.Send(gossipPacket, sharer.Dsdv.Map[dest])

		// fmt.Printf("Sending file request to %s\n", sharer.Dsdv.Map[dest])
		for {

			select {

			case <-ticker.C:
				// Step 3: Timeout -> resend
				fmt.Printf(notification)
				sharer.N.Send(gossipPacket, sharer.Dsdv.Map[dest])

			case reply := <-replyCh:

				// Step 4: Break and return if empty reply
				if len(reply.Data) == 0 {

					// fmt.Println(request.HashValue)
					fmt.Printf("Peer %s does not contain value for hash %s\n", request.Origin, hex.EncodeToString(request.HashValue))
					ch<- nil
					return
				}

				// Step 5. Trigger requestChunks if reply is valid and non-empty
				hashValueArray := sha256.Sum256(reply.Data)
				if bytes.Equal(hashValueArray[:], reply.HashValue) {

					//fmt.Println("Server's reply is valid, returning")
					ch<- reply
					return
				}
			}
		}
	
}

func (sharer *FileSharer) requestMetaFile(metahash []byte, dest string, notification string) ([]byte) {

	ch := make(chan *message.DataReply, 1)
	defer close(ch)
	sharer.Request(&metahash, dest, ch, notification)
	reply := <-ch

	// Store metafile to local if it does not exist
	file, err := os.Create(sharer.Indexer.SharedFolder +  "/" + hex.EncodeToString(metahash) + "_meta")
	if err != nil{
		fmt.Println(err)
		fmt.Println("invalid address")
		os.Exit(1)
	}

	_, err = file.Write(reply.Data)
	defer file.Close()
	if err != nil {

		fmt.Println(err)
		os.Exit(1)
	}

	sharer.IndexFileMap[string(metahash)] = nil

	// fmt.Println("reply", reply)
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

		metaNotification := fmt.Sprintf("DOWNLOADING metafile of %s from %s\n", fileName, dest)
		chunkHashes := sharer.requestMetaFile(metahash, dest, metaNotification)

		// fmt.Println("Finish metafile request")
		// Return if chunkHash is empty
		if len(chunkHashes) == 0 {
			return
		}

		backupChunkHashes := make([]byte, 0)
		for _, v := range chunkHashes {
			backupChunkHashes = append(backupChunkHashes, v)
		}

		// Trigger chunk downloading handler
		metaHashStr := hex.EncodeToString(metahash)
		sharer.Downloading.Mux.Lock()

		var downloadCh chan *message.DataReply
		if ch, ok := sharer.Downloading.Map[metaHashStr]; !ok {

			ch = make(chan *message.DataReply)
			sharer.Downloading.Map[metaHashStr] = ch
			go sharer.HandleDownloading(fileName, metaHashStr, backupChunkHashes, ch)
			downloadCh = ch
		} else {

			downloadCh = ch
		}
		sharer.Downloading.Mux.Unlock()

		if chunkHashes != nil {

			// TODO: Modify request chunks to parallel version
			
				var wg sync.WaitGroup
				contentCh := make(chan *message.DataReply, len(chunkHashes) / 32)

				// Request chunks
				for i := 0; i < len(chunkHashes); i += 32 {

					wg.Add(1)

					// Localize chunkhash
					chunkHash := make([]byte, 0)
					for _, v := range chunkHashes[i : i + 32] {
						chunkHash = append(chunkHash, v)
					}
					
					// Request chunk with notification
					notification := fmt.Sprintf("DOWNLOADING %s chunk %d from %s\n", fileName, (i / 32) + 1,
					 dest)
					sharer.requestChunk(&chunkHash, dest, contentCh, &wg, notification)

					// Renew chunkHashes
					copy(chunkHashes, backupChunkHashes)
				}

				wg.Wait()

				// Put non-empty chunks into download channel
				for i := 0; i < len(chunkHashes) / 32; i += 1 {

					reply := <-contentCh
					
					if len(reply.Data) != 0 {
						downloadCh<- reply
					}
				}
				close(contentCh)

				// TODO: Index retrived obj
	
				// fmt.Println(hex.EncodeToString(byteSlice))
		}
	}()
}

func (sharer *FileSharer) requestChunk(chunkHashPtr *[]byte, dest string,
										 contentCh chan *message.DataReply,
										wg *sync.WaitGroup,
										notification string) {

	chunkHash := *chunkHashPtr

	// fmt.Printf("IN REQUEST CHUNK, the hash is %s\n", hex.EncodeToString(chunkHash))
	// fmt.Printf("CHECK POINT 2 chunk 2 is %s", hex.EncodeToString((*tmp)[32 : 32]))
	ch := make(chan *message.DataReply, 1)
	defer close(ch)
	sharer.Request(chunkHashPtr, dest, ch, notification)
	reply := <-ch

	if reply == nil {

		wg.Done()
		fmt.Printf("Fail to request chunk with hash %s from %s\n",
					hex.EncodeToString(chunkHash),
					dest)
		os.Exit(-1)
		return
	} else {
		// Localize hashvalue and data
		hashvalue := make([]byte, len(reply.HashValue))
		copy(hashvalue, reply.HashValue)
		data := make([]byte, len(reply.Data))
		copy(data, reply.Data)

		reply.HashValue = hashvalue  
		reply.Data = data

		// Push data into channel
		contentCh<- reply

		// fmt.Printf("CHECKPOINT 6 chunk 2 hash is %s", hex.EncodeToString(chunkHash[32: ]))
		wg.Done()
		return
	}
}


func (sharer *FileSharer) HandleReply(wrapped_pkt *message.PacketIncome) {
	// 1. Drop the reply if its chunk does not hash to hashvalue field
	// 2. Notify the requesting routine if it still exists
	// 3. Close the requestReply channel

	// Step 1
	dataReply := wrapped_pkt.Packet.DataReply
	tmp := sha256.Sum256(dataReply.Data)
	if (bytes.Compare(tmp[:], dataReply.HashValue)) != 0 {
		fmt.Println("Invalid reply")
		return
	}

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
	dataRequest := wrapped_pkt.Packet.DataRequest
	hash := dataRequest.HashValue
	key := string(hash)
	// key := hex.EncodeToString(hash)

	if _, ok := sharer.IndexFileMap[key]; ok{
		// Handle the case where a metaFile exists locally is requested

		// Open file and metafile
		// fmt.Printf("Opening %s\n", sharer.Indexer.SharedFolder + "/" + indexFile.MetaFileName)
		metaFile, err := os.Open(sharer.Indexer.SharedFolder + "/" + hex.EncodeToString(hash) + "_meta")
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
		chunkFile, err := os.Open(sharer.Indexer.SharedFolder + "/" + hex.EncodeToString(hash))
		// fmt.Printf("Openning %s\n", sharer.Indexer.SharedFolder + "/" + hex.EncodeToString(hash))
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

		// fmt.Printf("SENDING FILE PART with length %d\n", len(buffer))
		// fmt.Println(dataReply.Data)

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
	sharer.IndexFileMap[string(indexFile.MetaFileHash)] = indexFile
	// fmt.Printf("Create index file for %s with value %v named %s\n", indexFile.FileName,
	// 															 indexFile.MetaFileHash,
	// 															  string(indexFile.MetaFileHash))
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
		// fmt.Println("The hash array has length", len(hashArray))

		// Store chunk locally
		hashName := hex.EncodeToString(hashArray[:])
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
	metaHashName := hex.EncodeToString(metahash)
	metafileObj, err := os.Create(indexer.SharedFolder + "/" + metaHashName + "_meta")
	defer metafileObj.Close()

	if err != nil {
		fmt.Println("Fail to open")
		fmt.Println(err)
		return
	}

	_, err = metafileObj.Write(metafile)
	if err != nil {
		fmt.Println("Fail to write")
		fmt.Println(err)
		return
	}

	// fmt.Printf("Write metafile with %d bytes\n", n2)

	// Build indexFile obj
	indexFile = &IndexFile{

		FileName : fileName,
		Size : totalSize,
		MetaFileName : metaHashName,
		MetaFileHash : metahash,
	}

	return
}

func (sharer *FileSharer) HandleDownloading(fileName, metaHashStr string, chunkHashes []byte, ch chan *message.DataReply) {
	// Step 1. Construct list of hashes of chunk to download
	// Step 2. Get dataReply from channel, update storage if new
	// Step 3. Loop 2 until all chunks are downloaded
	// Step 4. Create downloaded file, stop handling downloading current file

	/* Step 1*/
	chunkMap := make(map[string]bool)
	chunkDataMap := make(map[string][]byte)
	chunkHashStrList := make([]string, 0)
	chunkHashSlice := make([][]byte, 0)
	count := 0
	total := len(chunkHashes) / 32

	for i := 0; i < len(chunkHashes) / 32; i += 1 {

		chunkHashSlice = append(chunkHashSlice, make([]byte, 32))
		copy(chunkHashSlice[i], chunkHashes[i : i + 32])
		chunkHashStr := hex.EncodeToString(chunkHashes[i * 32 : i * 32 + 32])
		chunkMap[chunkHashStr] = false
		chunkHashStrList = append(chunkHashStrList, chunkHashStr)
	}

	/* Step 2 */
	for reply := range ch {

		chunkHash := reply.HashValue
		chunkHashStr := hex.EncodeToString(chunkHash)
		// fmt.Printf("CHUNKHASH IS %s\n", chunkHashStr)
		if chunkMap[chunkHashStr] == false {

			// Handle new chunk receive
			count += 1
			chunkMap[chunkHashStr] = true
			chunkDataMap[chunkHashStr] = reply.Data
			sharer.ChunkHashMap.Mux.Lock()
			sharer.ChunkHashMap.Map[string(chunkHash)] = true
			sharer.ChunkHashMap.Mux.Unlock()

			// Store new received chunk
			dst := sharer.Indexer.SharedFolder + "/" + chunkHashStr

			f, err := os.Open(dst)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			w := bufio.NewWriter(f)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			w.Write(reply.Data)
			w.Flush()

			// Stop receiving if already received all the chunks
			if count == total {

				break
			}
		}
	}

	/* Step 4 */
	sharer.Downloading.Mux.Lock()
	delete(sharer.Downloading.Map, metaHashStr)
	close(ch)
	sharer.Downloading.Mux.Unlock()

	content := make([]byte, 0)
	for _, key := range chunkHashStrList {

		content = append(content, chunkDataMap[key]...)
	}

	//fmt.Printf("%s", content)
	// Create download dir if it does not exist
	if _, err := os.Stat("_Downloads"); os.IsNotExist(err) {

		err := os.Mkdir("_Downloads", 0775)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

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
	fmt.Printf("RECONSTRUCTED file %s\n", fileName)
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
	// fmt.Println(indexFile.MetaFileHash)
	// fmt.Println(len(hex.EncodeToString(indexFile.MetaFileHash)))
	w.WriteString(hex.EncodeToString(indexFile.MetaFileHash))
	w.Flush()
	f.Close()
}
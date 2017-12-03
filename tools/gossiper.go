package tools

import (
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/dedis/protobuf"
	"sync"
	"github.com/gjolly/Gossiper/tools/Messages"
	"os"
	"log"
	"io"
	"encoding/hex"
)

// Gossiper -- Discripe a node of a Gossip network
type Gossiper struct {
	UIAddr           *net.UDPAddr
	gossipAddr       *net.UDPAddr
	UIConn           *net.UDPConn
	gossipConn       *net.UDPConn
	name             string
	peers            ListPeers
	vectorClock      []Messages.PeerStatus
	idMessage        uint32
	MessagesReceived ListMessages
	exchangeEnded    chan bool
	RoutingTable     RoutingTable
	mutex            *sync.Mutex
	rtimer           uint
	PrivateMessages  []Messages.PrivateMessage
	FileShared       ListFiles
	currentDownloads ListFiles
	downloadState    ListStates
	workingPath      string
	PendingRequests  PendingRequests
	searchState      bool
}

// NewGossiper -- Return a new gossiper structure
func NewGossiper(UIPort, gossipPort, identifier string, peerAddrs []string, rtimer uint, workingPath string) (*Gossiper, error) {
	// For UIPort
	UIUdpAddr, err := net.ResolveUDPAddr("udp4", ":"+UIPort)
	if err != nil {
		return nil, err
	}
	UIConn, err := net.ListenUDP("udp4", UIUdpAddr)
	if err != nil {
		return nil, err
	}
	// For gossipPort
	gossipUdpAddr, err := net.ResolveUDPAddr("udp4", gossipPort)
	if err != nil {
		return nil, err
	}
	gossipConn, err := net.ListenUDP("udp4", gossipUdpAddr)
	if err != nil {
		return nil, err
	}

	g := &Gossiper{
		UIAddr:           UIUdpAddr,
		gossipAddr:       gossipUdpAddr,
		UIConn:           UIConn,
		gossipConn:       gossipConn,
		name:             identifier,
		peers:            ListPeers{make(map[string]Peer, 0), &sync.RWMutex{}},
		vectorClock:      make([]Messages.PeerStatus, 0),
		idMessage:        1,
		MessagesReceived: ListMessages{make(map[string](map[uint32]Messages.RumorMessage)), &sync.RWMutex{}},
		exchangeEnded:    make(chan bool),
		RoutingTable:     *newRoutingTable(),
		mutex:            &sync.Mutex{},
		rtimer:           rtimer,
		FileShared:       ListFiles{make(map[string]MetaData), &sync.RWMutex{}},
		currentDownloads: ListFiles{make(map[string]MetaData), &sync.RWMutex{}},
		downloadState:    ListStates{make(map[string]*state), &sync.RWMutex{}},
		workingPath:      workingPath,
		PendingRequests:  PendingRequests{make(map[string]*Request), &sync.RWMutex{}, 0},
		searchState:      false,
	}

	for _, peerAddr := range peerAddrs {
		udpAddr, err := net.ResolveUDPAddr("udp4", peerAddr)
		g.AddPeer(*udpAddr)
		if err != nil {
			fmt.Println("Error:", err)
			return nil, err
		}
	}
	return g, nil
}

func (g Gossiper) excludeAddr(excludedAddrs string) (addrs []string) {
	addrs = make([]string, 0)
	g.peers.Mutex.Lock()
	for addrPeer := range g.peers.Peers {
		if addrPeer != excludedAddrs {
			addrs = append(addrs, addrPeer)
		}
	}
	g.peers.Mutex.Unlock()
	return
}

func (g Gossiper) getRandomPeer(excludedAddrs string) *Peer {
	availableAddrs := g.excludeAddr(excludedAddrs)
	//fmt.Println(availableAddrs)

	if len(availableAddrs) == 0 {
		return nil
	}

	i := rand.Intn(len(availableAddrs))
	g.peers.Mutex.Lock()
	peer := g.peers.Peers[availableAddrs[i]]
	g.peers.Mutex.Unlock()
	return &peer
}

// AddPeer -- Add a new peer to the list of peers. If Peer is already known: do nothing
func (g *Gossiper) AddPeer(address net.UDPAddr) {
	g.peers.Mutex.Lock()
	IPAddress := address.String()
	_, okAddr := g.peers.Peers[IPAddress]
	if !okAddr {
		g.peers.Peers[IPAddress] = newPeer(address)
	}
	g.peers.Mutex.Unlock()
}

//send a message to all known peers excepted Peer
func (g Gossiper) sendRumorToAllPeers(mess Messages.RumorMessage, excludeAddr *net.UDPAddr) {
	g.peers.Mutex.Lock()
	for _, p := range g.peers.Peers {
		if p.addr.String() != excludeAddr.String() {
			mess.Send(g.gossipConn, p.addr)
		}
	}
	g.peers.Mutex.Unlock()
}

func (g *Gossiper) listenConn(conn *net.UDPConn, isFromClient bool) {
	var bufferMess []byte
	var nbBytes int
	var err error
	var addr *net.UDPAddr
	for {
		bufferMess = make([]byte, 8*2048)
		nbBytes, addr, err = conn.ReadFromUDP(bufferMess)
		if err == nil {
			go g.accept(bufferMess[0:nbBytes], addr, isFromClient)
		}
	}
}

// Run -- Launch the server
func (g *Gossiper) Run() {
	go g.listenConn(g.UIConn, true)
	go g.listenConn(g.gossipConn, false)
	go g.antiEntropy()
	g.sendRouteRumor()
	g.routeRumorDeamon()
}

func (g *Gossiper) accept(buffer []byte, addr *net.UDPAddr, isFromClient bool) {
	mess := Messages.GossipMessage{}
	err := protobuf.Decode(buffer, &mess)
	if err != nil {
		log.Println("Protobuf:", err)
	}

	if mess.Rumor != nil {
		g.AcceptRumorMessage(*mess.Rumor, *addr, isFromClient)
	} else if mess.PrivateMessage != nil {
		g.AcceptPrivateMessage(*mess.PrivateMessage, *addr, isFromClient)
	} else if mess.Status != nil {
		g.acceptStatusMessage(*mess.Status, addr)
	} else if mess.ShareFile != nil && isFromClient {
		g.AcceptShareFile(*mess.ShareFile)
	} else if mess.DataReply != nil {
		g.AcceptDataReply(*mess.DataReply)
	} else if mess.DataRequest != nil {
		g.AcceptDataRequest(*mess.DataRequest)
	} else if mess.Download != nil {
		g.AcceptDownload(*mess.Download)
	} else if mess.SearchReply != nil {
		g.AcceptSearchReply(*mess.SearchReply)
	} else if mess.SearchRequest != nil {
		g.AcceptSearchRequest(*mess.SearchRequest, isFromClient)
	} else {
		log.Println("Gossip message received but unknown")
	}
}

func (g *Gossiper) AcceptSearchReply(mess Messages.SearchReply) {
	if len(mess.Results) <= 0 {
		log.Println("Search reply with no result received")
		return
	}

	g.PendingRequests.Mutex.Lock()
	for iFile := range mess.Results {

		fmt.Println("FOUND match", mess.Results[iFile].FileName, "at", mess.Origin, "budget=2")
		fmt.Printf("Metafile=%x chunks=%v\n", mess.Results[iFile].MetafileHash, mess.Results[iFile].ChunkMap)

		g.currentDownloads.Mutex.Lock()
		metaData, hasMetaFile := g.currentDownloads.Files[hashToString(mess.Results[iFile].MetafileHash)]
		g.currentDownloads.Mutex.Unlock()

		hash := hashToString(mess.Results[iFile].MetafileHash)
		if request, ok := g.PendingRequests.requests[hash]; ok {
			g.PendingRequests.requests[hash].chunkMap = mess.Results[iFile].ChunkMap
			if request.nbChunk == 0 && !hasMetaFile {
				g.askMetaFile(mess.Origin, mess.Results[iFile].FileName, mess.Results[iFile].MetafileHash)
			} else if request.nbChunk == 0 && hasMetaFile {
				request.nbChunk = uint64(metaData.size / 8000)
			} else if request.isClosed() {
				g.PendingRequests.nbMatch += 1
				if g.PendingRequests.nbMatch == 2 {
					fmt.Println("SEARCH FINISHED")
					g.searchState = false
				}
			}
		} else {
			g.PendingRequests.requests[hash] = &Request{mess.Results[iFile].FileName, mess.Results[iFile].ChunkMap, 0}
			g.askMetaFile(mess.Origin, mess.Results[iFile].FileName, mess.Results[iFile].MetafileHash)
		}
	}
	g.PendingRequests.Mutex.Unlock()
}

func (g Gossiper) askMetaFile(dest, filename string, hash []byte) {
	metafile, err := os.Create(g.workingPath + filename + "_meta")
	if err != nil {
		log.Println("askMetaFile:", err)
		return
	}
	metafile.Close()

	rq := &Messages.DataRequest{g.name, dest, 10, filename, hash}
	g.forward(rq)
}

func (g *Gossiper) AcceptSearchRequest(mess Messages.SearchRequest, isFromClient bool) {
	files := make(map[string]MetaData)
	downloadState := make(map[string]*state)

	g.FileShared.Mutex.Lock()
	for k, v := range g.FileShared.Files {
		files[k] = v
	}
	g.FileShared.Mutex.Unlock()

	g.downloadState.Mutex.Lock()
	for k, v := range g.downloadState.states {
		downloadState[k] = v
	}
	g.downloadState.Mutex.Unlock()

	searchResults := searchFile(mess.Keywords, files, downloadState)

	if isFromClient {
		if g.searchState == true {
			log.Println("A research is already running")
			return
		}
		g.searchState = true
		if mess.Budget == 1 {
			mess.Budget = 3
		}
		mess.Origin = g.name
	}

	if len(searchResults) != 0 {
		g.sendSearchReply(searchResults, mess.Origin)
	} else if mess.Budget > 1 && isFromClient {
		tick := time.NewTicker(time.Second)
		for g.searchState {
			<-tick.C
			mess.Budget *= 2
			g.propagateSearchFile(mess)
		}
	} else if mess.Budget > 1 {
		g.propagateSearchFile(mess)
	} else {
		fmt.Println("Search Request DELETED")
	}
}

func (g *Gossiper) propagateSearchFile(mess Messages.SearchRequest) {
	fmt.Println("Propagate search file")
	nbPeers := uint64(g.peers.len())

	// The budgets send to peers depend on mess.Budget.
	var newBudget uint64 = 1
	var remain uint64 = 0
	if mess.Budget-1 > nbPeers {
		newBudget = (mess.Budget - 1) / nbPeers
		remain = (mess.Budget - 1) % nbPeers
	}
	newRequest := Messages.SearchRequest{mess.Origin, newBudget, mess.Keywords}

	var iPeer uint64 = 0
	g.peers.Mutex.Lock()
	for _, peer := range g.peers.Peers {
		iPeer += 1
		if iPeer < remain {
			newRequest.Budget += 1
		}
		newRequest.Send(g.gossipConn, peer.addr)
	}
	g.peers.Mutex.Unlock()
}

func (g *Gossiper) sendSearchReply(searchResult []*Messages.SearchResult, dest string) {
	searchReply := Messages.SearchReply{g.name, dest, 10, searchResult}
	g.forward(&searchReply)
}

func (g *Gossiper) AcceptDownload(mess Messages.DownloadFile) {
	fmt.Println("DOWNLOADING metafile of", mess.FileName, "from", mess.Destination)

	meta := MetaData{mess.FileName, 0, g.workingPath + mess.FileName + "_meta", true, 0}

	request := Messages.DataRequest{g.name, mess.Destination, 10, mess.FileName, mess.HashValue}

	file, err := os.Create(g.workingPath + mess.FileName + "_meta")
	if err != nil {
		log.Println(err)
		return
	}
	file.Close()

	g.currentDownloads.Mutex.Lock()
	g.currentDownloads.Files[hashToString(mess.HashValue)] = meta
	g.currentDownloads.Mutex.Unlock()

	g.forward(&request)
}

func (g *Gossiper) AcceptDataReply(mess Messages.DataReply) {
	if mess.HopLimit > 1 && mess.Destination != g.name {
		g.forward(&mess)
	} else if g.name == mess.Destination {
		g.receiveFileData(mess.HashValue, mess.Data, mess.Origin)
	} else {
		log.Println("DATAREPLY from", mess.Origin, "to", mess.Destination, "hoplimit", mess.HopLimit, "DELETED")
	}
}

func (g *Gossiper) AcceptDataRequest(mess Messages.DataRequest) {
	fmt.Println("Data request: ", hashToString(mess.HashValue))
	g.FileShared.Mutex.Lock()
	metaData, ok := g.FileShared.Files[hashToString(mess.HashValue)]
	g.FileShared.Mutex.Unlock()

	if ok {
		file, err := os.Open(metaData.path)
		fmt.Println("Opening", metaData.path, "of size", getSizeFile(file))
		if err != nil {
			log.Println("AcceptDataRequest:", err)
		}
		defer file.Close()

		reply := Messages.DataReply{g.name, mess.Origin, 10, mess.FileName, mess.HashValue, nil}

		if metaData.isMetaFile {
			io.Copy(&reply, file)
		} else {
			file.Seek(metaData.offset, 0)
			io.CopyN(&reply, file, 8000)
		}

		if err != nil {
			log.Println(err)
			return
		}
		g.forward(&reply)
	} else {
		log.Println("File not found in shared files")
	}
}

func (g *Gossiper) receiveFileData(hash, data []byte, origin string) {
	strHash := hashToString(hash)
	g.currentDownloads.Mutex.Lock()
	metaData, download := g.currentDownloads.Files[strHash]
	g.currentDownloads.Mutex.Unlock()

	g.PendingRequests.Mutex.Lock()
	request, searching := g.PendingRequests.requests[hashToString(hash)]
	g.PendingRequests.Mutex.Unlock()

	var fileName string

	if download || searching {
		var file *os.File
		var err error
		if download {
			fileName = getNameFile(metaData.path)
			file, err = os.OpenFile(metaData.path, os.O_RDWR, 0333)
			if err != nil {
				log.Println("receiveFileData:", err)
				return
			}
		} else {
			fileName = request.fileName
			file, err = os.OpenFile(g.workingPath+fileName+"_meta", os.O_RDWR, 0333)
			if err != nil {
				log.Println("receiveFileData:", err)
				return
			}
		}

		if metaData.isMetaFile || searching {
			_, err = file.Write(data)
			file.Close()

			fmt.Println("METAFILE RECEIVED")

			chunks := ListFiles{make(map[string]MetaData), &sync.RWMutex{}}

			var nbChunk int
			if download {
				nbChunk, _ = analyseMetaFile(strHash, g.workingPath+metaData.Name, metaData.path, &chunks)

				fmt.Println("Nb chunks: ", nbChunk)
				g.downloadState.Mutex.Lock()
				g.downloadState.states[strHash] = initState(nbChunk)
				g.downloadState.Mutex.Unlock()

				tempFile, _ := os.Create(g.workingPath + metaData.Name)
				tempFile.Write(make([]byte, nbChunk*8000))
				tempFile.Close()

				g.sendRequests(chunks.Files, origin)
			} else if searching {
				nbChunk, _ := analyseMetaFile(strHash, "", g.workingPath+fileName+"_meta", &chunks)

				fmt.Println("Nb chunks: ", nbChunk)
				request.nbChunk = uint64(nbChunk)
			}
		} else {
			nb, _ := file.WriteAt(data, metaData.offset)

			chunkNumber := metaData.offset / 8000
			fmt.Println("RECEIVED", fileName, "chunk", chunkNumber)

			var remain int64

			g.downloadState.Mutex.Lock()
			isLastChunk := g.downloadState.chunkReceived(metaData.Name, int(chunkNumber))
			if nb < g.downloadState.states[metaData.Name].min {
				g.downloadState.states[metaData.Name].min = nb
			}
			if isLastChunk {
				remain = 8000 - int64(g.downloadState.states[metaData.Name].min)
				delete(g.downloadState.states, metaData.Name)
			}
			g.downloadState.Mutex.Unlock()

			if isLastChunk {
				fileComplete, err := os.Create(g.workingPath + "_Downloads/" + fileName)
				if err != nil {
					log.Println(err)
				}

				_, err = io.CopyN(fileComplete, file, getSizeFile(file)-remain)
				if err != nil {
					log.Println(err)
				}

				fmt.Println("RECONSTRUCTED file", fileName)
			}
			file.Close()
		}
		if err != nil {
			log.Println("receiveFileData:", err)
		}

		g.currentDownloads.Mutex.Lock()
		delete(g.currentDownloads.Files, strHash)
		g.currentDownloads.Mutex.Unlock()

		g.FileShared.Mutex.Lock()
		g.FileShared.Files[strHash] = metaData
		g.FileShared.Mutex.Unlock()

	}
}

func (g *Gossiper) sendRequests(chunks map[string]MetaData, dest string) {
	var request Messages.DataRequest
	tick := time.NewTicker(time.Duration(100) * time.Microsecond)
	for chunkHash, chunkMeta := range chunks {
		chunkHashByte, _ := hex.DecodeString(chunkHash)
		request = Messages.DataRequest{g.name, dest, 10, chunkMeta.Name, chunkHashByte}
		fmt.Println("DOWNLOADING", chunkMeta.Name, "chunk", chunkMeta.offset/8000, "from", dest)

		g.currentDownloads.Mutex.Lock()
		g.currentDownloads.Files[chunkHash] = chunkMeta
		g.currentDownloads.Mutex.Unlock()
		<-tick.C
		g.forward(&request)
	}
}

// Scan and store metadata when a file is shared
func (g *Gossiper) AcceptShareFile(mess Messages.ShareFile) {
	path := g.workingPath + mess.Path
	fileSize, metaHash, err := scanFile(path)
	if err != nil {
		fmt.Println(err)
		return
	}

	metaData := MetaData{mess.Path, fileSize, g.workingPath + mess.Path + "_meta", true, 0}

	g.FileShared.Mutex.Lock()
	g.FileShared.Files[hashToString(metaHash)] = metaData
	g.FileShared.Mutex.Unlock()

	analyseMetaFile(mess.Path, g.workingPath+mess.Path, metaData.path, &g.FileShared)
	fmt.Println("FILE", mess.Path, "of", fileSize, "Bytes", "is now shared: ", hashToString(metaHash))
}

func (g *Gossiper) AcceptPrivateMessage(mess Messages.PrivateMessage, addr net.UDPAddr, isFromClient bool) {
	if !isFromClient && g.alreadySeen(mess.ID, mess.Origin) {
		return
	}
	fmt.Println("PRIVATE RECEIVED origin", mess.Origin, "dest", mess.GetDest(), "HopLimit", mess.GetHopLimit())

	if isFromClient {
		mess.Origin = g.name
		mess.HopLimit = 10
		fmt.Println("CLIENT PRIVATE", mess, g.name)
	} else {
		g.mutex.Lock()
		fmt.Println("DSDV", mess.Origin+":"+addr.String())
		g.RoutingTable.add(mess.Origin, addr.String())
		g.mutex.Unlock()
	}

	if mess.GetHopLimit() > 1 && mess.GetDest() != g.name {
		g.forward(&mess)
	} else if mess.GetDest() == g.name {
		g.receivePrivateMessage(mess)
	} else {
		fmt.Println("Private Message", mess, "DELETED")
	}

}

//Callback function, call when a message is received
func (g *Gossiper) AcceptRumorMessage(mess Messages.RumorMessage, addr net.UDPAddr, isFromClient bool) {

	if !isFromClient && g.alreadySeen(mess.ID, mess.Origin) {
		return
	}

	if isFromClient {
		mess.Origin = g.name
		g.mutex.Lock()
		mess.ID = g.idMessage
		g.idMessage++
		g.mutex.Unlock()
	} else {
		g.mutex.Lock()
		fmt.Println("DSDV", mess.Origin+":"+addr.String())
		g.RoutingTable.add(mess.Origin, addr.String())
		g.mutex.Unlock()
	}

	if mess.Text != "" {
		g.printDebugRumor(mess, addr.String(), isFromClient)
	}

	g.updateVectorClock(mess.Origin, mess.ID)
	g.storeRumorMessage(mess, mess.ID, mess.Origin)

	if !isFromClient {
		statusMessage := Messages.StatusMessage{g.vectorClock}
		statusMessage.Send(g.gossipConn, addr)
		g.AddPeer(addr)
	}
	g.propagateRumorMessage(mess, addr.String())
}

func (g *Gossiper) receivePrivateMessage(message Messages.PrivateMessage) {
	fmt.Println("PRIVATE:", message.Origin+":"+fmt.Sprint(message.HopLimit)+":"+message.Text)
	g.PrivateMessages = append(g.PrivateMessages, message)
}

func (g Gossiper) forward(message Messages.Private) {
	message.DecHopLimit()
	addr := g.RoutingTable.FindNextHop(message.GetDest())
	if addr != "" {
		UDPAddr, err := net.ResolveUDPAddr("udp4", addr)
		if err == nil {
			fmt.Println("FORWARD private msg", message.GetDest(), addr)
			message.Send(g.gossipConn, *UDPAddr)
		}
	} else {
		fmt.Println("Forward: Private Message", message, "DELETED")
	}

}

func (g Gossiper) propagateRumorMessage(mess Messages.RumorMessage, excludedAddrs string) {
	coin := 1
	peer := g.getRandomPeer(excludedAddrs)

	for coin == 1 && peer != nil {

		fmt.Println("MONGERING with", peer.addr.String())
		mess.Send(g.gossipConn, peer.addr)

		peer = g.getRandomPeer("")
		coin = rand.Int() % 2
		//fmt.Println(coin, peer)
		if coin == 1 && peer != nil {
			fmt.Println("FLIPPED COIN sending rumor to", peer.addr.String())
		}
	}
}

func (g *Gossiper) acceptStatusMessage(mess Messages.StatusMessage, addr *net.UDPAddr) {
	g.AddPeer(*addr)
	g.printDebugStatus(mess, *addr)
	isMessageToAsk, node, id := g.compareVectorClocks(mess.Want)
	g.MessagesReceived.Mutex.Lock()
	messToSend := g.MessagesReceived.Messages[node][id]
	g.MessagesReceived.Mutex.Unlock()
	if node != "" {
		fmt.Println("MONGERING with", addr.String())
		messToSend.Send(g.gossipConn, *addr)
	}
	if isMessageToAsk {
		statusMessage := Messages.StatusMessage{g.vectorClock}
		statusMessage.Send(g.gossipConn, *addr)
	}
	if !isMessageToAsk && node == "" {
		fmt.Println("IN SYNC WITH", addr.String())
		g.exchangeEnded <- true
	}
}

func (g Gossiper) printPeerList() {
	first := true
	g.peers.Mutex.Lock()
	for _, peer := range g.peers.Peers {
		if first {
			first = false
			fmt.Print(peer)
		} else {
			fmt.Print(",", peer)
		}
	}
	g.peers.Mutex.Unlock()
	fmt.Println()
}

func (g Gossiper) printDebugStatus(mess Messages.StatusMessage, addr net.UDPAddr) {
	fmt.Print("STATUS from ", addr.String())
	for _, peerStatus := range mess.Want {
		fmt.Print(" origin ", peerStatus.Identifier, " nextID ", peerStatus.NextID)
	}
	fmt.Println()
	g.printPeerList()
}

func (g Gossiper) printDebugRumor(mess Messages.RumorMessage, lastHopIP string, isFromClient bool) {
	if isFromClient {
		fmt.Println("CLIENT", mess, g.name)
	} else {
		fmt.Println("RUMOR", "origin", mess.Origin, "from", lastHopIP, "ID", mess.ID, "contents", mess.Text)
	}
	g.printPeerList()
}

func (g Gossiper) alreadySeen(id uint32, nodeName string) bool {
	g.MessagesReceived.Mutex.Lock()
	_, ok := g.MessagesReceived.Messages[nodeName][id]
	g.MessagesReceived.Mutex.Unlock()
	return ok
}

func (g *Gossiper) updateVectorClock(name string, id uint32) {
	find := false
	for i := range g.vectorClock {
		if g.vectorClock[i].Identifier == name {
			find = true
			if g.vectorClock[i].NextID == id {
				g.mutex.Lock()
				g.vectorClock[i].NextID++
				g.mutex.Unlock()
				g.checkOnAlreadySeen(g.vectorClock[i].NextID, name)
			}
		}
	}
	if !find && id == 1 {
		g.mutex.Lock()
		g.vectorClock = append(g.vectorClock, Messages.PeerStatus{Identifier: name, NextID: 2})
		g.mutex.Unlock()
		g.checkOnAlreadySeen(2, name)
	}
}

func (g Gossiper) checkOnAlreadySeen(nextID uint32, nodeName string) {
	if g.alreadySeen(nextID, nodeName) {
		g.updateVectorClock(nodeName, nextID)
	}
}

func (g Gossiper) storeRumorMessage(mess Messages.RumorMessage, id uint32, nodeName string) {
	g.MessagesReceived.Mutex.Lock()
	if g.MessagesReceived.Messages[nodeName] == nil {
		g.MessagesReceived.Messages[nodeName] = make(map[uint32]Messages.RumorMessage)
	}
	g.MessagesReceived.Messages[nodeName][id] = mess
	g.MessagesReceived.Mutex.Unlock()
}

func (g *Gossiper) antiEntropy() {
	tick := time.NewTicker(time.Minute)
	for {
		<-tick.C
		peer := g.getRandomPeer("")
		if peer != nil {
			statusMessage := Messages.StatusMessage{g.vectorClock}
			statusMessage.Send(g.gossipConn, peer.addr)
		}
	}
}

func (g Gossiper) compareVectorClocks(vectorClock []Messages.PeerStatus) (isMessageToAsk bool, node string, id uint32) {
	isMessageToAsk = false
	find := true
	node = ""
	id = 0
	for _, ps := range g.vectorClock {
		find = false
		for _, wanted := range vectorClock {
			if ps.Identifier == wanted.Identifier {
				find = true
				if ps.NextID < wanted.NextID {
					isMessageToAsk = true
					if node != "" {
						return
					}
				} else if ps.NextID > wanted.NextID {
					node, id = ps.Identifier, wanted.NextID
					if isMessageToAsk {
						return
					}
				}
			}
		}
		if !find {
			node, id = ps.Identifier, 1
			if isMessageToAsk {
				return
			}
		}
	}
	for _, wanted := range vectorClock {
		find = false
		for _, ps := range g.vectorClock {
			if ps.Identifier == wanted.Identifier {
				find = true
			}
		}
		if !find {
			isMessageToAsk = true
			return
		}
	}
	return
}

func genRouteRumor() (Messages.RumorMessage) {
	mess := Messages.RumorMessage{
		Text: "",
	}
	return mess
}

func (g *Gossiper) routeRumorDeamon() {
	tick := time.NewTicker(time.Duration(g.rtimer) * time.Second)
	for {
		<-tick.C
		g.sendRouteRumor()
	}
}

func (g *Gossiper) sendRouteRumor() {
	g.AcceptRumorMessage(genRouteRumor(), *g.gossipAddr, true)
}

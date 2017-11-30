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
	UIAddr           *(net.UDPAddr)
	gossipAddr       *(net.UDPAddr)
	UIConn           *(net.UDPConn)
	gossipConn       *(net.UDPConn)
	name             string
	peers            map[string]Peer
	vectorClock      []Messages.PeerStatus
	idMessage        uint32
	MessagesReceived map[string](map[uint32]Messages.RumorMessage)
	exchangeEnded    chan bool
	RoutingTable     RoutingTable
	mutex            *sync.Mutex
	rtimer           uint
	PrivateMessages  []Messages.PrivateMessage
	FileShared       map[string]MetaData
	currentDownloads map[string]MetaData
	workingPath      string
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
		peers:            make(map[string]Peer, 0),
		vectorClock:      make([]Messages.PeerStatus, 0),
		idMessage:        1,
		MessagesReceived: make(map[string](map[uint32]Messages.RumorMessage), 0),
		exchangeEnded:    make(chan bool),
		RoutingTable:     *newRoutingTable(),
		mutex:            &sync.Mutex{},
		rtimer:           rtimer,
		FileShared:       make(map[string]MetaData),
		currentDownloads: make(map[string]MetaData),
		workingPath:      workingPath,
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
	for addrPeer := range g.peers {
		if addrPeer != excludedAddrs {
			addrs = append(addrs, addrPeer)
		}
	}
	return
}

func (g Gossiper) getRandomPeer(excludedAddrs string) *Peer {
	availableAddrs := g.excludeAddr(excludedAddrs)
	//fmt.Println(availableAddrs)

	if len(availableAddrs) == 0 {
		return nil
	}

	i := rand.Intn(len(availableAddrs))

	peer := g.peers[availableAddrs[i]]
	return &peer
}

// AddPeer -- Add a new peer to the list of peers. If Peer is already known: do nothing
func (g *Gossiper) AddPeer(address net.UDPAddr) {
	IPAddress := address.String()
	_, okAddr := g.peers[IPAddress]
	if !okAddr {
		g.mutex.Lock()
		g.peers[IPAddress] = newPeer(address)
		g.mutex.Unlock()
	}
}

//send a message to all known peers excepted Peer
func (g Gossiper) sendRumorToAllPeers(mess Messages.RumorMessage, excludeAddr *net.UDPAddr) {
	for _, p := range g.peers {
		if p.addr.String() != excludeAddr.String() {
			mess.Send(g.gossipConn, p.addr)
		}
	}
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
		fmt.Println("Protobuf:", err)
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
	} else {
		fmt.Println("Gossip message received but unknown")
	}
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
	g.currentDownloads[hashToString(mess.HashValue)] = meta
	g.forward(&request)
}

func (g *Gossiper) AcceptDataReply(mess Messages.DataReply) {
	if mess.HopLimit > 1 && mess.Destination != g.name {
		g.forward(&mess)
	} else if g.name == mess.Destination {
		g.receiveFileData(mess.HashValue, mess.Data, mess.Origin)
	} else {
		log.Println("DATAREPLY DELETED")
	}
}

func (g *Gossiper) AcceptDataRequest(mess Messages.DataRequest) {
	fmt.Println("Data request: ", hashToString(mess.HashValue))
	metaData, ok := g.FileShared[hashToString(mess.HashValue)]

	if ok {
		file, err := os.Open(metaData.path)
		fmt.Println("Opening", metaData.path, "of size", getSizeFile(file))
		if err != nil {
			log.Println("AcceptDataRequest:", err)
		}
		defer file.Close()

		reply := Messages.DataReply{g.name, mess.Origin, 10, mess.FileName, mess.HashValue, nil}

		if metaData.isMetaFile {
			nb, _ := io.Copy(&reply, file)
			fmt.Println("Nb write", nb, "/", getSizeFile(file), "path", metaData.path)
		} else {
			file.Seek(metaData.offset, 0)
			nb, _ := io.CopyN(&reply, file, 8000)
			fmt.Println("Nb write", nb, "/", getSizeFile(file), "path", metaData.path)
		}

		if err != nil {
			log.Println(err)
			return
		}
		g.forward(&reply)
	} else {
		log.Println("File not found")
	}
}

func (g *Gossiper) receiveFileData(hash, data []byte, origin string) {
	strHash := hashToString(hash)
	metaData, ok := g.currentDownloads[strHash]

	if ok {
		file, err := os.OpenFile(metaData.path, os.O_RDWR, 0333)
		if err != nil {
			log.Println("receiveFileData:", err)
			return
		}

		if metaData.isMetaFile {
			_, err = file.Write(data)
			file.Close()

			fmt.Println("METAFILE RECEIVED")

			chunks := make(map[string]MetaData)
			nbChunk, _ := analyseMetaFile(metaData.Name, g.workingPath + metaData.Name, metaData.path, chunks)
			g.sendRequests(chunks, origin)

			tempFile, _ := os.Create(g.workingPath + metaData.Name)
			tempFile.Write(make([]byte, nbChunk*8000))
			tempFile.Close()
		} else {
			nb, _ := file.WriteAt(data, metaData.offset)

			fmt.Println("RECEIVED", metaData.Name, "chunk", metaData.offset/8000)

			if nb < 8000 {
				var remaim int64 = 8000 - int64(nb)
				fileComplete, err := os.Create(g.workingPath + "_Downloads/" + metaData.Name)
				if err != nil {
					log.Println(err)
				}

				_, err = io.CopyN(fileComplete, file, getSizeFile(file)-remaim)
				if err != nil {
					log.Println(err)
				}

				fmt.Println("RECONSTRUCTED file", file.Name())
			}
			file.Close()
		}
		if err != nil {
			log.Println("receiveFileData:", err)
		}
		delete(g.currentDownloads, strHash)
		g.FileShared[strHash] = metaData

	}
}

func (g *Gossiper) sendRequests(chunks map[string]MetaData, dest string) {
	var request Messages.DataRequest
	for chunkHash, chunkMeta := range chunks {
		chunkHashByte, _ := hex.DecodeString(chunkHash)
		request = Messages.DataRequest{g.name, dest, 10, chunkMeta.Name, chunkHashByte}
		fmt.Println("DOWNLOADING", chunkMeta.Name, "chunk", chunkMeta.offset/8000, "from", dest)
		g.currentDownloads[chunkHash] = chunkMeta
		g.forward(&request)
	}
}

// Scan and store metadata when a file is shared
func (g *Gossiper) AcceptShareFile(mess Messages.ShareFile) {
	metaHash, err := scanFile(g.workingPath + mess.Path)
	if err != nil {
		fmt.Println(err)
		return
	}

	metaData := MetaData{mess.Path, 0, g.workingPath + mess.Path + "_meta", true, 0}
	g.FileShared[hashToString(metaHash)] = metaData
	analyseMetaFile(mess.Path, g.workingPath+mess.Path, metaData.path, g.FileShared)
	fmt.Println("FILE", mess.Path, "of", 0, "Bytes", "is now shared: ", hashToString(metaHash))
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
	messToSend := g.MessagesReceived[node][id]
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
	for _, peer := range g.peers {
		if first {
			first = false
			fmt.Print(peer)
		} else {
			fmt.Print(",", peer)
		}
	}
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
	g.mutex.Lock()
	_, ok := g.MessagesReceived[nodeName][id]
	g.mutex.Unlock()
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
	g.mutex.Lock()
	if g.MessagesReceived[nodeName] == nil {
		g.MessagesReceived[nodeName] = make(map[uint32]Messages.RumorMessage)
	}
	g.MessagesReceived[nodeName][id] = mess
	g.mutex.Unlock()
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

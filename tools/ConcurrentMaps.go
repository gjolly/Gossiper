package tools

import (
	"sync"
	"github.com/gjolly/Gossiper/tools/Messages"
)

type ListPeers struct {
	Peers map[string]Peer
	Mutex *sync.RWMutex
}

func (lp ListPeers) len() int {
	lp.Mutex.Lock()
	length := len(lp.Peers)
	lp.Mutex.Unlock()
	return length
}

type ListMessages struct {
	Messages map[string](map[uint32]Messages.RumorMessage)
	Mutex    *sync.RWMutex
}

type ListFiles struct {
	Files map[string]MetaData
	Mutex *sync.RWMutex
}

type ListStates struct {
	states map[string]*state
	Mutex  *sync.RWMutex
}

type state struct {
	chunkDL []bool
	min     int
	nbChunk int
}

type PendingRequests struct {
	requests map[string]*Request
	Mutex    *sync.RWMutex
	nbMatch  int
}

func (s state) chunkMap() (chunks []uint64) {
	chunks = make([]uint64, 0)
	for iState := range s.chunkDL {
		if s.chunkDL[iState] {
			chunks = append(chunks, uint64(iState))
		}
	}
	return
}

func (ls ListStates) chunkReceived(hash string, chunkNumber int) bool {
	ls.states[hash].chunkDL[chunkNumber] = true
	ls.states[hash].nbChunk += 1
	return ls.states[hash].nbChunk == len(ls.states[hash].chunkDL)
}

func initState(chunkNumber int) *state {
	return &state{make([]bool, chunkNumber), 8000, 0}
}

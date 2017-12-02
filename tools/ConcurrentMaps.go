package tools

import (
	"sync"
	"github.com/gjolly/Gossiper/tools/Messages"
)

type ListPeers struct {
	Peers map[string]Peer
	Mutex *sync.RWMutex
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
	dl    map[string]int
	min   map[string]int
	Mutex *sync.RWMutex
}

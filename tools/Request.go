package tools

type Request struct {
	fileName string
	chunkMap []uint64
	nbChunk  uint64
}

func (r Request) isClosed() bool {
	return len(r.chunkMap) == int(r.nbChunk)
}

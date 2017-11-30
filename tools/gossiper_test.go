package tools

import (
	"testing"
	"os"

	"github.com/gjolly/Gossiper/tools/Messages"
	"fmt"
)

var gossiper *Gossiper

func TestGossiper_AcceptShareFile(t *testing.T) {
	gossiper.AcceptShareFile(Messages.ShareFile{Path:"../tests/testFile.txt"})
	fmt.Println(gossiper.FileShared)
}

func TestGossiper_AcceptDataRequest(t *testing.T) {
	gossiper.AcceptDataRequest(Messages.DataRequest{})
}

func TestMain(m *testing.M){
	var err error
	gossiper, err = NewGossiper("5000", "localhost:10000", "A", make([]string, 0), 60)
	if err != nil {
		panic(err)
	}
	go gossiper.Run()
	os.Exit(m.Run())
}
package tools

import (
	"testing"
	"os"
	"fmt"
	"log"
)

func TestScanner_Read(t *testing.T) {
	file, err := os.Open("../tests/testFile.txt")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scan := scanner{file}
	buf := make([]byte, 32)
	nb, err := scan.Read(buf)

	for nb > 0 {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%x\n", buf)
		nb, err = scan.Read(buf)
	}
}

func TestScanFile(*testing.T) {
	file, err := os.Open("../tests/testFile.txt")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	metaHash, err := scanFile(file)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%x\n", metaHash)
}

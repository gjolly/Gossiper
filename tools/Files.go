package tools

import (
	"os"
	"fmt"
	"io"
	"crypto/sha256"
)

type MetaData struct {
	Name         string
	size         int64
	pathMetaFile string
	metaHash     []byte
}

func (m MetaData) String() (string){
	return fmt.Sprintf("%s\t%d Bytes\t%s\t%x", m.Name, m.size, m.pathMetaFile, m.metaHash)
}

// Reader to compute the hash of each 8kb of file
type scanner struct {
	file io.Reader
}

// Compute and return a 8kB chunk of file
func (s scanner) Read(bufOut []byte) (n int, err error) {
	err = nil
	if len(bufOut) < 32 {
		return 0, BufferTooSmall{}
	}
	buf := make([]byte, 8000)
	h := sha256.New()

	nb, errCpy := io.CopyBuffer(h, s.file, buf)

	if errCpy != nil {
		return 0, err
	}

	copy(bufOut, h.Sum(nil))
	if nb == 0 {
		err = io.EOF
	}

	return int(nb), err
}

func metaHash(file *os.File) ([]byte) {
	h := sha256.New()
	io.Copy(h, file)
	return h.Sum(nil)
}

// Create the metafile of file and return the metahash of the metafile
func scanFile(file *os.File) ([]byte, error) {
	output, err := os.Create(file.Name() + "_meta")
	if err != nil {
		return nil, err
	}
	defer output.Close()

	scan := scanner{file}
	buf := make([]byte, 32)
	_, err = io.CopyBuffer(output, scan, buf)

	if err != nil && err != io.EOF {
		return nil, err
	}

	output.Seek(0, 0)
	return metaHash(output), nil
}

func getSizeFile(file *os.File) (int64) {
	fi, err := file.Stat()
	if err != nil {
		fmt.Println(err)
		return 0
	}
	return fi.Size()
}

package tools

import (
	"os"
	"fmt"
	"io"
	"crypto/sha256"
	"log"
	"strings"
)

type MetaData struct {
	Name       string
	size       int64
	path       string
	isMetaFile bool
	offset     int64
}

func (m MetaData) String() (string) {
	return fmt.Sprintf("%s\t%d Bytes\t%s\t%x", m.Name, m.size, m.path)
}

// Reader to compute the hash of each 8kb of file
type scanner struct {
	file io.Reader
}

// Compute and return a 8kB hashed chunk of file
func (s scanner) Read(bufOut []byte) (n int, err error) {
	err = nil
	if len(bufOut) < 32 {
		return 0, BufferTooSmall{}
	}
	buf := make([]byte, 8000)

	nb, errRead := s.file.Read(buf)
	if errRead != nil && errRead != io.EOF {
		return 0, err
	}

	h := sha256.Sum256(buf)
	copy(bufOut[0:32], h[0:32])
	if nb == 0 {
		return 0, io.EOF
	}

	return 32, nil
}

func metaHash(file *os.File) ([]byte) {
	h := sha256.New()
	io.Copy(h, file)
	return h.Sum(nil)
}

// Create the metafile of file and return the metahash of the metafile
func scanFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	output, err := os.Create(path + "_meta")
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

func analyseMetaFile(sharedFileName, sharedFilePath, metaFile string, files *ListFiles) (int, error) {
	file, err := os.Open(metaFile)
	if err != nil {
		log.Println("Error reading metafile")
		return 0, err
	}
	defer file.Close()

	var offset int64 = 0
	buf := make([]byte, 32)
	nb, err := file.Read(buf)
	for nb > 0 {
		if err != nil {
			return 0, err
		}

		files.Mutex.Lock()
		files.Files[hashToString(buf[0:nb])] = MetaData{sharedFileName, 0, sharedFilePath, false, offset}
		files.Mutex.Unlock()

		offset += 8000
		nb, err = file.Read(buf)
	}

	return int(offset / 8000), nil
}

func hashToString(hash []byte) string {
	return fmt.Sprintf("%x", hash)
}

func getNameFile(path string) (string) {
	elmts := strings.Split(path, "/")
	return elmts[len(elmts) - 1]
}

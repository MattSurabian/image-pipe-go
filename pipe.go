package main

import (
	"github.com/minio/minio-go"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
)

// SourceURI does stuff...
func SourceURI(URI string) func(io.WriteCloser) {
	return func(data io.WriteCloser) {
		response, err := http.Get(URI)
		if err != nil {
			log.Fatal(err)
		}
		defer response.Body.Close()
		defer data.Close()
		io.Copy(data, response.Body)
	}
}

// DestFile does stuff...
func DestFile(filePath string) func(io.ReadCloser) {
	return func(data io.ReadCloser) {
		file, err := os.Create(filePath)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		io.Copy(file, data)
	}
}

// DestObjectStorage does stuff...
func DestObjectStorage(accessKeyID string, secretAccessKeyID string, bucket string, key string) func(io.ReadCloser) {
	return func(data io.ReadCloser) {
		client, err := minio.New("s3.amazonaws.com", accessKeyID, secretAccessKeyID, true)
		if err != nil {
			log.Fatal(err)
		}
		result, err := client.PutObject(bucket, key, data, -1, minio.PutObjectOptions{})
		if err != nil {
			log.Fatal(err)
		}
		log.Println(result)
		defer data.Close()
	}
}

// Pipe does stuff...
func Pipe(cmd *exec.Cmd, reader func(io.WriteCloser), writer func(io.ReadCloser)) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		reader(stdin)
	}()
	go func() {
		defer wg.Done()
		writer(stdout)
	}()
	wg.Wait()
	err = cmd.Wait()
	if err != nil {
		log.Fatal(err)
	}
}

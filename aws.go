package main

import (
	"github.com/minio/minio-go"
)

func GetS3Client(accessKeyID string, secretAccessKeyID string) (*minio.Client, error){
	return minio.New("s3.amazonaws.com", accessKeyID, secretAccessKeyID, true)
}
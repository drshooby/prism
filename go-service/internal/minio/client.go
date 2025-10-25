package minio

import (
	"context"
	"fmt"
	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioClientConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
}

type MinioClient struct {
	client *minio.Client
}

type ListBucketsResponse struct {
	Buckets    []string `json:"buckets"`
	StatusCode int      `json:"statusCode"`
	Error      string   `json:"error,omitempty"`
}

func NewMinioClient(config *MinioClientConfig) (*MinioClient, error) {
	minioClient, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure: config.UseSSL,
	})
	if err != nil {
		log.Fatalln(err)
	}

	return &MinioClient{
		client: minioClient,
	}, nil
}

func (minioClient MinioClient) ListBuckets(context context.Context) *ListBucketsResponse {
	buckets, err := minioClient.client.ListBuckets(context)
	if err != nil {
		return &ListBucketsResponse{
			StatusCode: 500,
			Error:      fmt.Sprintf("Error listing buckets: %v", err),
		}
	}

	return &ListBucketsResponse{
		Buckets:    extractBucketNames(buckets),
		StatusCode: 200,
	}
}

func extractBucketNames(buckets []minio.BucketInfo) []string {
	bucketNames := make([]string, len(buckets))
	for i, bucket := range buckets {
		bucketNames[i] = bucket.Name
	}
	return bucketNames
}

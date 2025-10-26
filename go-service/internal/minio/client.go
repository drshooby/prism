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

func (minioClient *MinioClient) ListBuckets(ctx context.Context) *ListBucketsResponse {
	buckets, err := minioClient.client.ListBuckets(ctx)
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

func (minioClient *MinioClient) getBucket(ctx context.Context, bucketName string) (*minio.BucketInfo, error) {
	buckets, err := minioClient.client.ListBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing buckets: %v", err)
	}

	for _, bucket := range buckets {
		if bucket.Name == bucketName {
			return &bucket, nil
		}
	}

	return nil, fmt.Errorf("bucket %s not found", bucketName)
}

func (minioClient *MinioClient) createBucket(ctx context.Context, bucketName string) error {
	err := minioClient.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		return fmt.Errorf("error creating bucket %s: %v", bucketName, err)
	}
	return nil
}

func (minioClient *MinioClient) GetOrCreateBucket(ctx context.Context, bucketName string) (*minio.BucketInfo, error) {
	bucket, err := minioClient.getBucket(ctx, bucketName)
	if err == nil {
		return bucket, nil
	}

	if err := minioClient.createBucket(ctx, bucketName); err != nil {
		return nil, err
	}

	return minioClient.getBucket(ctx, bucketName)
}

func (minioClient *MinioClient) DownloadFileObject(ctx context.Context, bucketName, objectName, filePath string) error {
	if err := minioClient.client.FGetObject(ctx, bucketName, objectName, filePath, minio.GetObjectOptions{}); err != nil {
		return fmt.Errorf("error downloading object %s from bucket %s to file %s: %v", objectName, bucketName, filePath, err)
	}
	return nil
}

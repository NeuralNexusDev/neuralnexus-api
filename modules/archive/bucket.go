package archive

import (
	"context"
	"io"
	"log"

	"github.com/minio/minio-go/v7"
)

// S3Store interface for an S3 store
type S3Store interface {
	MakeBucket()
	UploadFile(objectName string, reader io.Reader, size int64) (*minio.UploadInfo, error)
}

// s3store implementation of S3Store
type s3store struct {
	minioClient *minio.Client
	bucketName  string
	location    string
}

// NewS3Store creates a new S3 store
func NewS3Store(minioClient *minio.Client) S3Store {
	return &s3store{minioClient, "minecraft-archive", "ca-central-1"}
}

// MakeBucket creates a new bucket in the S3 store
func (s *s3store) MakeBucket() {
	ctx := context.Background()
	err := s.minioClient.MakeBucket(ctx, s.bucketName, minio.MakeBucketOptions{Region: s.location})
	if err != nil {
		exists, err := s.minioClient.BucketExists(ctx, s.bucketName)
		if err == nil && exists {
			log.Printf("We already own %s\n", s.bucketName)
		} else {
			log.Fatal("Unable to create bucket:", err)
		}
	} else {
		log.Printf("Successfully created %s\n", s.bucketName)
	}
}

// UploadFile uploads a file to the S3 store
func (s *s3store) UploadFile(objectName string, reader io.Reader, size int64) (*minio.UploadInfo, error) {
	info, err := s.minioClient.PutObject(context.Background(), s.bucketName, objectName, reader, size,
		minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return nil, err
	}
	return &info, nil
}

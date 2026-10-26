package archive

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Store interface for an S3 store
type S3Store interface {
	MakeBucket()
	UploadFile(ctx context.Context, objectName string, reader io.Reader, size int64) (*any, error)
}

// s3store implementation of S3Store
type s3store struct {
	c          *s3.Client
	bucketName string
	location   string
}

// NewS3Store creates a new S3 store
func NewS3Store(client *s3.Client) S3Store {
	return &s3store{client, "minecraft-archive", "ca-central-1"}
}

// MakeBucket creates a new bucket in the S3 store
func (s *s3store) MakeBucket() {
	//ctx := context.Background()
	//err := s.s3.MakeBucket(ctx, s.bucketName, minio.MakeBucketOptions{Region: s.location})
	//if err != nil {
	//	exists, err := s.s3.BucketExists(ctx, s.bucketName)
	//	if err == nil && exists {
	//		log.Printf("We already own %s\n", s.bucketName)
	//	} else {
	//		log.Fatal("Unable to create bucket:", err)
	//	}
	//} else {
	//	log.Printf("Successfully created %s\n", s.bucketName)
	//}
}

// UploadFile uploads a file to the S3 store
func (s *s3store) UploadFile(ctx context.Context, objectName string, reader io.Reader, size int64) (*any, error) {
	//info, err := s.c.PutObject(ctx, s.bucketName, objectName, reader, size,
	//	minio.PutObjectOptions{ContentType: "application/octet-stream"})
	//if err != nil {
	//	return nil, err
	//}
	//return &info, nil
	return nil, nil
}

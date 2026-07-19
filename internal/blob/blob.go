// Package blob stores and retrieves issue attachments in an S3-compatible
// object store (MinIO locally, any S3 provider in production).
package blob

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ErrNotFound is returned by Get when the requested key does not exist.
var ErrNotFound = errors.New("blob: object not found")

// Store wraps a MinIO/S3 client bound to a single bucket.
type Store struct {
	client *minio.Client
	bucket string
}

// New connects to the S3-compatible endpoint and ensures bucket exists,
// creating it if necessary.
func New(ctx context.Context, endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Store, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket %q exists: %w", bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create bucket %q: %w", bucket, err)
		}
	}

	return &Store{client: client, bucket: bucket}, nil
}

// Put uploads r under key. If size is unknown, pass -1 and minio streams the
// upload using multipart.
func (s *Store) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("put object %q: %w", key, err)
	}
	return nil
}

// Get returns a reader for the object stored under key. It stats the object
// up front so a missing key surfaces as ErrNotFound rather than failing on
// first read.
func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object %q: %w", key, err)
	}

	if _, err := obj.Stat(); err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			_ = obj.Close()
			return nil, ErrNotFound
		}
		_ = obj.Close()
		return nil, fmt.Errorf("stat object %q: %w", key, err)
	}

	return obj, nil
}

package storage

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOStore struct {
	client  *minio.Client
	bucket  string
	cdnBase string
}

func NewMinIOStore(endpoint string, accessKey string, secretKey string, bucket string, useSSL bool, cdnBase string) (*MinIOStore, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}
	return &MinIOStore{
		client:  client,
		bucket:  bucket,
		cdnBase: strings.TrimRight(cdnBase, "/"),
	}, nil
}

func (s *MinIOStore) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return err
		}
	}
	return s.client.SetBucketPolicy(ctx, s.bucket, fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {"AWS": ["*"]},
      "Action": ["s3:GetObject"],
      "Resource": ["arn:aws:s3:::%s/generated/*"]
    }
  ]
}`, s.bucket))
}

func (s *MinIOStore) WriteResult(ctx context.Context, taskID string, payload []byte) (string, error) {
	key := fmt.Sprintf("generated/%s.json", taskID)
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(payload), int64(len(payload)), minio.PutObjectOptions{
		ContentType: "application/json",
	})
	if err != nil {
		return "", err
	}
	return s.cdnURL(key), nil
}

func (s *MinIOStore) cdnURL(key string) string {
	escaped := (&url.URL{Path: key}).EscapedPath()
	return s.cdnBase + "/" + strings.TrimPrefix(escaped, "/")
}

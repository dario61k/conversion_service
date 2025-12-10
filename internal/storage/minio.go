package storage

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3 struct {
	client *minio.Client
}

func New(endpoint, ak, sk string, ssl bool) (*S3, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(ak, sk, ""),
		Secure:       false,
		Region:       "us-east-1",            // p.ej. "us-east-1"
		BucketLookup: minio.BucketLookupAuto, //  ←  fuerza path-style
		Transport: &http.Transport{
			ResponseHeaderTimeout: 15 * time.Second,
		},
	})
	if err != nil {
		return nil, err
	}
	return &S3{client: mc}, nil
}

func (s *S3) Exists(ctx context.Context, bucket, object string) bool {
	_, err := s.client.StatObject(ctx, bucket, object, minio.StatObjectOptions{})
	return err == nil
}

func (s *S3) Presign(ctx context.Context, bucket, object string, dur time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, bucket, object, dur, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *S3) List(ctx context.Context, bucket, prefix string) <-chan minio.ObjectInfo {
	return s.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
}

func (s *S3) Get(ctx context.Context, bucket, object, path string) error {
	return s.client.FGetObject(ctx, bucket, object, path, minio.GetObjectOptions{})
}

func (s *S3) Put(ctx context.Context, bucket, object, path, contentType string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = s.client.PutObject(ctx, bucket, object, f, info.Size(), minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *S3) Remove(ctx context.Context, bucket, object string) error {
	return s.client.RemoveObject(ctx, bucket, object, minio.RemoveObjectOptions{})
}

type BucketsInfo struct {
	Name         string
	CreationDate time.Time
}

func (s *S3) ListBuckets(ctx context.Context) ([]BucketsInfo, error) {
	buckets, err := s.client.ListBuckets(context.Background())
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	var bucketsList []BucketsInfo

	for _, bucket := range buckets {
		if bucket.Name != "" {
			bucketsList = append(bucketsList, BucketsInfo{
				Name:         bucket.Name,
				CreationDate: bucket.CreationDate,
			})
		}
	}
	return bucketsList, nil
}

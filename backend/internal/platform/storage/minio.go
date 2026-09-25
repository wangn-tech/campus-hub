package storage

import (
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/wangn-tech/campus-hub/internal/config"
	"io"
	"net/url"
	"time"
)

type Client struct {
	client     *minio.Client
	bucket     string
	presignTTL time.Duration
}

func Open(cfg config.StorageConfig) (*Client, error) {
	c, err := minio.New(cfg.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: cfg.UseSSL, Region: ""})
	if err != nil {
		return nil, fmt.Errorf("new minio client: %w", err)
	}
	return &Client{client: c, bucket: cfg.Bucket, presignTTL: cfg.PresignTTL}, nil
}
func (c *Client) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (minio.UploadInfo, error) {
	if err := c.ensureBucket(ctx); err != nil {
		return minio.UploadInfo{}, err
	}
	return c.client.PutObject(ctx, c.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
}
func (c *Client) Remove(ctx context.Context, key string) error {
	return c.client.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
}
func (c *Client) SignedURL(ctx context.Context, key string) (string, error) {
	u, err := c.client.PresignedGetObject(ctx, c.bucket, key, c.presignTTL, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
func (c *Client) Check(ctx context.Context) error {
	_, err := c.client.BucketExists(ctx, c.bucket)
	return err
}

func (c *Client) ensureBucket(ctx context.Context) error {
	exists, err := c.client.BucketExists(ctx, c.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if err = c.client.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
		exists, checkErr := c.client.BucketExists(ctx, c.bucket)
		if checkErr != nil || !exists {
			return err
		}
	}
	return nil
}

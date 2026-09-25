package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/wangn-tech/campus-hub/internal/config"
)

type Client struct {
	client     *s3.Client
	presigner  *s3.PresignClient
	bucket     string
	presignTTL time.Duration
}

func Open(cfg config.StorageConfig) (*Client, error) {
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	client := s3.New(s3.Options{
		Region:       "us-east-1",
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		BaseEndpoint: aws.String(fmt.Sprintf("%s://%s", scheme, cfg.Endpoint)),
		UsePathStyle: cfg.UsePathStyle,
	})
	return &Client{
		client:     client,
		presigner:  s3.NewPresignClient(client),
		bucket:     cfg.Bucket,
		presignTTL: cfg.PresignTTL,
	}, nil
}

func (c *Client) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	if err := c.ensureBucket(ctx); err != nil {
		return err
	}
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          r,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	return err
}

func (c *Client) Remove(ctx context.Context, key string) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (c *Client) SignedURL(ctx context.Context, key string) (string, error) {
	out, err := c.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(c.presignTTL))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func (c *Client) Check(ctx context.Context) error {
	_, err := c.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(c.bucket)})
	return err
}

func (c *Client) ensureBucket(ctx context.Context) error {
	if err := c.Check(ctx); err == nil {
		return nil
	}
	if _, err := c.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(c.bucket)}); err != nil {
		if checkErr := c.Check(ctx); checkErr != nil {
			return err
		}
	}
	return nil
}

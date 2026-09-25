package elasticsearch

import (
	"context"
	"fmt"
	"net/http"

	es "github.com/elastic/go-elasticsearch/v9"
	"github.com/wangn-tech/campus-hub/internal/config"
)

type Client struct{ *es.Client }

func Open(cfg config.ElasticsearchConfig) (*Client, error) {
	transport := &http.Transport{ResponseHeaderTimeout: cfg.RequestTimeout}
	client, err := es.NewClient(es.Config{
		Addresses:         cfg.Addresses,
		Username:          cfg.Username,
		Password:          cfg.Password,
		Transport:         transport,
		DisableRetry:      true,
		EnableMetrics:     false,
		EnableDebugLogger: false,
	})
	if err != nil {
		return nil, err
	}
	return &Client{Client: client}, nil
}

func (c *Client) Check(ctx context.Context) error {
	response, err := c.Info(c.Info.WithContext(ctx))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.IsError() {
		return fmt.Errorf("elasticsearch returned status %d", response.StatusCode)
	}
	return nil
}

func (c *Client) Close(ctx context.Context) error { return c.Client.Close(ctx) }

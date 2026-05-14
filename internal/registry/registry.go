package registry

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/eznix86/registry-client"
	"github.com/eznix86/docker-registry-cleaner/internal/config"
)

type Client struct {
	client *registryclient.BaseClient
	logger *slog.Logger
}

func New(cfg config.Registry, timeout time.Duration, logger *slog.Logger) (*Client, error) {
	httpClient := &http.Client{
		Timeout: timeout,
	}

	var auth registryclient.Auth
	username, password, err := cfg.GetCredentials()
	if err == nil && username != "" {
		auth = registryclient.BasicAuth{
			Username: username,
			Password: password,
		}
	}

	return &Client{
		client: &registryclient.BaseClient{
			HTTPClient:   httpClient,
			BaseURL:      cfg.URL,
			Auth:         auth,
			RetryBackoff: 500 * time.Millisecond,
			MaxAttempts:  3,
			Logger:       &slogAdapter{logger: logger.With("registry", cfg.Name)},
		},
		logger: logger,
	}, nil
}

func (c *Client) GetCatalog(ctx context.Context) ([]string, error) {
	resp, err := c.client.GetCatalog(ctx, nil)
	if err != nil {
		return nil, err
	}
	return resp.Repositories, nil
}

func (c *Client) ListTags(ctx context.Context, repo string) ([]string, error) {
	resp, err := c.client.ListTags(ctx, repo, nil)
	if err != nil {
		return nil, err
	}
	return resp.Tags, nil
}

func (c *Client) GetManifest(ctx context.Context, repo, ref string) (*registryclient.ManifestResponse, error) {
	return c.client.GetManifest(ctx, repo, ref)
}

func (c *Client) GetBlob(ctx context.Context, repo, digest string) (*registryclient.BlobResponse, error) {
	return c.client.GetBlob(ctx, repo, digest)
}

func (c *Client) DeleteManifest(ctx context.Context, repo, digest string) error {
	return c.client.DeleteManifest(ctx, repo, digest)
}

type slogAdapter struct {
	logger *slog.Logger
}

func (a *slogAdapter) Debug(msg string, args ...any) {
	a.logger.Debug(msg, args...)
}

func (a *slogAdapter) Info(msg string, args ...any) {
	a.logger.Info(msg, args...)
}

func (a *slogAdapter) Warn(msg string, args ...any) {
	a.logger.Warn(msg, args...)
}

func (a *slogAdapter) Error(msg string, args ...any) {
	a.logger.Error(msg, args...)
}

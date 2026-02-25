package s3client

import (
	"context"
	"fmt"
	"sync"

	"s3-client/internal/shared/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Factory struct {
	mu      sync.RWMutex
	clients map[string]*s3.Client
}

func NewFactory() *Factory {
	return &Factory{
		clients: make(map[string]*s3.Client),
	}
}

func (f *Factory) cacheKey(opts config.Options) string {
	return fmt.Sprintf("%s|%s|%s", opts.Profile, opts.Region, opts.Endpoint)
}

func (f *Factory) GetClient(ctx context.Context, opts config.Options) (*s3.Client, error) {
	key := f.cacheKey(opts)

	f.mu.RLock()
	if client, ok := f.clients[key]; ok {
		f.mu.RUnlock()
		return client, nil
	}
	f.mu.RUnlock()

	f.mu.Lock()
	defer f.mu.Unlock()

	if client, ok := f.clients[key]; ok {
		return client, nil
	}

	awsCfg, err := config.Load(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg)
	f.clients[key] = client

	return client, nil
}

func (f *Factory) ClearCache() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clients = make(map[string]*s3.Client)
}

type ClientOption func(*s3.Options)

func WithAccelerate(enabled bool) ClientOption {
	return func(o *s3.Options) {
		o.UseAccelerate = enabled
	}
}

func WithPathStyle(enabled bool) ClientOption {
	return func(o *s3.Options) {
		o.UsePathStyle = enabled
	}
}

func (f *Factory) GetClientWithOptions(ctx context.Context, opts config.Options, clientOpts ...ClientOption) (*s3.Client, error) {
	key := f.cacheKey(opts) + "|custom"

	f.mu.RLock()
	if client, ok := f.clients[key]; ok {
		f.mu.RUnlock()
		return client, nil
	}
	f.mu.RUnlock()

	f.mu.Lock()
	defer f.mu.Unlock()

	if client, ok := f.clients[key]; ok {
		return client, nil
	}

	awsCfg, err := config.Load(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		for _, opt := range clientOpts {
			opt(o)
		}
	})
	f.clients[key] = client

	return client, nil
}

func GetClientFromConfig(ctx context.Context, awsCfg aws.Config) *s3.Client {
	return s3.NewFromConfig(awsCfg)
}

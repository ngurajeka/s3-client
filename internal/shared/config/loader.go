package config

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

func Load(ctx context.Context, opts Options) (aws.Config, error) {
	var cfgOpts []func(*config.LoadOptions) error

	if opts.Region != "" {
		cfgOpts = append(cfgOpts, config.WithRegion(opts.Region))
	}

	if opts.Profile != "" {
		cfgOpts = append(cfgOpts, config.WithSharedConfigProfile(opts.Profile))
	}

	if opts.Endpoint != "" {
		cfgOpts = append(cfgOpts, config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{
						URL:               opts.Endpoint,
						HostnameImmutable: true,
					}, nil
				},
			),
		))
	}

	return config.LoadDefaultConfig(ctx, cfgOpts...)
}

func LoadWithCredentials(ctx context.Context, opts Options, accessKey, secretKey string) (aws.Config, error) {
	cfg, err := Load(ctx, opts)
	if err != nil {
		return cfg, err
	}

	creds := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
	cfg.Credentials = creds

	return cfg, nil
}

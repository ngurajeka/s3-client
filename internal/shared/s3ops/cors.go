package s3ops

import (
	"context"
	"encoding/xml"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type CORSRule struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	ExposeHeaders  []string
	MaxAgeSeconds  *int32
}

type CORSConfiguration struct {
	Rules []CORSRule `xml:"CORSRule"`
}

func GetBucketCors(ctx context.Context, client *s3.Client, bucket string) ([]CORSRule, error) {
	resp, err := client.GetBucketCors(ctx, &s3.GetBucketCorsInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket CORS: %w", err)
	}

	if resp.CORSRules == nil {
		return nil, nil
	}

	rules := make([]CORSRule, len(resp.CORSRules))
	for i, rule := range resp.CORSRules {
		rules[i] = CORSRule{
			AllowedOrigins: rule.AllowedOrigins,
			AllowedMethods: rule.AllowedMethods,
			AllowedHeaders: rule.AllowedHeaders,
			ExposeHeaders:  rule.ExposeHeaders,
			MaxAgeSeconds:  rule.MaxAgeSeconds,
		}
	}

	return rules, nil
}

func PutBucketCors(ctx context.Context, client *s3.Client, bucket string, rules []CORSRule) error {
	s3Rules := make([]types.CORSRule, len(rules))
	for i, rule := range rules {
		s3Rules[i] = types.CORSRule{
			AllowedOrigins: rule.AllowedOrigins,
			AllowedMethods: rule.AllowedMethods,
			AllowedHeaders: rule.AllowedHeaders,
			ExposeHeaders:  rule.ExposeHeaders,
			MaxAgeSeconds:  rule.MaxAgeSeconds,
		}
	}

	_, err := client.PutBucketCors(ctx, &s3.PutBucketCorsInput{
		Bucket: aws.String(bucket),
		CORSConfiguration: &types.CORSConfiguration{
			CORSRules: s3Rules,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to put bucket CORS: %w", err)
	}

	return nil
}

func DeleteBucketCors(ctx context.Context, client *s3.Client, bucket string) error {
	_, err := client.DeleteBucketCors(ctx, &s3.DeleteBucketCorsInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to delete bucket CORS: %w", err)
	}
	return nil
}

func ParseCORSConfig(data []byte) ([]CORSRule, error) {
	var config CORSConfiguration
	if err := xml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse CORS config: %w", err)
	}
	return config.Rules, nil
}

func MarshalCORSConfig(rules []CORSRule) ([]byte, error) {
	config := CORSConfiguration{Rules: rules}
	data, err := xml.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal CORS config: %w", err)
	}
	return data, nil
}

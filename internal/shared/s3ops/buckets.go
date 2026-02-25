package s3ops

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type BucketInfo struct {
	Name         string
	CreationDate time.Time
	Region       string
}

func ListBuckets(ctx context.Context, client *s3.Client) ([]BucketInfo, error) {
	resp, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	var buckets []BucketInfo
	for _, b := range resp.Buckets {
		buckets = append(buckets, BucketInfo{
			Name:         aws.ToString(b.Name),
			CreationDate: aws.ToTime(b.CreationDate),
		})
	}

	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Name < buckets[j].Name
	})

	return buckets, nil
}

func ListBucketNames(ctx context.Context, client *s3.Client) ([]string, error) {
	buckets, err := ListBuckets(ctx, client)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(buckets))
	for i, b := range buckets {
		names[i] = b.Name
	}

	return names, nil
}

func GetBucketLocation(ctx context.Context, client *s3.Client, bucket string) (string, error) {
	resp, err := client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get bucket location: %w", err)
	}

	region := string(resp.LocationConstraint)
	if region == "" {
		region = "us-east-1"
	}

	return region, nil
}

func CreateBucket(ctx context.Context, client *s3.Client, bucket, region string) error {
	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}

	_, err := client.CreateBucket(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	return nil
}

func BucketExists(ctx context.Context, client *s3.Client, bucket string) (bool, error) {
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

package s3ops

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ObjectMetadata struct {
	Name                 string
	Key                  string
	Size                 int64
	ContentType          string
	ContentLength        int64
	LastModified         *string
	ETag                 string
	StorageClass         string
	Metadata             map[string]string
	ServerSideEncryption string
}

func HeadObject(ctx context.Context, client *s3.Client, bucket, key string) (*ObjectMetadata, error) {
	resp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to head object: %w", err)
	}

	lastMod := ""
	if resp.LastModified != nil {
		lastMod = resp.LastModified.Format("2006-01-02 15:04:05")
	}

	meta := &ObjectMetadata{
		Name:                 key,
		Key:                  key,
		Size:                 aws.ToInt64(resp.ContentLength),
		ContentType:          aws.ToString(resp.ContentType),
		ContentLength:        aws.ToInt64(resp.ContentLength),
		LastModified:         &lastMod,
		ETag:                 aws.ToString(resp.ETag),
		StorageClass:         string(resp.StorageClass),
		Metadata:             resp.Metadata,
		ServerSideEncryption: string(resp.ServerSideEncryption),
	}

	return meta, nil
}

func GetObjectInfo(ctx context.Context, client *s3.Client, bucket, key string) (*ObjectMetadata, error) {
	return HeadObject(ctx, client, bucket, key)
}

func ObjectExists(ctx context.Context, client *s3.Client, bucket, key string) (bool, error) {
	_, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

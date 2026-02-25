package s3ops

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type ObjectInfo struct {
	Name         string
	Key          string
	IsDir        bool
	Size         int64
	LastModified *string
	StorageClass string
	ETag         string
}

func ListObjects(ctx context.Context, client *s3.Client, bucket, prefix string) ([]ObjectInfo, error) {
	if !strings.HasSuffix(prefix, "/") && prefix != "" {
		prefix += "/"
	}

	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	}

	var entries []ObjectInfo
	paginator := s3.NewListObjectsV2Paginator(client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, commonPrefix := range page.CommonPrefixes {
			name := aws.ToString(commonPrefix.Prefix)
			name = strings.TrimPrefix(name, prefix)
			if name == "" {
				continue
			}
			entries = append(entries, ObjectInfo{
				Name:  name,
				Key:   aws.ToString(commonPrefix.Prefix),
				IsDir: true,
			})
		}

		for _, obj := range page.Contents {
			name := aws.ToString(obj.Key)
			if name == prefix {
				continue
			}
			name = strings.TrimPrefix(name, prefix)
			if name == "" {
				continue
			}

			lastMod := ""
			if obj.LastModified != nil {
				lastMod = obj.LastModified.Format("2006-01-02 15:04:05")
			}

			entries = append(entries, ObjectInfo{
				Name:         name,
				Key:          aws.ToString(obj.Key),
				IsDir:        false,
				Size:         aws.ToInt64(obj.Size),
				LastModified: &lastMod,
				StorageClass: string(obj.StorageClass),
				ETag:         aws.ToString(obj.ETag),
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})

	return entries, nil
}

func ListObjectNames(ctx context.Context, client *s3.Client, bucket, prefix string) ([]string, error) {
	objects, err := ListObjects(ctx, client, bucket, prefix)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(objects))
	for i, o := range objects {
		names[i] = o.Name
	}

	return names, nil
}

type ListObjectsFunc func(bucket, prefix string) ([]ObjectInfo, error)

func NewListObjectsPaginator(client *s3.Client, bucket, prefix string) func(context.Context) (func(bucket, prefix string) ([]ObjectInfo, error), error) {
	return func(ctx context.Context) (func(bucket, prefix string) ([]ObjectInfo, error), error) {
		return func(bucket, prefix string) ([]ObjectInfo, error) {
			return ListObjects(ctx, client, bucket, prefix)
		}, nil
	}
}

func ListObjectsAll(ctx context.Context, client *s3.Client, bucket, prefix string) ([]ObjectInfo, error) {
	if !strings.HasSuffix(prefix, "/") && prefix != "" {
		prefix += "/"
	}

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}

	var entries []ObjectInfo
	paginator := s3.NewListObjectsV2Paginator(client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			lastMod := ""
			if obj.LastModified != nil {
				lastMod = obj.LastModified.Format("2006-01-02 15:04:05")
			}

			entries = append(entries, ObjectInfo{
				Name:         aws.ToString(obj.Key),
				Key:          aws.ToString(obj.Key),
				IsDir:        false,
				Size:         aws.ToInt64(obj.Size),
				LastModified: &lastMod,
				StorageClass: string(obj.StorageClass),
				ETag:         aws.ToString(obj.ETag),
			})
		}
	}

	return entries, nil
}

func CopyObject(ctx context.Context, client *s3.Client, sourceBucket, sourceKey, destBucket, destKey string) error {
	_, err := client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(destBucket),
		Key:        aws.String(destKey),
		CopySource: aws.String(sourceBucket + "/" + sourceKey),
	})
	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}

	return nil
}

func GetObjectACL(ctx context.Context, client *s3.Client, bucket, key string) (*types.AccessControlPolicy, error) {
	resp, err := client.GetObjectAcl(ctx, &s3.GetObjectAclInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object ACL: %w", err)
	}

	return &types.AccessControlPolicy{
		Grants: resp.Grants,
		Owner:  resp.Owner,
	}, nil
}

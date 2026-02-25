package s3ops

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func DeleteObject(ctx context.Context, client *s3.Client, bucket, key string) error {
	_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

type DeleteResult struct {
	Key     string
	Deleted bool
	Error   error
}

func DeleteObjects(ctx context.Context, client *s3.Client, bucket string, keys []string, quiet bool) ([]DeleteResult, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	objects := make([]types.ObjectIdentifier, len(keys))
	for i, key := range keys {
		objects[i] = types.ObjectIdentifier{Key: aws.String(key)}
	}

	resp, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{Objects: objects, Quiet: aws.Bool(quiet)},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to delete objects: %w", err)
	}

	results := make([]DeleteResult, len(keys))
	for i, key := range keys {
		results[i] = DeleteResult{Key: key, Deleted: true}
	}

	if resp.Deleted != nil {
		for _, d := range resp.Deleted {
			key := aws.ToString(d.Key)
			for i := range results {
				if results[i].Key == key {
					results[i].Deleted = true
					break
				}
			}
		}
	}

	if resp.Errors != nil {
		for _, e := range resp.Errors {
			key := aws.ToString(e.Key)
			for i := range results {
				if results[i].Key == key {
					results[i].Deleted = false
					results[i].Error = fmt.Errorf("%s: %s", aws.ToString(e.Code), aws.ToString(e.Message))
					break
				}
			}
		}
	}

	return results, nil
}

func DeletePrefix(ctx context.Context, client *s3.Client, bucket, prefix string) (int, error) {
	if prefix != "" && !hasSuffix(prefix, "/") {
		prefix += "/"
	}

	objects, err := ListObjectsAll(ctx, client, bucket, prefix)
	if err != nil {
		return 0, fmt.Errorf("failed to list objects: %w", err)
	}

	if len(objects) == 0 {
		return 0, nil
	}

	keys := make([]string, len(objects))
	for i, obj := range objects {
		keys[i] = obj.Key
	}

	results, err := DeleteObjects(ctx, client, bucket, keys, true)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, r := range results {
		if r.Deleted {
			deleted++
		}
	}

	return deleted, nil
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

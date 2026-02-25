package connect

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Entry struct {
	Name         string
	IsDir        bool
	Size         int64
	LastModified *string
	StorageClass string
	ETag         string
}

type Progress struct {
	TotalBytes      int64
	DownloadedBytes int64
}

func listBuckets(ctx context.Context, client *s3.Client) ([]string, error) {
	resp, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	var buckets []string
	for _, b := range resp.Buckets {
		buckets = append(buckets, aws.ToString(b.Name))
	}
	sort.Strings(buckets)
	return buckets, nil
}

func listObjects(ctx context.Context, client *s3.Client, bucket, prefix string) ([]S3Entry, error) {
	if !strings.HasSuffix(prefix, "/") && prefix != "" {
		prefix += "/"
	}

	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	}

	var entries []S3Entry
	paginator := s3.NewListObjectsV2Paginator(client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		// Folders
		for _, commonPrefix := range page.CommonPrefixes {
			name := aws.ToString(commonPrefix.Prefix)
			name = strings.TrimPrefix(name, prefix)
			if name == "" {
				continue
			}
			entries = append(entries, S3Entry{
				Name:  name,
				IsDir: true,
			})
		}

		// Files
		for _, obj := range page.Contents {
			name := aws.ToString(obj.Key)
			if name == prefix {
				continue // Skip the directory itself
			}
			name = strings.TrimPrefix(name, prefix)
			if name == "" {
				continue
			}

			lastMod := ""
			if obj.LastModified != nil {
				lastMod = obj.LastModified.Format("2006-01-02 15:04:05")
			}

			entries = append(entries, S3Entry{
				Name:         name,
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

func getObjectMetadata(ctx context.Context, client *s3.Client, bucket, key string) (*S3Entry, error) {
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

	return &S3Entry{
		Name:         key,
		IsDir:        false,
		Size:         aws.ToInt64(resp.ContentLength),
		LastModified: &lastMod,
		StorageClass: string(resp.StorageClass),
		ETag:         aws.ToString(resp.ETag),
	}, nil
}

func downloadObject(ctx context.Context, client *s3.Client, bucket, key, outputPath string, progress func(Progress)) error {
	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	total := aws.ToInt64(resp.ContentLength)
	downloaded := int64(0)
	buf := make([]byte, 32*1024)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			downloaded += int64(n)
			progress(Progress{
				TotalBytes:      total,
				DownloadedBytes: downloaded,
			})
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

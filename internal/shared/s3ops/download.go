package s3ops

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type DownloadProgress struct {
	TotalBytes      int64
	DownloadedBytes int64
}

func DownloadObject(ctx context.Context, client *s3.Client, bucket, key, outputPath string, progress func(DownloadProgress)) error {
	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to get object: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	total := aws.ToInt64(resp.ContentLength)
	downloaded := int64(0)
	buf := make([]byte, 32*1024)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return fmt.Errorf("failed to write: %w", werr)
			}
			downloaded += int64(n)
			if progress != nil {
				progress(DownloadProgress{
					TotalBytes:      total,
					DownloadedBytes: downloaded,
				})
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read object: %w", err)
		}
	}

	return nil
}

type RangeDownload struct {
	Start int64
	End   int64
}

func DownloadRange(ctx context.Context, client *s3.Client, bucket, key string, rangeSpec RangeDownload) ([]byte, error) {
	rangeVal := fmt.Sprintf("bytes=%d-%d", rangeSpec.Start, rangeSpec.End)

	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  aws.String(rangeVal),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object range: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object: %w", err)
	}

	return data, nil
}

func GetObjectSize(ctx context.Context, client *s3.Client, bucket, key string) (int64, error) {
	resp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to head object: %w", err)
	}

	return aws.ToInt64(resp.ContentLength), nil
}

func DownloadToWriter(ctx context.Context, client *s3.Client, bucket, key string, w io.WriterAt, offset int64, length int64, progress func(int64)) error {
	rangeVal := fmt.Sprintf("bytes=%d-%d", offset, offset+length-1)

	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  aws.String(rangeVal),
	})
	if err != nil {
		return fmt.Errorf("failed to get object range: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read object: %w", err)
	}

	if _, err := w.WriteAt(data, offset); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}

	if progress != nil {
		progress(int64(len(data)))
	}

	return nil
}

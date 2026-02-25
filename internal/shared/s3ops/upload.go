package s3ops

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type UploadProgress struct {
	TotalBytes    int64
	UploadedBytes int64
	PartNumber    int
	TotalParts    int
}

func UploadFile(ctx context.Context, client *s3.Client, localPath, bucket, key string, progress func(UploadProgress)) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          file,
		ContentLength: aws.Int64(stat.Size()),
		ContentType:   aws.String(getContentType(localPath)),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	if progress != nil {
		progress(UploadProgress{
			TotalBytes:    stat.Size(),
			UploadedBytes: stat.Size(),
		})
	}

	return nil
}

func UploadDirectory(ctx context.Context, client *s3.Client, localDir, bucket, prefix string, progress func(UploadProgress)) error {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	var totalBytes int64
	for _, e := range entries {
		if !e.IsDir() {
			info, _ := e.Info()
			totalBytes += info.Size()
		}
	}

	var uploaded int64

	for _, e := range entries {
		path := filepath.Join(localDir, e.Name())
		key := filepath.Join(prefix, e.Name())

		if e.IsDir() {
			subDir := filepath.Join(localDir, e.Name())
			err := uploadDirectoryRecursive(ctx, client, subDir, bucket, key, &uploaded, totalBytes, progress)
			if err != nil {
				return err
			}
		} else {
			err := UploadFile(ctx, client, path, bucket, key, nil)
			if err != nil {
				return fmt.Errorf("failed to upload %s: %w", e.Name(), err)
			}
			info, _ := e.Info()
			uploaded += info.Size()
			if progress != nil {
				progress(UploadProgress{
					TotalBytes:    totalBytes,
					UploadedBytes: uploaded,
				})
			}
		}
	}

	return nil
}

func uploadDirectoryRecursive(ctx context.Context, client *s3.Client, localDir, bucket, prefix string, uploaded *int64, total int64, progress func(UploadProgress)) error {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, e := range entries {
		path := filepath.Join(localDir, e.Name())
		key := filepath.Join(prefix, e.Name())

		if e.IsDir() {
			err := uploadDirectoryRecursive(ctx, client, path, bucket, key, uploaded, total, progress)
			if err != nil {
				return err
			}
		} else {
			err := UploadFile(ctx, client, path, bucket, key, nil)
			if err != nil {
				return fmt.Errorf("failed to upload %s: %w", e.Name(), err)
			}
			info, _ := e.Info()
			*uploaded += info.Size()
			if progress != nil {
				progress(UploadProgress{
					TotalBytes:    total,
					UploadedBytes: *uploaded,
				})
			}
		}
	}

	return nil
}

type MultipartUploader struct {
	client         *s3.Client
	bucket         string
	key            string
	uploadID       *string
	completedParts []types.CompletedPart
	partSize       int64
	totalBytes     int64
	uploadedBytes  int64
}

func NewMultipartUploader(client *s3.Client, bucket, key string, partSize int64) *MultipartUploader {
	return &MultipartUploader{
		client:   client,
		bucket:   bucket,
		key:      key,
		partSize: partSize,
	}
}

func (m *MultipartUploader) Start(ctx context.Context) error {
	resp, err := m.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(m.key),
	})
	if err != nil {
		return fmt.Errorf("failed to start multipart upload: %w", err)
	}

	m.uploadID = resp.UploadId
	return nil
}

func (m *MultipartUploader) UploadPart(ctx context.Context, partNumber int, data []byte) error {
	resp, err := m.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(m.bucket),
		Key:        aws.String(m.key),
		UploadId:   m.uploadID,
		PartNumber: aws.Int32(int32(partNumber)),
		Body:       strings.NewReader(string(data)),
	})
	if err != nil {
		return fmt.Errorf("failed to upload part %d: %w", partNumber, err)
	}

	m.completedParts = append(m.completedParts, types.CompletedPart{
		ETag:       resp.ETag,
		PartNumber: aws.Int32(int32(partNumber)),
	})

	m.uploadedBytes += int64(len(data))

	return nil
}

func (m *MultipartUploader) Complete(ctx context.Context) error {
	_, err := m.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(m.bucket),
		Key:             aws.String(m.key),
		UploadId:        m.uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: m.completedParts},
	})
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	return nil
}

func (m *MultipartUploader) Abort(ctx context.Context) error {
	_, err := m.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(m.bucket),
		Key:      aws.String(m.key),
		UploadId: m.uploadID,
	})
	return err
}

func UploadMultipart(ctx context.Context, client *s3.Client, localPath, bucket, key string, partSize int64, progress func(UploadProgress)) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	resp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to start multipart upload: %w", err)
	}

	uploadID := resp.UploadId

	var completedParts []types.CompletedPart
	partNumber := 1
	offset := int64(0)

	for offset < stat.Size() {
		remaining := stat.Size() - offset
		chunkSize := partSize
		if remaining < chunkSize {
			chunkSize = remaining
		}

		buf := make([]byte, chunkSize)
		if _, err := file.Read(buf); err != nil {
			client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(bucket),
				Key:      aws.String(key),
				UploadId: uploadID,
			})
			return fmt.Errorf("failed to read file: %w", err)
		}

		uploadResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(bucket),
			Key:        aws.String(key),
			UploadId:   uploadID,
			PartNumber: aws.Int32(int32(partNumber)),
			Body:       strings.NewReader(string(buf)),
		})
		if err != nil {
			client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(bucket),
				Key:      aws.String(key),
				UploadId: uploadID,
			})
			return fmt.Errorf("failed to upload part %d: %w", partNumber, err)
		}

		completedParts = append(completedParts, types.CompletedPart{
			ETag:       uploadResp.ETag,
			PartNumber: aws.Int32(int32(partNumber)),
		})

		offset += chunkSize
		partNumber++

		if progress != nil {
			progress(UploadProgress{
				TotalBytes:    stat.Size(),
				UploadedBytes: offset,
				PartNumber:    partNumber - 1,
			})
		}
	}

	_, err = client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(key),
		UploadId:        uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completedParts},
	})
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	return nil
}

func getContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".txt":
		return "text/plain"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".tar":
		return "application/x-tar"
	case ".gz", ".tgz":
		return "application/gzip"
	default:
		return "application/octet-stream"
	}
}

type ReaderAtSeeker interface {
	io.ReaderAt
	io.Seeker
}

func UploadMultipartWithReader(ctx context.Context, client *s3.Client, reader ReaderAtSeeker, size int64, bucket, key string, partSize int64, progress func(UploadProgress)) error {
	resp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to start multipart upload: %w", err)
	}

	uploadID := resp.UploadId

	var completedParts []types.CompletedPart
	partNumber := 1
	offset := int64(0)

	for offset < size {
		remaining := size - offset
		chunkSize := partSize
		if remaining < chunkSize {
			chunkSize = remaining
		}

		buf := make([]byte, chunkSize)
		_, err := reader.ReadAt(buf, offset)
		if err != nil && err != io.EOF {
			client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(bucket),
				Key:      aws.String(key),
				UploadId: uploadID,
			})
			return fmt.Errorf("failed to read at offset %d: %w", offset, err)
		}

		uploadResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(bucket),
			Key:        aws.String(key),
			UploadId:   uploadID,
			PartNumber: aws.Int32(int32(partNumber)),
			Body:       strings.NewReader(string(buf)),
		})
		if err != nil {
			client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(bucket),
				Key:      aws.String(key),
				UploadId: uploadID,
			})
			return fmt.Errorf("failed to upload part %d: %w", partNumber, err)
		}

		completedParts = append(completedParts, types.CompletedPart{
			ETag:       uploadResp.ETag,
			PartNumber: aws.Int32(int32(partNumber)),
		})

		offset += chunkSize
		partNumber++

		if progress != nil {
			progress(UploadProgress{
				TotalBytes:    size,
				UploadedBytes: offset,
				PartNumber:    partNumber - 1,
			})
		}
	}

	_, err = client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(key),
		UploadId:        uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completedParts},
	})
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	return nil
}

package upload

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"s3-client/internal/s3uri"
	"s3-client/internal/shared/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func newFlagSet() *flag.FlagSet {
	return flag.NewFlagSet("upload", flag.ContinueOnError)
}

func printUsage(fs *flag.FlagSet) {
	fmt.Fprintln(os.Stderr, "Usage: s3-client upload [flags] <local-path> s3://bucket/key")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Upload a file or directory to S3.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Examples:")
	fmt.Fprintln(os.Stderr, "  s3-client upload file.txt s3://my-bucket/backups/")
	fmt.Fprintln(os.Stderr, "  s3-client upload -profile prod -region us-west-2 ./data/ s3://my-bucket/data/")
	fmt.Fprintln(os.Stderr, "  s3-client upload -multipart -part-size 25 large.file s3://my-bucket/large/")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Flags:")
	fs.PrintDefaults()
}

func Run(args []string) int {
	fs := newFlagSet()
	multipart := fs.Bool("multipart", false, "Use multipart upload for large files")
	partSizeMB := fs.Int("part-size", 10, "Part size in MB for multipart upload")
	metadata := fs.String("metadata", "", "Metadata in KEY=VALUE,KEY=VALUE format")
	guessContentType := fs.Bool("guess-content-type", true, "Guess content type from file extension")

	opts := &config.Options{}
	config.AddFlags(fs, opts)

	fs.Usage = func() {
		printUsage(fs)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 2 {
		fs.Usage()
		return 1
	}

	localPath := fs.Arg(0)
	s3URI := fs.Arg(1)

	bucket, keyPrefix, err := s3uri.Parse(s3URI)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	stat, err := os.Stat(localPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	ctx := context.Background()
	cfg, err := config.Load(ctx, *opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load AWS config: %v\n", err)
		return 1
	}

	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "\n❌ AWS credentials not found or invalid.")
		fmt.Fprintln(os.Stderr, "\nOptions to fix:")
		fmt.Fprintln(os.Stderr, "  1. s3-client upload -profile myprofile ...")
		fmt.Fprintln(os.Stderr, "  2. export AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=...")
		fmt.Fprintf(os.Stderr, "\nDetail: %v\n", err)
		return 1
	}
	if opts.Profile != "" {
		fmt.Printf("Using AWS profile: %s (source: %s)\n", opts.Profile, creds.Source)
	}

	client := s3.NewFromConfig(cfg)

	var meta map[string]string
	if *metadata != "" {
		meta = parseMetadata(*metadata)
	}

	start := time.Now()

	if stat.IsDir() {
		localPath = strings.TrimSuffix(localPath, string(os.PathSeparator))
		dirName := filepath.Base(localPath)
		prefix := keyPrefix + dirName + "/"

		fmt.Printf("Uploading directory: %s\n", localPath)
		fmt.Printf("To: s3://%s/%s\n\n", bucket, prefix)

		err = uploadDirectory(ctx, client, localPath, bucket, prefix, meta, *guessContentType)
	} else {
		fileName := filepath.Base(localPath)
		key := keyPrefix + fileName

		fmt.Printf("Uploading file: %s\n", localPath)
		fmt.Printf("To: s3://%s/%s\n\n", bucket, key)

		if *multipart || stat.Size() > int64(*partSizeMB)*1024*1024 {
			err = uploadMultipart(ctx, client, localPath, bucket, key, int64(*partSizeMB)*1024*1024, meta)
		} else {
			err = uploadSingleFile(ctx, client, localPath, bucket, key, meta, *guessContentType)
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Upload failed: %v\n", err)
		return 1
	}

	elapsed := time.Since(start)
	fmt.Printf("\n✓ Done! Uploaded in %s\n", formatDuration(elapsed))
	return 0
}

func uploadSingleFile(ctx context.Context, client *s3.Client, localPath, bucket, key string, meta map[string]string, guessContentType bool) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	input := &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          file,
		ContentLength: aws.Int64(stat.Size()),
	}

	if guessContentType {
		contentType := guessContentTypeFromExt(localPath)
		if contentType != "" {
			input.ContentType = aws.String(contentType)
		}
	}

	if len(meta) > 0 {
		input.Metadata = meta
	}

	_, err = client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to upload: %w", err)
	}

	return nil
}

func uploadMultipart(ctx context.Context, client *s3.Client, localPath, bucket, key string, partSize int64, meta map[string]string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	totalSize := stat.Size()
	partSizeBytes := partSize
	if partSizeBytes <= 0 {
		partSizeBytes = 10 * 1024 * 1024
	}

	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		Metadata: meta,
	})
	if err != nil {
		return fmt.Errorf("failed to start multipart upload: %w", err)
	}

	uploadID := createResp.UploadId

	var completedParts []types.CompletedPart
	partNumber := 1
	offset := int64(0)

	fmt.Printf("Multipart upload: %d parts\n", (totalSize+partSizeBytes-1)/partSizeBytes)

	for offset < totalSize {
		remaining := totalSize - offset
		chunkSize := partSizeBytes
		if remaining < chunkSize {
			chunkSize = remaining
		}

		buf := make([]byte, chunkSize)
		_, err := file.ReadAt(buf, offset)
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

		pct := float64(offset) / float64(totalSize) * 100
		fmt.Printf("\rProgress: %.1f%%", pct)
	}
	fmt.Println()

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

func uploadDirectory(ctx context.Context, client *s3.Client, localDir, bucket, prefix string, meta map[string]string, guessContentType bool) error {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	var totalFiles int
	var totalBytes int64
	for _, e := range entries {
		if !e.IsDir() {
			info, _ := e.Info()
			totalFiles++
			totalBytes += info.Size()
		}
	}

	fmt.Printf("Total files: %d, Total size: %s\n\n", totalFiles, formatSize(totalBytes))

	uploaded := 0
	var uploadedBytes int64

	for _, e := range entries {
		path := filepath.Join(localDir, e.Name())
		key := prefix + e.Name()

		if e.IsDir() {
			err := uploadDirectoryRecursive(ctx, client, path, bucket, key+"/", meta, guessContentType, &uploaded, &uploadedBytes, totalBytes)
			if err != nil {
				return err
			}
		} else {
			err := uploadSingleFile(ctx, client, path, bucket, key, meta, guessContentType)
			if err != nil {
				return fmt.Errorf("failed to upload %s: %w", e.Name(), err)
			}
			info, _ := e.Info()
			uploadedBytes += info.Size()
			uploaded++
			pct := float64(uploadedBytes) / float64(totalBytes) * 100
			fmt.Printf("\rUploaded %d/%d files (%.1f%%)", uploaded, totalFiles, pct)
		}
	}
	fmt.Println()

	return nil
}

func uploadDirectoryRecursive(ctx context.Context, client *s3.Client, localDir, bucket, prefix string, meta map[string]string, guessContentType bool, uploaded *int, uploadedBytes *int64, totalBytes int64) error {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, e := range entries {
		path := filepath.Join(localDir, e.Name())
		key := prefix + e.Name()

		if e.IsDir() {
			err := uploadDirectoryRecursive(ctx, client, path, bucket, key+"/", meta, guessContentType, uploaded, uploadedBytes, totalBytes)
			if err != nil {
				return err
			}
		} else {
			err := uploadSingleFile(ctx, client, path, bucket, key, meta, guessContentType)
			if err != nil {
				return fmt.Errorf("failed to upload %s: %w", e.Name(), err)
			}
			info, _ := e.Info()
			*uploadedBytes += info.Size()
			*uploaded++
			pct := float64(*uploadedBytes) / float64(totalBytes) * 100
			fmt.Printf("\rUploaded %d files (%.1f%%)", *uploaded, pct)
		}
	}

	return nil
}

func parseMetadata(s string) map[string]string {
	meta := make(map[string]string)
	if s == "" {
		return meta
	}
	pairs := strings.Split(s, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			meta[parts[0]] = parts[1]
		}
	}
	return meta
}

func guessContentTypeFromExt(path string) string {
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
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
}

package s3uri

import (
	"fmt"
	"strings"
)

// Parse extracts bucket and key from an S3 URI (s3://bucket/key/path).
func Parse(uri string) (bucket, key string, err error) {
	if !strings.HasPrefix(uri, "s3://") {
		return "", "", fmt.Errorf("invalid S3 URI %q: must start with s3://", uri)
	}
	rest := strings.TrimPrefix(uri, "s3://")
	idx := strings.IndexByte(rest, '/')
	if idx == -1 {
		return "", "", fmt.Errorf("invalid S3 URI %q: no key found after bucket name", uri)
	}
	bucket = rest[:idx]
	key = rest[idx+1:]
	if bucket == "" {
		return "", "", fmt.Errorf("invalid S3 URI %q: bucket name is empty", uri)
	}
	if key == "" {
		return "", "", fmt.Errorf("invalid S3 URI %q: key is empty", uri)
	}
	return bucket, key, nil
}

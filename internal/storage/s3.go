package storage

import (
	"errors"
	"os"
)

// ErrS3NotImplemented is returned by all S3Storage methods.
var ErrS3NotImplemented = errors.New("s3 storage: not yet implemented — planned for future release")

// S3Storage is a placeholder for future S3-compatible object storage.
// It implements the Storage interface as a compile-time check but returns
// ErrS3NotImplemented on every operation.
//
// To add real S3 support:
//  1. Add an S3 SDK dependency (e.g. github.com/aws/aws-sdk-go-v2/service/s3)
//  2. Replace the stub methods with real S3 API calls
//  3. Map S3 keys to paths and vice versa
//  4. Handle S3-specific concerns: eventual consistency, multipart upload, etags
type S3Storage struct {
	bucket string
	prefix string // optional key prefix for multi-tenant within a bucket
}

// NewS3Storage creates an S3Storage placeholder for the given bucket and key prefix.
// prefix is optional — use "" for bucket-root access.
// Until the S3 implementation is completed, all operations return ErrS3NotImplemented.
func NewS3Storage(bucket, prefix string) (*S3Storage, error) {
	if bucket == "" {
		return nil, errors.New("s3 bucket name is required")
	}
	return &S3Storage{bucket: bucket, prefix: prefix}, nil
}

// Bucket returns the configured S3 bucket name.
func (s *S3Storage) Bucket() string { return s.bucket }

// KeyPrefix returns the optional key prefix for path mapping.
func (s *S3Storage) KeyPrefix() string { return s.prefix }

// pathToKey converts a filesystem-style path to an S3 object key.
func (s *S3Storage) pathToKey(path string) string {
	return s.prefix + path
}

func (s *S3Storage) ReadFile(path string) ([]byte, error)                 { return nil, ErrS3NotImplemented }
func (s *S3Storage) WriteFile(path string, data []byte, _ os.FileMode) error { return ErrS3NotImplemented }
func (s *S3Storage) MkdirAll(_ string, _ os.FileMode) error    { return ErrS3NotImplemented }
func (s *S3Storage) RemoveAll(_ string) error                  { return ErrS3NotImplemented }
func (s *S3Storage) Rename(_, _ string) error                  { return ErrS3NotImplemented }
func (s *S3Storage) Stat(_ string) (os.FileInfo, error)        { return nil, ErrS3NotImplemented }
func (s *S3Storage) ReadDir(_ string) ([]os.DirEntry, error)   { return nil, ErrS3NotImplemented }

// compile-time interface check
var _ Storage = (*S3Storage)(nil)

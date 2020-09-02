package warehouse

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/fullstorydev/hauser/client"
	"github.com/fullstorydev/hauser/config"
)

type S3Storage struct {
	StorageMixin
	conf *config.S3Config
}

var _ Storage = (*S3Storage)(nil)

func NewS3Storage(conf *config.S3Config) *S3Storage {
	return &S3Storage{
		conf: conf,
	}
}

func (s *S3Storage) LastSyncPoint(ctx context.Context) (time.Time, error) {
	return StorageMixin{s}.LastSyncPoint(ctx)
}

func (s *S3Storage) SaveSyncPoints(ctx context.Context, bundles ...client.ExportMeta) error {
	return StorageMixin{s}.SaveSyncPoints(ctx, bundles...)
}

func (s *S3Storage) SaveFile(ctx context.Context, name string, reader io.Reader) error {
	ctx, cancelFn := context.WithTimeout(ctx, s.conf.Timeout.Duration)
	defer cancelFn()

	bucket, key := s.getBucketAndKey(name)
	uploader := s3manager.NewUploader(s.newSession())
	_, err := uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   reader,
	})
	return err
}

func (s *S3Storage) ReadFile(ctx context.Context, name string) (io.Reader, error) {
	ctx, cancelFn := context.WithTimeout(ctx, s.conf.Timeout.Duration)
	defer cancelFn()

	bucket, key := s.getBucketAndKey(name)
	client := s3.New(s.newSession())
	out, err := client.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == s3.ErrCodeNoSuchKey {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to read s3 file %s: %s", name, err)
	}
	return out.Body, nil
}

func (s *S3Storage) DeleteFile(ctx context.Context, name string) error {
	ctx, cancelFn := context.WithTimeout(ctx, s.conf.Timeout.Duration)
	defer cancelFn()

	bucket, key := s.getBucketAndKey(name)
	client := s3.New(s.newSession())
	_, err := client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Not returning an error to maintain backward compatibility
		log.Printf("failed to delete S3 object %s: %s", name, err)
	}
	return nil
}

func (s *S3Storage) GetFileReference(name string) string {
	bucket, key := s.getBucketAndKey(name)
	return fmt.Sprintf("s3://%s/%s", bucket, key)
}

func (s *S3Storage) newSession() *session.Session {
	return session.Must(session.NewSession(aws.NewConfig().WithRegion(s.conf.Region)))
}

func (s *S3Storage) getBucketAndKey(objName string) (string, string) {
	bucketParts := strings.Split(s.conf.Bucket, "/")
	bucketName := bucketParts[0]
	keyPath := strings.Trim(strings.Join(bucketParts[1:], "/"), "/")
	key := strings.Trim(fmt.Sprintf("%s/%s", keyPath, objName), "/")

	return bucketName, key
}

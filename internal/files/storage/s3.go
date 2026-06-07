package storage

import (
	"bytes"
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Storage struct {
	client *s3.Client
	bucket string
}

func NewS3Storage(
	client *s3.Client,
	bucket string,
) *S3Storage {

	return &S3Storage{
		client: client,
		bucket: bucket,
	}
}

func (s *S3Storage) Upload(
	ctx context.Context,
	key string,
	data []byte,
) error {

	_, err := s.client.PutObject(
		ctx,
		&s3.PutObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader(data),
		},
	)

	return err
}

func (s *S3Storage) Download(
	ctx context.Context,
	key string,
) ([]byte, error) {

	output, err := s.client.GetObject(
		ctx,
		&s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		},
	)

	if err != nil {
		return nil, err
	}

	defer output.Body.Close()

	return io.ReadAll(
		output.Body,
	)
}

func (s *S3Storage) Delete(
	ctx context.Context,
	key string,
) error {

	_, err := s.client.DeleteObject(
		ctx,
		&s3.DeleteObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		},
	)

	return err
}

func (s *S3Storage) Exists(
	ctx context.Context,
	key string,
) (bool, error) {

	_, err := s.client.HeadObject(
		ctx,
		&s3.HeadObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		},
	)

	if err != nil {
		return false, nil
	}

	return true, nil
}

func (s *S3Storage) PutObject(ctx context.Context, key string, data []byte) error {
	return s.Upload(ctx, key, data)
}

func (s *S3Storage) GetObject(ctx context.Context, key string) ([]byte, error) {
	return s.Download(ctx, key)
}

func (s *S3Storage) DeleteObject(ctx context.Context, key string) error {
	return s.Delete(ctx, key)
}

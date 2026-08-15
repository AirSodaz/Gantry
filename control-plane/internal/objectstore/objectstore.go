package objectstore

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"time"

	"github.com/AirSodaz/gantry/internal/config"
	aws "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ObjectStore is the readiness port used by the public server.
type ObjectStore interface {
	Ready(context.Context) error
}

// ArtifactStore extends ObjectStore with the object operations used by task
// output and artifact services. Provider SDK types stay inside this adapter.
type ArtifactStore interface {
	ObjectStore
	Put(context.Context, string, io.Reader, int64, string) error
	Get(context.Context, string) (io.ReadCloser, error)
	Head(context.Context, string) (int64, error)
	PresignGet(context.Context, string, time.Duration) (string, time.Time, error)
}

type s3Store struct {
	endpoint *url.URL
	bucket   string
	client   *s3.Client
	presign  *s3.PresignClient
}

func NewS3(cfg config.ObjectStorageConfig) (ArtifactStore, error) {
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || endpoint.Host == "" {
		return nil, fmt.Errorf("invalid S3 endpoint")
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load S3 configuration: %w", err)
	}
	awsCfg.BaseEndpoint = aws.String(stringsTrimRight(cfg.Endpoint, "/"))
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.UsePathStyle = cfg.UsePathStyle
	})
	publicCfg := awsCfg
	publicCfg.BaseEndpoint = aws.String(stringsTrimRight(cfg.PublicEndpoint, "/"))
	publicClient := s3.NewFromConfig(publicCfg, func(options *s3.Options) {
		options.UsePathStyle = cfg.UsePathStyle
	})
	return &s3Store{endpoint: endpoint, bucket: cfg.Bucket, client: client, presign: s3.NewPresignClient(publicClient)}, nil
}

func (store *s3Store) Ready(ctx context.Context) error {
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "tcp", store.endpoint.Host)
	if err != nil {
		return err
	}
	if err := connection.Close(); err != nil {
		return err
	}
	_, err = store.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(store.bucket)})
	if err == nil {
		return nil
	}
	_, createErr := store.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(store.bucket)})
	return createErr
}

func (store *s3Store) Put(ctx context.Context, key string, body io.Reader, size int64, mediaType string) error {
	_, err := store.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(store.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(mediaType),
	})
	return err
}

func (store *s3Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	result, err := store.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}
	return result.Body, nil
}

func (store *s3Store) Head(ctx context.Context, key string) (int64, error) {
	result, err := store.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(key)})
	if err != nil {
		return 0, err
	}
	return aws.ToInt64(result.ContentLength), nil
}

func (store *s3Store) PresignGet(ctx context.Context, key string, lifetime time.Duration) (string, time.Time, error) {
	if lifetime <= 0 {
		lifetime = 2 * time.Minute
	}
	expiresAt := time.Now().UTC().Add(lifetime)
	presigned, err := store.presign.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(key)}, func(options *s3.PresignOptions) {
		options.Expires = lifetime
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return presigned.URL, expiresAt, nil
}

func stringsTrimRight(value, cutset string) string {
	for len(value) > 0 && value[len(value)-1] == cutset[0] {
		value = value[:len(value)-1]
	}
	return value
}

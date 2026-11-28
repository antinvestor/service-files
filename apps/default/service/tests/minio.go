package tests

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/pitabwire/util"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"
)

const (
	// MinioImage is the S3-compatible server the storage suites run against.
	// MinIO stopped publishing images to Docker Hub and Quay; Chainguard
	// ships a maintained build whose entrypoint is the minio binary. Its free
	// tier only publishes floating tags, so pin the multi-arch index digest
	// for reproducible runs. This digest is minio RELEASE.2026-11-23T00-10-37Z.
	MinioImage = "cgr.dev/chainguard/minio@sha256:b288eaa709ba2f799a39e9a8ceda71a08fabf90109b12c90badba9afff29485c"
	// MinioRegion is the region the S3 clients sign requests for.
	MinioRegion = "us-east-1"
)

// MinioServer is a running MinIO container plus a raw S3 client that reads
// and writes the store directly, bypassing the provider under test.
type MinioServer struct {
	Container *tcminio.MinioContainer
	Endpoint  string
	Client    *awss3.Client
}

// StartMinio starts MinIO and creates the given buckets.
func StartMinio(ctx context.Context, buckets ...string) (*MinioServer, error) {
	container, err := tcminio.Run(ctx, MinioImage)
	if err != nil {
		return nil, fmt.Errorf("start minio: %w", err)
	}

	conn, err := container.ConnectionString(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("minio connection string: %w", err)
	}

	srv := &MinioServer{Container: container, Endpoint: "http://" + conn}
	srv.Client = awss3.NewFromConfig(aws.Config{
		Credentials:  credentials.NewStaticCredentialsProvider(container.Username, container.Password, ""),
		BaseEndpoint: aws.String(srv.Endpoint),
		Region:       MinioRegion,
	}, func(o *awss3.Options) { o.UsePathStyle = true })

	for _, b := range buckets {
		if _, err = srv.Client.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String(b)}); err != nil {
			_ = container.Terminate(ctx)
			return nil, fmt.Errorf("create bucket %s: %w", b, err)
		}
	}
	return srv, nil
}

// Terminate stops the container.
func (m *MinioServer) Terminate(ctx context.Context) error {
	if m == nil || m.Container == nil {
		return nil
	}
	return m.Container.Terminate(ctx)
}

// PutObject writes body under key, bypassing the provider.
func (m *MinioServer) PutObject(ctx context.Context, bucket, key string, body []byte) error {
	_, err := m.Client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	})
	return err
}

// GetObject reads key straight from the store.
func (m *MinioServer) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	out, err := m.Client.GetObject(ctx, &awss3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}
	defer util.CloseAndLogOnError(ctx, out.Body)
	return io.ReadAll(out.Body)
}

// ObjectExists reports whether key is present in bucket.
func (m *MinioServer) ObjectExists(ctx context.Context, bucket, key string) (bool, error) {
	_, err := m.Client.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err == nil {
		return true, nil
	}
	var notFound *s3types.NotFound
	if errors.As(err, &notFound) {
		return false, nil
	}
	return false, err
}

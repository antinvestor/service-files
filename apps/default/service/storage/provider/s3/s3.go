package s3

import (
	"context"
	"errors"

	"github.com/antinvestor/service-files/apps/default/service/storage/provider/blobprovider"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
)

// ProviderS3 stores objects in an S3-compatible object store (AWS S3, R2, MinIO).
type ProviderS3 struct {
	*blobprovider.Provider

	s3Endpoint    string
	s3AccessKeyID string
	s3Secret      string
	s3Token       string
	s3Region      string
	client        *s3.Client
}

func (provider *ProviderS3) Setup(_ context.Context) error {
	s3Config := aws.Config{
		Credentials:  credentials.NewStaticCredentialsProvider(provider.s3AccessKeyID, provider.s3Secret, provider.s3Token),
		BaseEndpoint: aws.String(provider.s3Endpoint),
		Region:       provider.s3Region,
	}

	provider.client = s3.NewFromConfig(s3Config, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return nil
}

func (provider *ProviderS3) openBucket(ctx context.Context, bucketName string) (*blob.Bucket, error) {
	if provider.client == nil {
		return nil, errors.New("s3 provider used before Setup")
	}
	return s3blob.OpenBucketV2(ctx, provider.client, bucketName, nil)
}

func NewProvider(name, privateBucket, publicBucket, s3Endpoint, s3Region, s3Secret, s3Token, s3AccessKeyID string) *ProviderS3 {
	provider := &ProviderS3{
		s3Endpoint:    s3Endpoint,
		s3Region:      s3Region,
		s3Secret:      s3Secret,
		s3Token:       s3Token,
		s3AccessKeyID: s3AccessKeyID,
	}
	provider.Provider = blobprovider.New(name, privateBucket, publicBucket, provider.openBucket)
	return provider
}

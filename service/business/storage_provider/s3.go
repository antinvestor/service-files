package storage_provider

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
)

type ProviderS3 struct {
	ProviderLocal

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

func (provider *ProviderS3) Init(ctx context.Context, bucketName string) (*blob.Bucket, error) {
	return s3blob.OpenBucketV2(ctx, provider.client, bucketName, nil)
}

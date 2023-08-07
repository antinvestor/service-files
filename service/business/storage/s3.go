package storage

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
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
	s3Session     *session.Session
}

func (provider *ProviderS3) Setup(_ context.Context) error {
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(provider.s3AccessKeyID, provider.s3Secret, provider.s3Token),
		Endpoint:         aws.String(provider.s3Endpoint),
		Region:           aws.String(provider.s3Region),
		S3ForcePathStyle: aws.Bool(true),
	}

	sess, err := session.NewSession(s3Config)
	if err != nil {
		return err
	}

	provider.s3Session = sess
	return nil
}

func (provider *ProviderS3) Init(ctx context.Context, bucketName string) (*blob.Bucket, error) {
	return s3blob.OpenBucket(ctx, provider.s3Session, bucketName, nil)
}

package storage

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
)

type ProviderWasabi struct {
	ProviderLocal

	wasabiEndpoint    string
	wasabiAccessKeyID string
	wasabiSecret      string
	wasabiToken       string
	wasabiRegion      string
	wasabiSession     *session.Session
}

func (provider *ProviderWasabi) Setup(ctx context.Context) error {
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(provider.wasabiAccessKeyID, provider.wasabiSecret, provider.wasabiToken),
		Endpoint:         aws.String(provider.wasabiEndpoint),
		Region:           aws.String(provider.wasabiRegion),
		S3ForcePathStyle: aws.Bool(true),
	}

	sess, err := session.NewSession(s3Config)
	if err != nil {
		return err
	}

	provider.wasabiSession = sess
	return nil
}

func (provider *ProviderWasabi) Init(ctx context.Context, bucketName string) (*blob.Bucket, error) {
	return s3blob.OpenBucket(ctx, provider.wasabiSession, bucketName, nil)
}

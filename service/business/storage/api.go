package storage

import (
	"context"
	"github.com/antinvestor/files/config"
	"github.com/pitabwire/frame"
	"gocloud.dev/blob"
)

type Provider interface {
	Name() string
	PrivateBucket() string
	PublicBucket() string

	Setup(ctx context.Context) error
	Init(ctx context.Context, bucketName string) (*blob.Bucket, error)
	UploadFile(ctx context.Context, bucket string, pathName string, extension string, contents []byte) (string, error)
	DownloadFile(ctx context.Context, bucket string, pathName string, extension string) ([]byte, error)
}

func GetStorageProvider(ctx context.Context, config *config.FilesConfig) (Provider, error) {
	var provider Provider
	switch config.StorageProvider {
	case "GCS":
		provider = &ProviderGCS{
			ProviderLocal: ProviderLocal{
				name:          "GCS",
				privateBucket: config.ProviderGcsPrivateBucket,
				publicBucket:  config.ProviderGcsPublicBucket,
			},
		}

	case "S3":

		provider = &ProviderS3{
			ProviderLocal: ProviderLocal{
				name:          "S3",
				privateBucket: config.ProviderS3PrivateBucket,
				publicBucket:  config.ProviderS3PublicBucket,
			},
			s3Endpoint:    config.ProviderS3Endpoint,
			s3Region:      config.ProviderS3Region,
			s3Secret:      config.ProviderS3Secret,
			s3Token:       config.ProviderS3Token,
			s3AccessKeyID: config.ProviderS3AccessKeyId,
		}
	default:

		provider = &ProviderLocal{
			name:          "LOCAL",
			privateBucket: frame.GetEnv("LOCAL_PRIVATE_DIRECTORY", "/tmp/private"),
			publicBucket:  frame.GetEnv("LOCAL_PUBLIC_DIRECTORY", "/tmp/public"),
		}

	}

	err := provider.Setup(ctx)
	return provider, err

}

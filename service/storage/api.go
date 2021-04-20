package storage

import (
	"context"
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

func GetStorageProvider(ctx context.Context, providerName string) (Provider, error) {

	var provider Provider
	switch providerName {

	case "GCS":
		provider = &ProviderGCS{
			ProviderLocal: ProviderLocal{
				name:          "GCS",
				privateBucket: frame.GetEnv("GCS_PRIVATE_BUCKET", ""),
				publicBucket:  frame.GetEnv("GCS_PUBLIC_BUCKET", ""),
			},
		}

		break

	case "WASABI":

		provider = &ProviderWasabi{
			ProviderLocal: ProviderLocal{
				name:          "WASABI",
				privateBucket: frame.GetEnv("WASABI_PRIVATE_BUCKET", ""),
				publicBucket:  frame.GetEnv("WASABI_PUBLIC_BUCKET", ""),
			},
			wasabiEndpoint:    frame.GetEnv("WASABI_ENDPOINT", ""),
			wasabiRegion:      frame.GetEnv("WASABI_REGION", ""),
			wasabiSecret:      frame.GetEnv("WASABI_SECRET", ""),
			wasabiToken:       frame.GetEnv("WASABI_TOKEN", ""),
			wasabiAccessKeyID: frame.GetEnv("WASABI_ACCESS_KEY_ID", ""),
		}
		break
	default:

		provider = &ProviderLocal{
			name:          "LOCAL",
			privateBucket: frame.GetEnv("LOCAL_PRIVATE_DIRECTORY", "/tmp/private"),
			publicBucket:  frame.GetEnv("LOCAL_PUBLIC_DIRECTORY", "/tmp/public"),
		}
		break

	}

	err := provider.Setup(ctx)
	return provider, err

}

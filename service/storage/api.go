package storage

import (
	"context"
	"github.com/pitabwire/frame"
)

type Provider interface {
	Name() string
	PrivateBucket() string
	PublicBucket() string

	Init(ctx context.Context) (interface{}, error)
	UploadFile(ctx context.Context, bucket string, pathName string, extension string, contents []byte) (string, error)
	DownloadFile(ctx context.Context, bucket string, pathName string, extension string) ([]byte, error)
}

func GetStorageProvider(providerName string) Provider {

	switch providerName {

	case "GCS":
		return &ProviderGCS{
			name:          "GCS",
			projectID:     frame.GetEnv("GCS_PROJECT_ID", ""),
			privateBucket: frame.GetEnv("GCS_PRIVATE_BUCKET", ""),
			publicBucket:  frame.GetEnv("GCS_PUBLIC_BUCKET", ""),
		}

	case "WASABI":

		return &ProviderWasabi{
			name:              "WASABI",
			privateBucket:     frame.GetEnv("WASABI_PRIVATE_BUCKET", ""),
			publicBucket:      frame.GetEnv("WASABI_PUBLIC_BUCKET", ""),
			wasabiEndpoint:    frame.GetEnv("WASABI_ENDPOINT", ""),
			wasabiRegion:      frame.GetEnv("WASABI_REGION", ""),
			wasabiSecret:      frame.GetEnv("WASABI_SECRET", ""),
			wasabiToken:       frame.GetEnv("WASABI_TOKEN", ""),
			wasabiAccessKeyID: frame.GetEnv("WASABI_ACCESS_KEY_ID", ""),
		}

	default:

		return &ProviderLocal{
			name:          "LOCAL",
			privateBucket: frame.GetEnv("LOCAL_PRIVATE_DIRECTORY", "/tmp/private"),
			publicBucket:  frame.GetEnv("LOCAL_PUBLIC_DIRECTORY", "/tmp/public"),
		}

	}

}

package storage

import (
	"context"
	"github.com/antinvestor/files/utils"
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
			projectID:     utils.GetEnv("GCS_PROJECT_ID", ""),
			privateBucket: utils.GetEnv("GCS_PRIVATE_BUCKET", ""),
			publicBucket:  utils.GetEnv("GCS_PUBLIC_BUCKET", ""),
		}

	case "WASABI":

		return &ProviderWasabi{
			name:              "WASABI",
			privateBucket:     utils.GetEnv("WASABI_PRIVATE_BUCKET", ""),
			publicBucket:      utils.GetEnv("WASABI_PUBLIC_BUCKET", ""),
			wasabiEndpoint:    utils.GetEnv("WASABI_ENDPOINT", ""),
			wasabiRegion:      utils.GetEnv("WASABI_REGION", ""),
			wasabiSecret:      utils.GetEnv("WASABI_SECRET", ""),
			wasabiToken:       utils.GetEnv("WASABI_TOKEN", ""),
			wasabiAccessKeyID: utils.GetEnv("WASABI_ACCESS_KEY_ID", ""),
		}

	default:

		return &ProviderLocal{
			name:          "LOCAL",
			privateBucket: utils.GetEnv("LOCAL_PRIVATE_DIRECTORY", "/tmp/private"),
			publicBucket:  utils.GetEnv("LOCAL_PUBLIC_DIRECTORY", "/tmp/public"),
		}

	}

}

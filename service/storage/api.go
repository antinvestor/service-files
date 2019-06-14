package storage

import (
	"bitbucket.org/antinvestor/service-file/utils"
)

type Provider interface {
	Name() string
	PrivateBucket() string
	PublicBucket() string

	Init() error
	UploadFile(bucket string, pathName string, extension string, contents []byte) (string, error)
	DownloadFile(bucket string, pathName string, extension string) ([]byte, error)
}

func GetStorageProvider(providerName string) Provider {

	switch providerName {

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

package storage

import "os"

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
			privateBucket:     os.Getenv("WASABI_PRIVATE_BUCKET"),
			publicBucket:      os.Getenv("WASABI_PUBLIC_BUCKET"),
			wasabiEndpoint:    os.Getenv("WASABI_ENDPOINT"),
			wasabiRegion:      os.Getenv("WASABI_REGION"),
			wasabiSecret:      os.Getenv("WASABI_SECRET"),
			wasabiToken:       os.Getenv("WASABI_TOKEN"),
			wasabiAccessKeyID: os.Getenv("WASABI_ACCESS_KEY_ID"),
		}

	default:

		return &ProviderLocal{
			name:          "LOCAL",
			privateBucket: os.Getenv("LOCAL_PRIVATE_DIRECTORY"),
			publicBucket:  os.Getenv("LOCAL_PUBLIC_DIRECTORY"),
		}

	}

}

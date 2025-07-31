package provider

import (
	"context"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider/gcs"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider/local"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider/s3"
	"github.com/pitabwire/frame"
)

func GetStorageProvider(ctx context.Context, config *config.FilesConfig) (storage.Provider, error) {
	var provider storage.Provider
	switch config.StorageProvider {
	case "GCS":
		provider = gcs.NewProvider("GCS", config.ProviderGcsPrivateBucket, config.ProviderGcsPublicBucket)

	case "S3":

		provider = s3.NewProvider("S3", config.ProviderS3PrivateBucket, config.ProviderS3PublicBucket,
			config.ProviderS3Endpoint, config.ProviderS3Region, config.ProviderS3AccessKeySecret,
			config.ProviderS3SessionToken, config.ProviderS3AccessKeyId)

	default:

		provider = local.NewProvider("LOCAL", frame.GetEnv("LOCAL_PRIVATE_DIRECTORY", "/tmp/private"), frame.GetEnv("LOCAL_PUBLIC_DIRECTORY", "/tmp/public"))

	}

	err := provider.Setup(ctx)
	return provider, err

}

package gcs

import (
	"context"
	"errors"

	"github.com/antinvestor/service-files/apps/default/service/storage/provider/blobprovider"
	"gocloud.dev/blob"
	"gocloud.dev/blob/gcsblob"
	"gocloud.dev/gcp"
)

// ProviderGCS stores objects in Google Cloud Storage.
type ProviderGCS struct {
	*blobprovider.Provider
	client *gcp.HTTPClient
}

func (provider *ProviderGCS) Setup(ctx context.Context) error {
	creds, err := gcp.DefaultCredentials(ctx)
	if err != nil {
		return err
	}

	provider.client, err = gcp.NewHTTPClient(
		gcp.DefaultTransport(),
		gcp.CredentialsTokenSource(creds))

	return err
}

func (provider *ProviderGCS) openBucket(ctx context.Context, bucketName string) (*blob.Bucket, error) {
	if provider.client == nil {
		return nil, errors.New("gcs provider used before Setup")
	}
	return gcsblob.OpenBucket(ctx, provider.client, bucketName, nil)
}

func NewProvider(name, privateBucket, publicBucket string) *ProviderGCS {
	provider := &ProviderGCS{}
	provider.Provider = blobprovider.New(name, privateBucket, publicBucket, provider.openBucket)
	return provider
}

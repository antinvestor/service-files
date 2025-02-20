package storage_provider

import (
	"context"
	"gocloud.dev/blob"
	"gocloud.dev/blob/gcsblob"
	"gocloud.dev/gcp"
)

type ProviderGCS struct {
	ProviderLocal
	client *gcp.HTTPClient
}

func (provider *ProviderGCS) Setup(ctx context.Context) error {

	creds, err := gcp.DefaultCredentials(ctx)
	if err != nil {
		return err
	}

	// Create an HTTP client.
	// This example uses the default HTTP transport and the credentials
	// created above.
	provider.client, err = gcp.NewHTTPClient(
		gcp.DefaultTransport(),
		gcp.CredentialsTokenSource(creds))

	return err
}

func (provider *ProviderGCS) Init(ctx context.Context, bucketName string) (*blob.Bucket, error) {
	return gcsblob.OpenBucket(ctx, provider.client, bucketName, nil)
}

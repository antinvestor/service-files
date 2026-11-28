package local

import (
	"context"
	"fmt"
	"os"

	"github.com/antinvestor/service-files/apps/default/service/storage/provider/blobprovider"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/fileblob"
)

// ProviderLocal stores objects on the local filesystem; bucket names are directories.
type ProviderLocal struct {
	*blobprovider.Provider
}

func (provider *ProviderLocal) Setup(_ context.Context) error {
	for _, dir := range []string{provider.PrivateBucket(), provider.PublicBucket()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func openBucket(ctx context.Context, bucketName string) (*blob.Bucket, error) {
	return blob.OpenBucket(ctx, fmt.Sprintf("file://%s", bucketName))
}

func NewProvider(name, privateBucket, publicBucket string) *ProviderLocal {
	return &ProviderLocal{
		Provider: blobprovider.New(name, privateBucket, publicBucket, openBucket),
	}
}

package local

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/antinvestor/service-files/service/types"
	"github.com/pitabwire/util"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/fileblob"
)

type ProviderLocal struct {
	name          string
	privateBucket string
	publicBucket  string
}

func (provider *ProviderLocal) Name() string {
	return provider.name
}

func (provider *ProviderLocal) PublicBucket() string {
	return provider.publicBucket
}

func (provider *ProviderLocal) PrivateBucket() string {
	return provider.privateBucket
}

func (provider *ProviderLocal) GetBucket(isPublic bool) string {

	if isPublic {
		return provider.PublicBucket()
	}
	return provider.PrivateBucket()
}

func (provider *ProviderLocal) Setup(ctx context.Context) error {

	err := os.MkdirAll(provider.privateBucket, 0755)
	if err != nil {
		return err
	}

	err = os.MkdirAll(provider.publicBucket, 0755)
	if err != nil {
		return err
	}

	return nil
}

func (provider *ProviderLocal) Init(ctx context.Context, bucketName string) (*blob.Bucket, error) {
	return blob.OpenBucket(ctx, fmt.Sprintf("file://%s", bucketName))
}

func (provider *ProviderLocal) UploadFile(ctx context.Context, bucketName string, sourcePath types.Path, inBucketPath types.Path) (bool, error) {

	bucket, err := provider.Init(ctx, bucketName)
	if err != nil {
		return false, err
	}
	defer util.CloseAndLogOnError(ctx, bucket)

	writeCtx, cancelWrite := context.WithCancel(ctx)
	defer cancelWrite()

	exits, err := bucket.Exists(writeCtx, string(inBucketPath))
	if err != nil {
		return false, err
	}

	if exits {
		return true, nil
	}

	// Open the key "foo.txt" for writing with the default options.
	w, err := bucket.NewWriter(writeCtx, string(inBucketPath), nil)
	if err != nil {
		return false, err
	}
	defer util.CloseAndLogOnError(ctx, w)

	tempFile, err := os.Open(string(sourcePath))
	if err != nil {
		return false, err
	}
	defer util.CloseAndLogOnError(ctx, tempFile)

	_, err = w.ReadFrom(tempFile)

	if err != nil {
		return false, err
	}

	return false, nil
}

func (provider *ProviderLocal) DownloadFile(ctx context.Context, bucketName string, inBucketPath types.Path) (io.Reader, func(), error) {

	bucket, err := provider.Init(ctx, bucketName)
	if err != nil {
		return nil, nil, err
	}

	r, err := bucket.NewReader(ctx, string(inBucketPath), nil)
	if err != nil {
		util.CloseAndLogOnError(ctx, bucket)
		return nil, nil, err
	}

	return r, func() {
		util.CloseAndLogOnError(ctx, r) // Ignore errors on cleanup
		util.CloseAndLogOnError(ctx, bucket)
	}, nil
}

func NewProvider(name, provateBucket, publicBucket string) *ProviderLocal {
	return &ProviderLocal{
		name:          name,
		privateBucket: provateBucket,
		publicBucket:  publicBucket,
	}
}

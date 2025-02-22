package local

import (
	"context"
	"fmt"
	"github.com/antinvestor/service-files/service/types"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/fileblob"
	"io"
	"os"
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
	defer bucket.Close()

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
	defer w.Close()

	tempFile, err := os.Open(string(sourcePath))
	if err != nil {
		return false, err
	}
	defer tempFile.Close()

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
		_ = bucket.Close()
		return nil, nil, err
	}

	return r, func() {
		_ = r.Close() // Ignore errors on cleanup
		_ = bucket.Close()
	}, nil
}

func NewProvider(name, provateBucket, publicBucket string) *ProviderLocal {
	return &ProviderLocal{
		name:          name,
		privateBucket: provateBucket,
		publicBucket:  publicBucket,
	}
}

package storage

import (
	"context"
	"fmt"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/fileblob"
	"io"
	"os"
	"path/filepath"
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

func (provider *ProviderLocal) UploadFile(ctx context.Context, bucketName string, pathName string, extension string, contents []byte) (string, error) {

	bucket, err := provider.Init(ctx, bucketName)
	if err != nil {
		return "", err
	}
	defer bucket.Close()

	fullPathName := filepath.Join(pathName, extension)

	writeCtx, cancelWrite := context.WithCancel(ctx)
	defer cancelWrite()

	// Open the key "foo.txt" for writing with the default options.
	w, err := bucket.NewWriter(writeCtx, fullPathName, nil)
	if err != nil {
		return "", err
	}
	defer w.Close()

	_, err = w.Write(contents)

	if err != nil {
		// First cancel the context.
		cancelWrite()
		return "", err
	}

	return fullPathName, nil
}

func (provider *ProviderLocal) DownloadFile(ctx context.Context, bucketName string, pathName string, extension string) ([]byte, error) {

	bucket, err := provider.Init(ctx, bucketName)
	if err != nil {
		return nil, err
	}
	defer bucket.Close()

	fullPathName := filepath.Join(pathName, extension)

	r, err := bucket.NewReader(ctx, fullPathName, nil)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return io.ReadAll(r)
}

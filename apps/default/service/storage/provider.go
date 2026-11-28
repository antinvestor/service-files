package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/antinvestor/service-files/apps/default/service/types"
	"gocloud.dev/blob"
)

type Provider interface {
	Name() string
	PrivateBucket() string
	PublicBucket() string
	GetBucket(isPublic bool) string
	Setup(ctx context.Context) error
	Init(ctx context.Context, bucketName string) (*blob.Bucket, error)
	UploadFile(ctx context.Context, bucket string, sourcePath types.Path, destinationPath types.Path) (bool, error)
	DownloadFile(ctx context.Context, bucket string, sourcePath types.Path) (io.Reader, func(), error)
	// DeleteFile removes the object; a missing object is not an error.
	DeleteFile(ctx context.Context, bucket string, path types.Path) error
}

// UploadFileWithHashCheck uploads sourcePath under the content's stable object
// key (see ContentObjectKey) in the bucket for its visibility, and records
// that location (bucket, key, provider) on mediaMetadata so it is persisted
// with the row and every later read and delete targets the same object.
// It reports whether the object already existed (a content-addressed duplicate).
func UploadFileWithHashCheck(ctx context.Context, provider Provider, sourcePath types.Path, mediaMetadata *types.MediaMetadata) (types.Path, bool, error) {
	key, err := ContentObjectKey(mediaMetadata)
	if err != nil {
		return "", false, err
	}

	bucket := provider.GetBucket(mediaMetadata.IsPublic)
	duplicate, err := provider.UploadFile(ctx, bucket, sourcePath, types.Path(key))
	if err != nil {
		return "", duplicate, fmt.Errorf("upload object %q to bucket %q: %w", key, bucket, err)
	}

	mediaMetadata.BucketName = bucket
	mediaMetadata.ObjectKey = key
	mediaMetadata.StorageProvider = provider.Name()
	return types.Path(key), duplicate, nil
}
